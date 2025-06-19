package input

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/protocol"
	evdev "github.com/gvalkov/golang-evdev"
)

// Modifier bit constants for keyboard events
const (
	ModifierShift = 1 << 0 // Shift modifier bit
	ModifierCaps  = 1 << 1 // Caps lock modifier bit
	ModifierCtrl  = 1 << 2 // Control modifier bit
	ModifierAlt   = 1 << 3 // Alt modifier bit
	ModifierMeta  = 1 << 6 // Meta/Super modifier bit
)

// AllDevicesCapture captures input events from all available input devices
type AllDevicesCapture struct {
	mu             sync.RWMutex
	devices        map[string]*deviceHandler
	ignoredDevices map[string]bool // devices that are not suitable for capture
	eventChan      chan *protocol.InputEvent
	onInputEvent   func(*protocol.InputEvent)
	currentTarget  string
	capturing      bool
	ctx            context.Context
	cancel         context.CancelFunc
	deviceMonitor  *DeviceMonitor

	// Safety mechanisms
	grabTimeout      time.Duration // Auto-release timeout
	grabTimer        *time.Timer   // Timer for auto-release
	emergencyKey     uint16        // Key code for emergency release (e.g., ESC)
	lastActivity     time.Time     // Last input activity time
	noGrab           bool          // Disable exclusive grab (for safer testing)
	ctrlPressed      bool          // Track if Ctrl key is pressed
	emergencyHandler func()        // Optional callback for emergency release

	// Key state tracking for proper release
	pressedKeys map[uint16]bool // Track currently pressed keys
	keyStateMu  sync.Mutex      // Mutex for key state access

	// Modifier state tracking
	modifierState uint32 // Current modifier bitmask
	shiftPressed  bool   // Track Shift key state
	altPressed    bool   // Track Alt key state
	metaPressed   bool   // Track Meta/Super key state
}

// deviceHandler manages a single input device
type deviceHandler struct {
	path    string
	device  *evdev.InputDevice
	cancel  context.CancelFunc
	name    string
	grabbed bool // Track if device is currently grabbed
}

// NewAllDevicesCapture creates a new all-devices input capture
func NewAllDevicesCapture() *AllDevicesCapture {
	return &AllDevicesCapture{
		devices:        make(map[string]*deviceHandler),
		ignoredDevices: make(map[string]bool),
		eventChan:      make(chan *protocol.InputEvent, 4096), // Large buffer for low latency
		capturing:      false,
		grabTimeout:    10 * time.Second,      // Reduced to 10 second safety timeout for better UX
		emergencyKey:   evdev.KEY_ESC,         // ESC key for emergency release (requires Ctrl)
		pressedKeys:    make(map[uint16]bool), // Initialize pressed keys tracking
	}
}

// Start starts capturing input events from all devices
func (a *AllDevicesCapture) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.capturing {
		return fmt.Errorf("already capturing")
	}

	// Create cancellable context
	a.ctx, a.cancel = context.WithCancel(ctx)

	// Initialize device monitor
	a.deviceMonitor = NewDeviceMonitor()

	// Start monitoring for device changes
	go a.monitorDeviceChanges()

	// Start event processing goroutine
	go a.processEvents()

	// Discover and start capturing from existing devices
	if err := a.discoverAndStartDevices(); err != nil {
		a.cancel()
		return fmt.Errorf("failed to discover devices: %w", err)
	}

	a.capturing = true
	logger.Info("All-devices input capture started")
	return nil
}

