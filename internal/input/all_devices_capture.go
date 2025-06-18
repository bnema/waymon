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
		eventChan:      make(chan *protocol.InputEvent, 1000), // Increased buffer to handle bursts
		capturing:      false,
		grabTimeout:    30 * time.Second, // Default 30 second safety timeout
		emergencyKey:   evdev.KEY_ESC,    // ESC key for emergency release (requires Ctrl)
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
	a.eventChan = make(chan *protocol.InputEvent, 1000) // Match the initial buffer size

	a.capturing = false
	logger.Info("All-devices input capture stopped")
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
					a.currentTarget = ""
					for _, handler := range a.devices {
						if handler.device != nil && handler.grabbed {
							handler.device.Release()
							handler.grabbed = false
						}
					}
				}
				a.mu.Unlock()
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
		device.File.Close()
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
		if capType.Type == 1 { // EV_KEY
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
		} else if capType.Type == 2 { // EV_REL
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
	ticker := time.NewTicker(16 * time.Millisecond) // ~60 FPS
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Debugf("Device capture context cancelled for %s", handler.path)
			return
		case <-ticker.C:
			// Send accumulated movement if any
			if accX != 0 || accY != 0 {
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
			}
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
						accX += event.Value
					case evdev.REL_Y:
						accY += event.Value
					case evdev.REL_WHEEL:
						a.sendScrollEvent(0, float64(event.Value))
					case evdev.REL_HWHEEL:
						a.sendScrollEvent(float64(event.Value), 0)
					}
				case evdev.EV_KEY:
					// Track Ctrl key state
					if event.Code == evdev.KEY_LEFTCTRL || event.Code == evdev.KEY_RIGHTCTRL {
						a.mu.Lock()
						a.ctrlPressed = (event.Value == 1)
						a.mu.Unlock()
						logger.Debugf("Ctrl key state changed: pressed=%v", event.Value == 1)
					}

					// Check for emergency release key combination (Ctrl+ESC)
					a.mu.RLock()
					emergencyKey := a.emergencyKey
					currentTarget := a.currentTarget
					noGrab := a.noGrab
					ctrlPressed := a.ctrlPressed
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
	a.sendEvent(&protocol.InputEvent{
		Event: &protocol.InputEvent_Keyboard{
			Keyboard: &protocol.KeyboardEvent{
				Key:     uint32(code),
				Pressed: value > 0,
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "all-devices-capture",
	})
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
			} else if target == "" {
				// Controlling local system, don't forward
			} else if callback == nil {
				logger.Warnf("No callback set for input events!")
			}
		}
	}
}
