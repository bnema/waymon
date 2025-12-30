// Package evdev provides input capture using Linux evdev devices.
package evdev

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/domain"
	evdev "github.com/gvalkov/golang-evdev"
	"github.com/rs/zerolog"
)

// Compile-time interface check
var _ interface {
	Start(ctx context.Context) error
	Stop() error
	SetTarget(clientID string) error
	SetEventCallback(callback func(*domain.InputEvent))
} = (*Capture)(nil)

// Capture implements InputCapturePort using Linux evdev devices.
// It captures mouse and keyboard events from all suitable input devices.
type Capture struct {
	mu             sync.RWMutex
	devices        map[string]*deviceHandler
	ignoredDevices map[string]bool
	eventChan      chan *domain.InputEvent
	onInputEvent   func(*domain.InputEvent)
	currentTarget  string
	capturing      bool
	ctx            context.Context
	cancel         context.CancelFunc

	// Safety mechanisms
	grabTimeout      time.Duration
	grabTimer        *time.Timer
	emergencyKey     uint16
	lastActivity     time.Time
	noGrab           bool
	ctrlPressed      bool
	emergencyHandler func()
	watchdogStop     chan struct{}
}

// deviceHandler manages a single input device.
type deviceHandler struct {
	path    string
	device  *evdev.InputDevice
	cancel  context.CancelFunc
	name    string
	grabbed bool
}

// New creates a new evdev Capture instance.
func New() *Capture {
	return &Capture{
		devices:        make(map[string]*deviceHandler),
		ignoredDevices: make(map[string]bool),
		eventChan:      make(chan *domain.InputEvent, 5000),
		capturing:      false,
		grabTimeout:    30 * time.Second,
		emergencyKey:   evdev.KEY_ESC,
	}
}

// Start begins capturing input events from all devices.
func (c *Capture) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	log := zerolog.Ctx(ctx)

	if c.capturing {
		return fmt.Errorf("already capturing")
	}

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Start event processing goroutine
	go c.processEvents()

	// Start watchdog
	c.watchdogStop = make(chan struct{})
	go c.watchdog()

	// Start device monitoring
	go c.monitorDeviceChanges()

	// Discover and start capturing from existing devices
	if err := c.discoverAndStartDevices(); err != nil {
		c.cancel()
		return fmt.Errorf("failed to discover devices: %w", err)
	}

	c.capturing = true
	log.Info().Msg("evdev input capture started")
	return nil
}

// Stop stops capturing input events.
func (c *Capture) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	log := zerolog.Ctx(c.ctx)

	if !c.capturing {
		return nil
	}

	if c.cancel != nil {
		c.cancel()
	}

	// Stop all device handlers
	for _, handler := range c.devices {
		c.stopDeviceHandler(handler)
	}

	c.devices = make(map[string]*deviceHandler)
	c.ignoredDevices = make(map[string]bool)

	if c.watchdogStop != nil {
		close(c.watchdogStop)
	}

	close(c.eventChan)
	c.eventChan = make(chan *domain.InputEvent, 5000)

	c.capturing = false
	log.Info().Msg("evdev input capture stopped")
	return nil
}