// Stop stops capturing input events
func (a *AllDevicesCapture) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.capturing {
		return nil
	}

	// Cancel any existing grab timer first
	if a.grabTimer != nil {
		a.grabTimer.Stop()
		a.grabTimer = nil
	}

	// Release all pressed keys before stopping
	a.releaseAllPressedKeys()

	// Release all grabbed devices
	for _, handler := range a.devices {
		if handler.device != nil && handler.grabbed {
			if err := handler.device.Release(); err != nil {
				logger.Errorf("Failed to release device %s on stop: %v", handler.path, err)
			}
			handler.grabbed = false
		}
	}

	// Clear the current target
	a.currentTarget = ""

	// Cancel context to stop all goroutines
	if a.cancel != nil {
		a.cancel()
	}

	// Stop all device handlers
	for _, handler := range a.devices {
		a.stopDeviceHandler(handler)
	}

	// Clear devices map
	a.devices = make(map[string]*deviceHandler)

	// Clear ignored devices (fresh start)
	a.ignoredDevices = make(map[string]bool)

	// Close event channel
	close(a.eventChan)
	a.eventChan = make(chan *protocol.InputEvent, 4096) // Match the initial buffer size

	a.capturing = false
	logger.Info("All-devices input capture stopped and all devices released")
	return nil
}

// SetTarget sets the target client ID for forwarding events
func (a *AllDevicesCapture) SetTarget(clientID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	oldTarget := a.currentTarget
	a.currentTarget = clientID

	// Handle device grabbing based on target
	if clientID == "" {
		// Cancel any existing grab timer
		if a.grabTimer != nil {
			a.grabTimer.Stop()
			a.grabTimer = nil
		}

		// Send key release events for all currently pressed keys
		a.releaseAllPressedKeys()

		// Release all devices when controlling local system
		var releaseCount int
		for _, handler := range a.devices {
			if handler.device != nil && handler.grabbed {
				if err := handler.device.Release(); err != nil {
					logger.Errorf("Failed to release device %s: %v", handler.path, err)
				} else {
					handler.grabbed = false
					releaseCount++
				}
			}
		}
		if releaseCount > 0 {
			logger.Infof("Released %d devices", releaseCount)
		}
		logger.Info("Input capture target cleared - controlling local system")
	} else {
		// Only grab devices if not in no-grab mode
		if !a.noGrab {
			// Grab all devices when controlling a client
			var grabErrors []string
			var successCount int
			for _, handler := range a.devices {
				if handler.device != nil && !handler.grabbed {
					if err := handler.device.Grab(); err != nil {
						grabErrors = append(grabErrors, fmt.Sprintf("%s: %v", handler.path, err))
						logger.Warnf("Failed to grab device %s (%s): %v", handler.name, handler.path, err)
					} else {
						handler.grabbed = true
						successCount++
						logger.Debugf("Successfully grabbed device %s (%s)", handler.name, handler.path)
					}
				}
			}
			logger.Infof("Grabbed %d/%d devices", successCount, len(a.devices))

			if len(grabErrors) > 0 {
				// Revert target on grab failure
				a.currentTarget = oldTarget
				return fmt.Errorf("failed to grab input devices: %s", strings.Join(grabErrors, ", "))
			}

			// Set up safety timeout
			a.lastActivity = time.Now()
			a.grabTimer = time.AfterFunc(a.grabTimeout, func() {
				logger.Warnf("Safety timeout reached - auto-releasing devices")
				a.mu.Lock()
				if a.currentTarget != "" {
					target := a.currentTarget
					a.currentTarget = ""
					// Release all pressed keys first to prevent stuck modifiers
					a.releaseAllPressedKeys()
					for _, handler := range a.devices {
						if handler.device != nil && handler.grabbed {
							if err := handler.device.Release(); err != nil {
								logger.Errorf("Failed to release device in safety timeout: %v", err)
							}
							handler.grabbed = false
						}
					}
					a.mu.Unlock()
					// Notify emergency handler if set (outside of lock)
					if a.emergencyHandler != nil {
						logger.Debugf("Notifying emergency handler from safety timeout (was controlling: %s)", target)
						go a.emergencyHandler()
					}
				} else {
					a.mu.Unlock()
				}
			})

			logger.Infof("Set input capture target to client: %s (timeout: %v)", clientID, a.grabTimeout)
		} else {
			logger.Infof("Set input capture target to client: %s (no-grab mode enabled)", clientID)
		}
	}
	return nil
}