// SetTarget sets the target client ID for forwarding events.
// Empty string means control local system (release devices).
func (c *Capture) SetTarget(clientID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	log := zerolog.Ctx(c.ctx)
	oldTarget := c.currentTarget
	c.currentTarget = clientID

	if clientID == "" {
		// Cancel existing grab timer
		if c.grabTimer != nil {
			c.grabTimer.Stop()
			c.grabTimer = nil
		}

		// Release all devices
		var releaseErrors []string
		var releaseCount int
		for _, handler := range c.devices {
			if handler.device != nil && handler.grabbed {
				if err := handler.device.Release(); err != nil {
					releaseErrors = append(releaseErrors, fmt.Sprintf("%s: %v", handler.path, err))
					log.Error().Err(err).Str("device", handler.path).Msg("failed to release device")
				} else {
					handler.grabbed = false
					releaseCount++
				}
			}
		}

		if len(releaseErrors) > 0 {
			log.Error().Int("count", len(releaseErrors)).Msg("failed to release some devices, forcing")
			c.forceReleaseDevices()
		} else if releaseCount > 0 {
			log.Info().Int("count", releaseCount).Msg("released devices")
		}
		log.Info().Msg("input capture target cleared - controlling local system")
	} else {
		if !c.noGrab {
			// Grab all devices
			var grabErrors []string
			var successCount int
			for _, handler := range c.devices {
				if handler.device != nil && !handler.grabbed {
					if err := handler.device.Grab(); err != nil {
						grabErrors = append(grabErrors, fmt.Sprintf("%s: %v", handler.path, err))
						log.Warn().Err(err).Str("device", handler.name).Str("path", handler.path).Msg("failed to grab device")
					} else {
						handler.grabbed = true
						successCount++
						log.Debug().Str("device", handler.name).Str("path", handler.path).Msg("grabbed device")
					}
				}
			}
			log.Info().Int("grabbed", successCount).Int("total", len(c.devices)).Msg("grabbed devices")

			if len(grabErrors) > 0 {
				c.currentTarget = oldTarget
				return fmt.Errorf("failed to grab input devices: %s", strings.Join(grabErrors, ", "))
			}

			// Set up safety timeout
			c.lastActivity = time.Now()
			c.grabTimer = time.AfterFunc(c.grabTimeout, func() {
				c.handleSafetyTimeout()
			})

			log.Info().Str("target", clientID).Dur("timeout", c.grabTimeout).Msg("set input capture target")
		} else {
			log.Info().Str("target", clientID).Msg("set input capture target (no-grab mode)")
		}
	}
	return nil
}

// SetEventCallback sets the callback for captured input events.
func (c *Capture) SetEventCallback(callback func(*domain.InputEvent)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onInputEvent = callback
	zerolog.Ctx(c.ctx).Info().Msg("input event callback set")
}

// SetGrabTimeout sets the safety timeout for device grabbing.
func (c *Capture) SetGrabTimeout(timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.grabTimeout = timeout
}

// SetEmergencyKey sets the key code for emergency release (requires Ctrl).
func (c *Capture) SetEmergencyKey(keyCode uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.emergencyKey = keyCode
}

// SetNoGrab enables or disables no-grab mode (non-exclusive capture).
func (c *Capture) SetNoGrab(noGrab bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.noGrab = noGrab
}

// SetEmergencyHandler sets a callback for emergency release events.
func (c *Capture) SetEmergencyHandler(handler func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.emergencyHandler = handler
}

// handleSafetyTimeout handles the automatic device release on timeout.
func (c *Capture) handleSafetyTimeout() {
	log := zerolog.Ctx(c.ctx)
	log.Warn().Msg("safety timeout reached - auto-releasing devices")

	c.mu.Lock()
	if c.currentTarget != "" {
		c.currentTarget = ""
		var releaseErrors []string
		for _, handler := range c.devices {
			if handler.device != nil && handler.grabbed {
				if err := handler.device.Release(); err != nil {
					releaseErrors = append(releaseErrors, fmt.Sprintf("%s: %v", handler.path, err))
					log.Error().Err(err).Str("device", handler.path).Msg("failed to release device in safety timeout")
				} else {
					handler.grabbed = false
				}
			}
		}
		if len(releaseErrors) > 0 {
			log.Error().Int("count", len(releaseErrors)).Msg("failed to release devices in safety timeout")
			c.forceReleaseDevices()
		}
		if c.emergencyHandler != nil {
			c.mu.Unlock()
			log.Warn().Msg("emergency handler triggered from safety timeout")
			c.emergencyHandler()
			return
		}
	}
	c.mu.Unlock()
}