// OnInputEvent sets the callback for input events
func (a *AllDevicesCapture) OnInputEvent(callback func(*protocol.InputEvent)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onInputEvent = callback
	logger.Info("All-devices input event callback set")
}

// discoverAndStartDevices finds all input devices and starts capturing from them
func (a *AllDevicesCapture) discoverAndStartDevices() error {
	eventDir := "/dev/input"
	entries, err := os.ReadDir(eventDir)
	if err != nil {
		return fmt.Errorf("failed to read input directory: %w", err)
	}

	logger.Infof("Discovering input devices in %s", eventDir)
	deviceCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "event") {
			path := filepath.Join(eventDir, entry.Name())

			// Skip devices we've already determined are not suitable
			if a.ignoredDevices[path] {
				continue
			}

			if err := a.addDevice(path); err != nil {
				// Add to ignored devices list so we don't try again
				a.ignoredDevices[path] = true
				if strings.Contains(err.Error(), "no relevant input capabilities") {
					// This is expected for many devices, use trace level
					logger.Debugf("Device %s not suitable for capture: %v", path, err)
				} else {
					// This might be a real error, log it
					logger.Warnf("Failed to add device %s: %v", path, err)
				}
			} else {
				deviceCount++
			}
		}
	}

	ignoredCount := len(a.ignoredDevices)
	if ignoredCount > 0 {
		logger.Infof("Started capturing from %d input devices (%d devices ignored)", deviceCount, ignoredCount)
	} else {
		logger.Infof("Started capturing from %d input devices", deviceCount)
	}
	return nil
}

// addDevice adds a new input device for monitoring
func (a *AllDevicesCapture) addDevice(path string) error {
	// Check if device is already being monitored
	if _, exists := a.devices[path]; exists {
		return nil
	}

	// Try to open the device
	device, err := evdev.Open(path)
	if err != nil {
		logger.Debugf("Cannot open device %s: %v", path, err)
		return fmt.Errorf("failed to open device %s: %w", path, err)
	}

	// Check if device has input capabilities we care about
	if !a.isValidInputDevice(device) {
		if err := device.File.Close(); err != nil {
			logger.Warnf("Failed to close device file %s: %v", path, err)
		}
		return fmt.Errorf("device %s has no relevant input capabilities", path)
	}

	// Create device handler
	handler := &deviceHandler{
		path:   path,
		device: device,
		name:   device.Name,
	}

	// Start capturing from this device
	handlerCtx, handlerCancel := context.WithCancel(a.ctx)
	handler.cancel = handlerCancel

	// Add to devices map
	a.devices[path] = handler

	// Start capture goroutine for this device
	go a.captureFromDevice(handlerCtx, handler)

	logger.Infof("Added input device: %s (%s)", handler.name, path)
	return nil
}

// removeDevice removes a device from monitoring
func (a *AllDevicesCapture) removeDevice(path string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	handler, exists := a.devices[path]
	if !exists {
		return
	}

	a.stopDeviceHandler(handler)
	delete(a.devices, path)
	logger.Infof("Removed input device: %s", path)
}

// stopDeviceHandler stops a device handler
func (a *AllDevicesCapture) stopDeviceHandler(handler *deviceHandler) {
	if handler.cancel != nil {
		handler.cancel()
	}
	// evdev devices are automatically closed when the process exits
	// No explicit Close() method needed
}

// isValidInputDevice checks if a device has input capabilities we care about
func (a *AllDevicesCapture) isValidInputDevice(device *evdev.InputDevice) bool {
	// Filter out virtual terminals, console devices, and other system devices
	deviceName := strings.ToLower(device.Name)

	// Exclude devices that are likely to cause issues
	excludePatterns := []string{
		"virtual console",
		"system console",
		"tty",
		"vt",
		"console mouse",
		"speakup",
		"pc speaker",
		"hdmi",
		"video bus",
		"power button",
		"sleep button",
		"lid switch",
	}

	for _, pattern := range excludePatterns {
		if strings.Contains(deviceName, pattern) {
			logger.Debugf("Excluding device %s (matches pattern: %s)", device.Name, pattern)
			return false
		}
	}

	capabilities := device.Capabilities

	// Look for capability types we care about
	for capType, caps := range capabilities {
		switch capType.Type {
		case 1: // EV_KEY
			for _, cap := range caps {
				// Mouse buttons
				if cap.Code >= evdev.BTN_LEFT && cap.Code <= evdev.BTN_TASK {
					return true
				}
				// Keyboard keys (any key from A-Z range or common keys)
				if cap.Code >= evdev.KEY_A && cap.Code <= evdev.KEY_Z {
					return true
				}
				if cap.Code == evdev.KEY_SPACE || cap.Code == evdev.KEY_ENTER ||
					cap.Code == evdev.KEY_ESC || cap.Code == evdev.KEY_TAB {
					return true
				}
			}
		case 2: // EV_REL
			for _, cap := range caps {
				if cap.Code == evdev.REL_X || cap.Code == evdev.REL_Y ||
					cap.Code == evdev.REL_WHEEL || cap.Code == evdev.REL_HWHEEL {
					return true
				}
			}
		}
	}

	return false
}

// monitorDeviceChanges monitors for device addition/removal
func (a *AllDevicesCapture) monitorDeviceChanges() {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Device monitor panic: %v", r)
		}
	}()

	// Use simple polling for now - can be improved with inotify later
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.checkForDeviceChanges()
		}
	}
}

// checkForDeviceChanges checks for new or removed devices
func (a *AllDevicesCapture) checkForDeviceChanges() {
	eventDir := "/dev/input"
	entries, err := os.ReadDir(eventDir)
	if err != nil {
		logger.Warnf("Failed to read input directory during device monitoring: %v", err)
		return
	}

	// Track current devices
	currentDevices := make(map[string]bool)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "event") {
			path := filepath.Join(eventDir, entry.Name())
			currentDevices[path] = true

			// Skip devices we know are not suitable
			a.mu.RLock()
			_, exists := a.devices[path]
			ignored := a.ignoredDevices[path]
			a.mu.RUnlock()

			if !exists && !ignored {
				if err := a.addDevice(path); err != nil {
					// Add to ignored devices list
					a.mu.Lock()
					a.ignoredDevices[path] = true
					a.mu.Unlock()
					logger.Debugf("Device %s not suitable for capture, adding to ignore list: %v", path, err)
				}
			}
		}
	}

	// Remove devices that no longer exist (from both active and ignored lists)
	a.mu.RLock()
	devicePaths := make([]string, 0, len(a.devices))
	for path := range a.devices {
		devicePaths = append(devicePaths, path)
	}
	ignoredPaths := make([]string, 0, len(a.ignoredDevices))
	for path := range a.ignoredDevices {
		ignoredPaths = append(ignoredPaths, path)
	}
	a.mu.RUnlock()

	// Remove active devices that no longer exist
	for _, path := range devicePaths {
		if !currentDevices[path] {
			a.removeDevice(path)
		}
	}

	// Remove ignored devices that no longer exist (so they can be retested if reconnected)
	for _, path := range ignoredPaths {
		if !currentDevices[path] {
			a.mu.Lock()
			delete(a.ignoredDevices, path)
			a.mu.Unlock()
			logger.Debugf("Removed %s from ignore list (device no longer exists)", path)
		}
	}
}