// discoverAndStartDevices finds all input devices and starts capturing.
func (c *Capture) discoverAndStartDevices() error {
	log := zerolog.Ctx(c.ctx)
	eventDir := "/dev/input"

	entries, err := os.ReadDir(eventDir)
	if err != nil {
		return fmt.Errorf("failed to read input directory: %w", err)
	}

	log.Info().Str("dir", eventDir).Msg("discovering input devices")
	deviceCount := 0

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "event") {
			path := filepath.Join(eventDir, entry.Name())

			if c.ignoredDevices[path] {
				continue
			}

			if err := c.addDevice(path); err != nil {
				c.ignoredDevices[path] = true
				if strings.Contains(err.Error(), "no relevant input capabilities") {
					log.Debug().Str("path", path).Msg("device not suitable for capture")
				} else if strings.Contains(err.Error(), "permission denied") {
					log.Warn().Str("path", path).Msg("permission denied - run as root with: sudo waymon server")
				} else {
					log.Warn().Err(err).Str("path", path).Msg("failed to add device")
				}
			} else {
				deviceCount++
			}
		}
	}

	ignoredCount := len(c.ignoredDevices)

	if deviceCount == 0 {
		if ignoredCount > 0 {
			return fmt.Errorf("no usable input devices found (%d devices ignored). Run as root: sudo waymon server", ignoredCount)
		}
		return fmt.Errorf("no input devices found in /dev/input/")
	}

	log.Info().Int("active", deviceCount).Int("ignored", ignoredCount).Msg("started capturing from input devices")
	return nil
}

// addDevice adds a new input device for monitoring.
func (c *Capture) addDevice(path string) error {
	log := zerolog.Ctx(c.ctx)

	if _, exists := c.devices[path]; exists {
		return nil
	}

	device, err := evdev.Open(path)
	if err != nil {
		log.Debug().Err(err).Str("path", path).Msg("cannot open device")
		return fmt.Errorf("failed to open device %s: %w", path, err)
	}

	if !c.isValidInputDevice(device) {
		if err := device.File.Close(); err != nil {
			log.Error().Err(err).Str("path", path).Msg("failed to close device")
		}
		return fmt.Errorf("device %s has no relevant input capabilities", path)
	}

	handler := &deviceHandler{
		path:   path,
		device: device,
		name:   device.Name,
	}

	handlerCtx, handlerCancel := context.WithCancel(c.ctx)
	handler.cancel = handlerCancel

	c.devices[path] = handler

	go c.captureFromDevice(handlerCtx, handler)

	log.Info().Str("name", handler.name).Str("path", path).Msg("added input device")
	return nil
}

// removeDevice removes a device from monitoring.
func (c *Capture) removeDevice(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	log := zerolog.Ctx(c.ctx)

	handler, exists := c.devices[path]
	if !exists {
		return
	}

	c.stopDeviceHandler(handler)
	delete(c.devices, path)
	log.Info().Str("path", path).Msg("removed input device")
}

// stopDeviceHandler stops a device handler.
func (c *Capture) stopDeviceHandler(handler *deviceHandler) {
	log := zerolog.Ctx(c.ctx)

	if handler.cancel != nil {
		handler.cancel()
	}
	if handler.device != nil && handler.grabbed {
		if err := handler.device.Release(); err != nil {
			log.Error().Err(err).Str("path", handler.path).Msg("failed to release device")
		}
		handler.grabbed = false
	}
	if handler.device != nil {
		if err := handler.device.File.Close(); err != nil {
			log.Error().Err(err).Str("path", handler.path).Msg("failed to close device")
		}
		handler.device = nil
	}
}

// isValidInputDevice checks if a device has input capabilities we care about.
func (c *Capture) isValidInputDevice(device *evdev.InputDevice) bool {
	deviceName := strings.ToLower(device.Name)

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
			return false
		}
	}

	capabilities := device.Capabilities

	for capType, caps := range capabilities {
		switch capType.Type {
		case 1: // EV_KEY
			for _, cap := range caps {
				// Mouse buttons
				if cap.Code >= evdev.BTN_LEFT && cap.Code <= evdev.BTN_TASK {
					return true
				}
				// Keyboard keys
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

// monitorDeviceChanges monitors for device addition/removal.
func (c *Capture) monitorDeviceChanges() {
	log := zerolog.Ctx(c.ctx)

	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Msg("device monitor panic")
		}
	}()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.checkForDeviceChanges()
		}
	}
}

// checkForDeviceChanges checks for new or removed devices.
func (c *Capture) checkForDeviceChanges() {
	log := zerolog.Ctx(c.ctx)
	eventDir := "/dev/input"

	entries, err := os.ReadDir(eventDir)
	if err != nil {
		log.Warn().Err(err).Msg("failed to read input directory during device monitoring")
		return
	}

	currentDevices := make(map[string]bool)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "event") {
			path := filepath.Join(eventDir, entry.Name())
			currentDevices[path] = true

			c.mu.RLock()
			_, exists := c.devices[path]
			ignored := c.ignoredDevices[path]
			c.mu.RUnlock()

			if !exists && !ignored {
				c.mu.Lock()
				if err := c.addDevice(path); err != nil {
					c.ignoredDevices[path] = true
					log.Debug().Err(err).Str("path", path).Msg("device not suitable, adding to ignore list")
				}
				c.mu.Unlock()
			}
		}
	}

	// Remove devices that no longer exist
	c.mu.RLock()
	devicePaths := make([]string, 0, len(c.devices))
	for path := range c.devices {
		devicePaths = append(devicePaths, path)
	}
	ignoredPaths := make([]string, 0, len(c.ignoredDevices))
	for path := range c.ignoredDevices {
		ignoredPaths = append(ignoredPaths, path)
	}
	c.mu.RUnlock()

	for _, path := range devicePaths {
		if !currentDevices[path] {
			c.removeDevice(path)
		}
	}

	for _, path := range ignoredPaths {
		if !currentDevices[path] {
			c.mu.Lock()
			delete(c.ignoredDevices, path)
			c.mu.Unlock()
			log.Debug().Str("path", path).Msg("removed from ignore list (device gone)")
		}
	}
}

// captureFromDevice captures events from a specific device.
func (c *Capture) captureFromDevice(ctx context.Context, handler *deviceHandler) {
	log := zerolog.Ctx(ctx)

	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("path", handler.path).Msg("device capture panic")
		}
	}()

	log.Debug().Str("name", handler.name).Str("path", handler.path).Msg("starting capture from device")

	// Variables to accumulate relative movements
	var accX, accY int32
	ticker := time.NewTicker(16 * time.Millisecond) // ~60 FPS
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Debug().Str("path", handler.path).Msg("device capture context cancelled")
			return
		case <-ticker.C:
			// Send accumulated movement if any
			if accX != 0 || accY != 0 {
				c.sendEvent(&domain.InputEvent{
					MouseMove: &domain.MouseMoveEvent{
						DX: float64(accX),
						DY: float64(accY),
					},
					Timestamp: time.Now().UnixNano(),
					SourceID:  fmt.Sprintf("evdev-%s", filepath.Base(handler.path)),
				})
				accX, accY = 0, 0
			}
		default:
			events, err := handler.device.Read()
			if err != nil {
				if !strings.Contains(err.Error(), "resource temporarily unavailable") {
					log.Error().Err(err).Str("path", handler.path).Msg("error reading events")
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
						c.sendScrollEvent(0, float64(event.Value))
					case evdev.REL_HWHEEL:
						c.sendScrollEvent(float64(event.Value), 0)
					}
				case evdev.EV_KEY:
					c.handleKeyEvent(event, handler)
				case evdev.EV_SYN, evdev.EV_MSC:
					// Ignore sync and misc events
				}
			}
		}
	}
}

// handleKeyEvent handles a key event (mouse button or keyboard).
func (c *Capture) handleKeyEvent(event evdev.InputEvent, handler *deviceHandler) {
	log := zerolog.Ctx(c.ctx)

	// Track Ctrl key state
	if event.Code == evdev.KEY_LEFTCTRL || event.Code == evdev.KEY_RIGHTCTRL {
		c.mu.Lock()
		c.ctrlPressed = (event.Value == 1)
		c.mu.Unlock()
		log.Debug().Bool("pressed", event.Value == 1).Msg("ctrl key state changed")
	}

	// Check for emergency release key combination (Ctrl+ESC)
	c.mu.RLock()
	emergencyKey := c.emergencyKey
	currentTarget := c.currentTarget
	ctrlPressed := c.ctrlPressed
	c.mu.RUnlock()

	if event.Code == emergencyKey {
		log.Debug().
			Int32("value", event.Value).
			Bool("ctrlPressed", ctrlPressed).
			Str("target", currentTarget).
			Msg("escape key detected")
	}

	if event.Code == emergencyKey && event.Value == 1 && ctrlPressed {
		log.Warn().Msg("emergency release triggered - Ctrl+ESC pressed")
		go func() {
			if err := c.SetTarget(""); err != nil {
				log.Error().Err(err).Msg("failed to release on emergency")
				c.mu.Lock()
				c.forceReleaseDevices()
				c.mu.Unlock()
			}
			if c.emergencyHandler != nil {
				log.Debug().Msg("notifying emergency handler")
				c.emergencyHandler()
			}
		}()
		return
	}

	if event.Code >= evdev.BTN_LEFT && event.Code <= evdev.BTN_TASK {
		c.sendMouseButtonEvent(event.Code, event.Value)
	} else {
		c.sendKeyboardEvent(event.Code, event.Value)
	}
}