// captureFromDevice captures events from a specific device
func (a *AllDevicesCapture) captureFromDevice(ctx context.Context, handler *deviceHandler) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Device capture panic for %s: %v", handler.path, r)
		}
	}()

	logger.Debugf("Starting capture from device: %s (%s)", handler.name, handler.path)

	// Variables to accumulate relative movements
	var accX, accY int32
	var lastFlush time.Time
	flushInterval := 500 * time.Microsecond // 0.5ms for lower latency
	var mu sync.Mutex

	// Start a goroutine to periodically flush accumulated movements
	flushTicker := time.NewTicker(flushInterval)
	defer flushTicker.Stop()
	
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-flushTicker.C:
				mu.Lock()
				if (accX != 0 || accY != 0) && time.Since(lastFlush) >= flushInterval {
					a.sendEvent(&protocol.InputEvent{
						Event: &protocol.InputEvent_MouseMove{
							MouseMove: &protocol.MouseMoveEvent{
								Dx: float64(accX),
								Dy: float64(accY),
							},
						},
						Timestamp: time.Now().UnixNano(),
						SourceId:  fmt.Sprintf("all-devices-%s", filepath.Base(handler.path)),
					})
					accX, accY = 0, 0
					lastFlush = time.Now()
				}
				mu.Unlock()
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			logger.Debugf("Device capture context cancelled for %s", handler.path)
			return
		default:
			// Read events with a small timeout
			events, err := handler.device.Read()
			if err != nil {
				if !strings.Contains(err.Error(), "resource temporarily unavailable") {
					logger.Errorf("Error reading events from %s: %v", handler.path, err)
				}
				time.Sleep(5 * time.Millisecond)
				continue
			}

			for _, event := range events {
				switch event.Type {
				case evdev.EV_REL:
					switch event.Code {
					case evdev.REL_X:
						mu.Lock()
						accX += event.Value
						mu.Unlock()
					case evdev.REL_Y:
						mu.Lock()
						accY += event.Value
						mu.Unlock()
					case evdev.REL_WHEEL:
						a.sendScrollEvent(0, float64(event.Value))
					case evdev.REL_HWHEEL:
						a.sendScrollEvent(float64(event.Value), 0)
					}
				case evdev.EV_KEY:
					// Modifier state will be handled in sendKeyboardEvent
					// No need to duplicate tracking here

					// First update the key state for this event
					if event.Value == 1 || event.Value == 0 { // Only for press/release, not autorepeat
						a.keyStateMu.Lock()
						a.updateModifierState(event.Code, event.Value == 1)
						ctrlPressed := a.ctrlPressed
						a.keyStateMu.Unlock()

						// Check for emergency release key combination (Ctrl+ESC)
						a.mu.RLock()
						emergencyKey := a.emergencyKey
						currentTarget := a.currentTarget
						noGrab := a.noGrab
						a.mu.RUnlock()

						// Debug emergency key detection
						if event.Code == emergencyKey {
							logger.Debugf("ESC key detected: value=%d, ctrlPressed=%v, currentTarget=%s, noGrab=%v",
								event.Value, ctrlPressed, currentTarget, noGrab)
						}

						// Only check emergency key if we're grabbing devices and Ctrl is pressed
						if !noGrab && event.Code == emergencyKey && event.Value == 1 && currentTarget != "" && ctrlPressed {
							logger.Warnf("Emergency release triggered - Ctrl+ESC pressed")
							// Use goroutine to avoid deadlock
							go func() {
								// First try to release at our level
								if err := a.SetTarget(""); err != nil {
									logger.Errorf("Failed to release on emergency: %v", err)
								}

								// Also notify the handler if set (ClientManager)
								if a.emergencyHandler != nil {
									logger.Debug("Notifying emergency handler")
									a.emergencyHandler()
								}
							}()
							continue
						}
					}

					if event.Code >= evdev.BTN_LEFT && event.Code <= evdev.BTN_TASK {
						a.sendMouseButtonEvent(event.Code, event.Value)
					} else {
						a.sendKeyboardEvent(event.Code, event.Value)
					}
				case evdev.EV_SYN:
					// Synchronization event - ignore
				case evdev.EV_MSC:
					// Miscellaneous events - ignore
				}
			}
		}
	}
}

// sendEvent sends an event to the event channel
func (a *AllDevicesCapture) sendEvent(event *protocol.InputEvent) {
	// Update activity timestamp and reset timer if we have an active grab
	a.mu.Lock()
	if a.currentTarget != "" {
		a.lastActivity = time.Now()
		if a.grabTimer != nil {
			a.grabTimer.Reset(a.grabTimeout)
		}
	}
	a.mu.Unlock()

	select {
	case a.eventChan <- event:
	default:
		// Channel full, drop event
		logger.Warnf("Event channel full, dropping event")
	}
}

// sendMouseButtonEvent sends a mouse button event
func (a *AllDevicesCapture) sendMouseButtonEvent(code uint16, value int32) {
	// Convert evdev button codes to protocol button numbers
	var button uint32
	switch code {
	case evdev.BTN_LEFT:
		button = 1
	case evdev.BTN_RIGHT:
		button = 2
	case evdev.BTN_MIDDLE:
		button = 3
	case evdev.BTN_SIDE:
		button = 4
	case evdev.BTN_EXTRA:
		button = 5
	default:
		// For other buttons, try to normalize by subtracting BTN_LEFT base
		if code >= evdev.BTN_LEFT && code <= evdev.BTN_TASK {
			button = uint32(code - evdev.BTN_LEFT + 1)
		} else {
			logger.Warnf("Unknown button code: %d", code)
			return
		}
	}

	a.sendEvent(&protocol.InputEvent{
		Event: &protocol.InputEvent_MouseButton{
			MouseButton: &protocol.MouseButtonEvent{
				Button:  button,
				Pressed: value == 1,
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "all-devices-capture",
	})
}

// sendScrollEvent sends a mouse scroll event
func (a *AllDevicesCapture) sendScrollEvent(dx, dy float64) {
	a.sendEvent(&protocol.InputEvent{
		Event: &protocol.InputEvent_MouseScroll{
			MouseScroll: &protocol.MouseScrollEvent{
				Dx: dx,
				Dy: dy,
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "all-devices-capture",
	})
}

// sendKeyboardEvent sends a keyboard event
func (a *AllDevicesCapture) sendKeyboardEvent(code uint16, value int32) {
	// value can be 0 (release), 1 (press), 2 (autorepeat)
	// We treat autorepeat as a press
	pressed := value > 0

	// Track key state and modifier state
	a.keyStateMu.Lock()
	if pressed {
		a.pressedKeys[code] = true
	} else {
		delete(a.pressedKeys, code)
	}

	// Update modifier state based on modifier key events
	a.updateModifierState(code, pressed)

	// Get current modifier state for this event
	currentModifierState := a.modifierState
	a.keyStateMu.Unlock()

	logger.Debugf("[ALL-DEVICES] Sending keyboard event: key=%d, pressed=%v, modifiers=%032b", code, pressed, currentModifierState)

	a.sendEvent(&protocol.InputEvent{
		Event: &protocol.InputEvent_Keyboard{
			Keyboard: &protocol.KeyboardEvent{
				Key:       uint32(code),
				Pressed:   pressed,
				Modifiers: currentModifierState,
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "all-devices-capture",
	})
}

// updateModifierState updates the modifier state tracking
func (a *AllDevicesCapture) updateModifierState(code uint16, pressed bool) {
	// Check for modifier keys and update state accordingly
	switch code {
	case evdev.KEY_LEFTSHIFT, evdev.KEY_RIGHTSHIFT:
		a.shiftPressed = pressed
		if pressed {
			a.modifierState |= ModifierShift
		} else {
			a.modifierState &^= ModifierShift
		}
	case evdev.KEY_LEFTCTRL, evdev.KEY_RIGHTCTRL:
		a.ctrlPressed = pressed
		if pressed {
			a.modifierState |= ModifierCtrl
		} else {
			a.modifierState &^= ModifierCtrl
		}
	case evdev.KEY_LEFTALT, evdev.KEY_RIGHTALT:
		a.altPressed = pressed
		if pressed {
			a.modifierState |= ModifierAlt
		} else {
			a.modifierState &^= ModifierAlt
		}
	case evdev.KEY_LEFTMETA, evdev.KEY_RIGHTMETA:
		a.metaPressed = pressed
		if pressed {
			a.modifierState |= ModifierMeta
		} else {
			a.modifierState &^= ModifierMeta
		}
	case evdev.KEY_CAPSLOCK:
		// Handle caps lock as a toggle modifier
		if pressed {
			a.modifierState ^= ModifierCaps
		}
	}
}

// releaseAllPressedKeys sends release events for all currently pressed keys
func (a *AllDevicesCapture) releaseAllPressedKeys() {
	a.keyStateMu.Lock()
	pressedKeyCodes := make([]uint16, 0, len(a.pressedKeys))
	for keyCode := range a.pressedKeys {
		pressedKeyCodes = append(pressedKeyCodes, keyCode)
	}
	a.keyStateMu.Unlock()

	// Send release events for all pressed keys
	for _, keyCode := range pressedKeyCodes {
		logger.Debugf("Releasing stuck key: %d", keyCode)
		a.sendEvent(&protocol.InputEvent{
			Event: &protocol.InputEvent_Keyboard{
				Keyboard: &protocol.KeyboardEvent{
					Key:     uint32(keyCode),
					Pressed: false,
				},
			},
			Timestamp: time.Now().UnixNano(),
			SourceId:  "all-devices-capture-release",
		})
	}

	// Clear the pressed keys map and reset modifier state
	a.keyStateMu.Lock()
	a.pressedKeys = make(map[uint16]bool)
	a.modifierState = 0
	a.ctrlPressed = false
	a.shiftPressed = false
	a.altPressed = false
	a.metaPressed = false
	a.keyStateMu.Unlock()

	if len(pressedKeyCodes) > 0 {
		logger.Infof("Released %d stuck keys during control handoff", len(pressedKeyCodes))
	}
}

// SetGrabTimeout sets the safety timeout for device grabbing
func (a *AllDevicesCapture) SetGrabTimeout(timeout time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.grabTimeout = timeout
	logger.Infof("Set device grab timeout to %v", timeout)
}

// SetEmergencyKey sets the key code for emergency release
func (a *AllDevicesCapture) SetEmergencyKey(keyCode uint16) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.emergencyKey = keyCode
	logger.Infof("Set emergency release key to code %d", keyCode)
}

// SetNoGrab enables or disables no-grab mode (non-exclusive capture)
func (a *AllDevicesCapture) SetNoGrab(noGrab bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.noGrab = noGrab
	if noGrab {
		logger.Info("No-grab mode enabled - devices will not be grabbed exclusively")
	} else {
		logger.Info("No-grab mode disabled - devices will be grabbed exclusively when controlling clients")
	}
}

// SetEmergencyHandler sets a callback for emergency release events
func (a *AllDevicesCapture) SetEmergencyHandler(handler func()) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.emergencyHandler = handler
}

// processEvents processes events from the event channel
func (a *AllDevicesCapture) processEvents() {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Event processor panic: %v", r)
		}
	}()

	for {
		select {
		case <-a.ctx.Done():
			return
		case event, ok := <-a.eventChan:
			if !ok {
				return
			}

			a.mu.RLock()
			target := a.currentTarget
			callback := a.onInputEvent
			a.mu.RUnlock()

			// Only forward events if we have a target and callback
			if target != "" && callback != nil {
				logger.Debugf("Forwarding %T event to callback (target: %s)", event.Event, target)
				callback(event)
			} else if callback == nil && target != "" {
				logger.Warnf("No callback set for input events!")
			}
			// Note: if target == "", we're controlling local system and don't forward
		}
	}
}