// sendEvent sends an event to the event channel.
func (c *Capture) sendEvent(event *domain.InputEvent) {
	log := zerolog.Ctx(c.ctx)

	// Update activity timestamp and reset timer if we have an active grab
	c.mu.Lock()
	if c.currentTarget != "" {
		c.lastActivity = time.Now()
		if c.grabTimer != nil {
			c.grabTimer.Reset(c.grabTimeout)
		}
	}
	c.mu.Unlock()

	// Try to send with priority for important events
	if event.Control != nil || event.Keyboard != nil {
		select {
		case c.eventChan <- event:
		case <-time.After(10 * time.Millisecond):
			log.Error().Str("type", fmt.Sprintf("%T", event)).Msg("event channel full, dropping important event")
			select {
			case <-c.eventChan:
				c.eventChan <- event
			default:
			}
		}
	} else {
		select {
		case c.eventChan <- event:
		default:
			log.Debug().Msg("event channel full, dropping non-critical event")
		}
	}
}

// sendMouseButtonEvent sends a mouse button event.
func (c *Capture) sendMouseButtonEvent(code uint16, value int32) {
	log := zerolog.Ctx(c.ctx)

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
		if code >= evdev.BTN_LEFT && code <= evdev.BTN_TASK {
			button = uint32(code - evdev.BTN_LEFT + 1)
		} else {
			log.Warn().Uint16("code", code).Msg("unknown button code")
			return
		}
	}

	c.sendEvent(&domain.InputEvent{
		MouseButton: &domain.MouseButtonEvent{
			Button:  button,
			Pressed: value == 1,
		},
		Timestamp: time.Now().UnixNano(),
		SourceID:  "evdev-capture",
	})
}

// sendScrollEvent sends a mouse scroll event.
func (c *Capture) sendScrollEvent(dx, dy float64) {
	c.sendEvent(&domain.InputEvent{
		MouseScroll: &domain.MouseScrollEvent{
			DX:   dx,
			DY:   dy,
			Type: domain.ScrollWheel,
		},
		Timestamp: time.Now().UnixNano(),
		SourceID:  "evdev-capture",
	})
}

// sendKeyboardEvent sends a keyboard event.
func (c *Capture) sendKeyboardEvent(code uint16, value int32) {
	log := zerolog.Ctx(c.ctx)

	// Log modifier key state changes for debugging
	switch code {
	case evdev.KEY_LEFTSHIFT, evdev.KEY_RIGHTSHIFT:
		log.Debug().Bool("pressed", value > 0).Msg("shift key state changed")
	case evdev.KEY_LEFTALT, evdev.KEY_RIGHTALT:
		log.Debug().Bool("pressed", value > 0).Msg("alt key state changed")
	case evdev.KEY_LEFTMETA, evdev.KEY_RIGHTMETA:
		log.Debug().Bool("pressed", value > 0).Msg("super/meta key state changed")
	}

	// value: 0 = release, 1 = press, 2 = autorepeat (treat as press)
	c.sendEvent(&domain.InputEvent{
		Keyboard: &domain.KeyboardEvent{
			Key:     uint32(code),
			Pressed: value > 0,
		},
		Timestamp: time.Now().UnixNano(),
		SourceID:  "evdev-capture",
	})
}

// processEvents processes events from the event channel.
func (c *Capture) processEvents() {
	log := zerolog.Ctx(c.ctx)

	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Msg("event processor panic")
		}
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case event, ok := <-c.eventChan:
			if !ok {
				return
			}

			c.mu.RLock()
			target := c.currentTarget
			callback := c.onInputEvent
			c.mu.RUnlock()

			if target != "" && callback != nil {
				log.Debug().Str("target", target).Msg("forwarding event to callback")
				callback(event)
			} else if callback == nil && target != "" {
				log.Warn().Msg("no callback set for input events")
			}
		}
	}
}

// forceReleaseDevices forcefully releases all devices by closing and reopening them.
func (c *Capture) forceReleaseDevices() {
	log := zerolog.Ctx(c.ctx)
	log.Warn().Msg("force releasing all devices by closing and reopening")

	for _, handler := range c.devices {
		if handler.device != nil {
			if err := handler.device.File.Close(); err != nil {
				log.Error().Err(err).Str("path", handler.path).Msg("failed to close device during force release")
			}
			handler.device = nil
			handler.grabbed = false
		}
	}

	c.devices = make(map[string]*deviceHandler)

	if err := c.discoverAndStartDevices(); err != nil {
		log.Error().Err(err).Msg("failed to rediscover devices after force release")
	}
}

// watchdog monitors device state and ensures recovery from inconsistent states.
func (c *Capture) watchdog() {
	log := zerolog.Ctx(c.ctx)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.validateAndRecoverState()
		case <-c.watchdogStop:
			log.Info().Msg("watchdog stopped")
			return
		}
	}
}

// validateAndRecoverState checks device state consistency and recovers if needed.
func (c *Capture) validateAndRecoverState() {
	log := zerolog.Ctx(c.ctx)

	c.mu.RLock()
	target := c.currentTarget
	deviceCount := len(c.devices)
	grabbedCount := 0
	var inconsistentDevices []string

	for path, handler := range c.devices {
		if handler.grabbed {
			grabbedCount++
			if handler.device == nil {
				inconsistentDevices = append(inconsistentDevices, path)
			}
		}
	}
	lastActivityAge := time.Since(c.lastActivity)
	c.mu.RUnlock()

	// Fix inconsistent device states
	if len(inconsistentDevices) > 0 {
		log.Error().Int("count", len(inconsistentDevices)).Msg("found devices marked grabbed but with nil handle")
		c.mu.Lock()
		for _, path := range inconsistentDevices {
			if handler, ok := c.devices[path]; ok {
				handler.grabbed = false
			}
		}
		c.mu.Unlock()
	}

	// Check for inconsistent state
	if target == "" && grabbedCount > 0 {
		log.Error().Int("grabbed", grabbedCount).Msg("inconsistent state - no target but devices grabbed")
		c.mu.Lock()
		c.forceReleaseDevices()
		c.mu.Unlock()
	}

	// Check for stuck grab (no activity for extended period)
	if target != "" && lastActivityAge > 60*time.Second {
		log.Warn().Dur("inactivity", lastActivityAge).Msg("no activity with devices grabbed - forcing release")
		c.mu.Lock()
		if err := c.SetTarget(""); err != nil {
			log.Error().Err(err).Msg("failed to clear target during watchdog release")
		}
		if c.emergencyHandler != nil {
			c.emergencyHandler()
		}
		c.mu.Unlock()
	}

	// Verify all expected devices are present
	if target != "" && grabbedCount == 0 && deviceCount > 0 {
		log.Warn().Msg("target set but no devices grabbed - attempting to re-grab")
		c.mu.Lock()
		oldTarget := c.currentTarget
		if err := c.SetTarget(""); err != nil {
			log.Error().Err(err).Msg("failed to clear target during re-grab")
		}
		if err := c.SetTarget(oldTarget); err != nil {
			log.Error().Err(err).Msg("failed to restore target during re-grab")
		}
		c.mu.Unlock()
	}

	if grabbedCount > 0 {
		log.Debug().
			Int("grabbed", grabbedCount).
			Int("total", deviceCount).
			Str("target", target).
			Dur("lastActivity", lastActivityAge).
			Msg("watchdog status")
	}
}
