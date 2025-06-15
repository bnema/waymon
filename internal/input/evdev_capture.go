package input

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/protocol"
	"github.com/gvalkov/golang-evdev"
)

// EvdevCapture captures input events from evdev devices
type EvdevCapture struct {
	mu             sync.RWMutex
	mouseDevice    *evdev.InputDevice
	keyboardDevice *evdev.InputDevice
	onInputEvent   func(*protocol.InputEvent)
	currentTarget  string
	capturing      bool
	ctx            context.Context
	cancel         context.CancelFunc
	mousePath      string // Configured mouse device path
	keyboardPath   string // Configured keyboard device path
	devicesGrabbed bool   // Track if devices are grabbed
}

// NewEvdevCapture creates a new evdev-based input capture
func NewEvdevCapture() *EvdevCapture {
	return &EvdevCapture{
		capturing: false,
	}
}

// NewEvdevCaptureWithDevices creates a new evdev capture with specific device paths
func NewEvdevCaptureWithDevices(mousePath, keyboardPath string) *EvdevCapture {
	return &EvdevCapture{
		capturing:    false,
		mousePath:    mousePath,
		keyboardPath: keyboardPath,
	}
}

// Start starts capturing input events
func (e *EvdevCapture) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.capturing {
		return fmt.Errorf("already capturing")
	}

	// Use configured device paths if available
	if e.mousePath != "" {
		mouseDevice, err := evdev.Open(e.mousePath)
		if err != nil {
			return fmt.Errorf("failed to open configured mouse device %s: %w", e.mousePath, err)
		}
		e.mouseDevice = mouseDevice
		logger.Infof("Using configured mouse device: %s", e.mousePath)
	} else {
		// Find mouse device automatically
		mouseDevice, err := e.findMouseDevice()
		if err != nil {
			return fmt.Errorf("failed to find mouse device: %w", err)
		}
		e.mouseDevice = mouseDevice
	}

	// Handle keyboard device
	if e.keyboardPath != "" {
		keyboardDevice, err := evdev.Open(e.keyboardPath)
		if err != nil {
			logger.Warnf("Failed to open configured keyboard device %s: %v", e.keyboardPath, err)
			// Don't fail if keyboard not found, mouse is more important
		} else {
			e.keyboardDevice = keyboardDevice
			logger.Infof("Using configured keyboard device: %s", e.keyboardPath)
		}
	} else {
		// Find keyboard device automatically
		keyboardDevice, err := e.findKeyboardDevice()
		if err != nil {
			logger.Warnf("Failed to find keyboard device: %v", err)
			// Don't fail if keyboard not found, mouse is more important
		} else {
			e.keyboardDevice = keyboardDevice
		}
	}

	// Create cancellable context
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.capturing = true

	// Start capture goroutines
	if e.mouseDevice != nil {
		go e.captureMouseEvents()
	}
	if e.keyboardDevice != nil {
		go e.captureKeyboardEvents()
	}

	logger.Info("Evdev input capture started")
	return nil
}

// Stop stops capturing input events
func (e *EvdevCapture) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.capturing {
		return nil
	}

	// Cancel context to stop capture goroutines
	if e.cancel != nil {
		e.cancel()
	}

	// Release grabbed devices
	if e.devicesGrabbed {
		e.ungrabDevices()
	}

	// Clear device references
	e.mouseDevice = nil
	e.keyboardDevice = nil

	e.capturing = false
	logger.Info("Evdev input capture stopped")
	return nil
}

// SetTarget sets the target client ID for forwarding events
func (e *EvdevCapture) SetTarget(clientID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	oldTarget := e.currentTarget
	e.currentTarget = clientID

	// Handle device grabbing based on target
	if clientID == "" {
		// Release devices when controlling local system
		if e.devicesGrabbed {
			e.ungrabDevices()
		}
		logger.Info("Input capture target cleared - controlling local system")
	} else {
		// Grab devices when controlling a client
		if !e.devicesGrabbed {
			if err := e.grabDevices(); err != nil {
				// Revert target on grab failure
				e.currentTarget = oldTarget
				return fmt.Errorf("failed to grab input devices: %w", err)
			}
		}
		logger.Infof("Set input capture target to client: %s", clientID)
	}
	return nil
}

// OnInputEvent sets the callback for input events
func (e *EvdevCapture) OnInputEvent(callback func(*protocol.InputEvent)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onInputEvent = callback
	logger.Info("Evdev input event callback set")
}

// grabDevices grabs exclusive access to input devices
func (e *EvdevCapture) grabDevices() error {
	if e.mouseDevice != nil {
		if err := e.mouseDevice.Grab(); err != nil {
			return fmt.Errorf("failed to grab mouse device: %w", err)
		}
		logger.Debug("Grabbed exclusive access to mouse device")
	}
	if e.keyboardDevice != nil {
		if err := e.keyboardDevice.Grab(); err != nil {
			// Release mouse if keyboard grab fails
			if e.mouseDevice != nil {
				e.mouseDevice.Release()
			}
			return fmt.Errorf("failed to grab keyboard device: %w", err)
		}
		logger.Debug("Grabbed exclusive access to keyboard device")
	}
	e.devicesGrabbed = true
	return nil
}

// ungrabDevices releases exclusive access to input devices
func (e *EvdevCapture) ungrabDevices() {
	if e.mouseDevice != nil {
		e.mouseDevice.Release()
		logger.Debug("Released exclusive access to mouse device")
	}
	if e.keyboardDevice != nil {
		e.keyboardDevice.Release()
		logger.Debug("Released exclusive access to keyboard device")
	}
	e.devicesGrabbed = false
}

// findMouseDevice finds the first available mouse device
func (e *EvdevCapture) findMouseDevice() (*evdev.InputDevice, error) {
	// Use the device selector to get all devices, we'll use the first one
	selector := NewDeviceSelector()
	devices, err := selector.ListDevices(DeviceTypeMouse) // deviceType ignored now
	if err != nil {
		return nil, err
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("no suitable mouse device found")
	}

	// Use the first found device
	device, err := evdev.Open(devices[0].Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open mouse device %s: %w", devices[0].Path, err)
	}

	logger.Infof("Found mouse device: %s at %s", devices[0].Name, devices[0].Path)
	return device, nil
}

// findKeyboardDevice finds the first available keyboard device
func (e *EvdevCapture) findKeyboardDevice() (*evdev.InputDevice, error) {
	// Use the device selector to get all devices, we'll use the first one
	selector := NewDeviceSelector()
	devices, err := selector.ListDevices(DeviceTypeKeyboard) // deviceType ignored now
	if err != nil {
		return nil, err
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("no suitable keyboard device found")
	}

	// Use the first found device
	device, err := evdev.Open(devices[0].Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open keyboard device %s: %w", devices[0].Path, err)
	}

	logger.Infof("Found keyboard device: %s at %s", devices[0].Name, devices[0].Path)
	return device, nil
}

// captureMouseEvents captures mouse events from the device
func (e *EvdevCapture) captureMouseEvents() {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Mouse capture panic: %v", r)
		}
	}()

	logger.Debug("Starting mouse event capture")
	
	// Variables to accumulate relative movements
	var accX, accY int32
	ticker := time.NewTicker(16 * time.Millisecond) // ~60 FPS
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			logger.Debug("Mouse capture context cancelled")
			return
		case <-ticker.C:
			// Send accumulated movement if any
			e.mu.RLock()
			target := e.currentTarget
			callback := e.onInputEvent
			e.mu.RUnlock()

			if target != "" && callback != nil && (accX != 0 || accY != 0) {
				event := &protocol.InputEvent{
					Event: &protocol.InputEvent_MouseMove{
						MouseMove: &protocol.MouseMoveEvent{
							Dx: float64(accX),
							Dy: float64(accY),
						},
					},
					Timestamp: time.Now().UnixNano(),
					SourceId:  "evdev-capture",
				}
				callback(event)
				logger.Debugf("Sent accumulated mouse movement: dx=%d, dy=%d", accX, accY)
				accX, accY = 0, 0
			}
		default:
			// Read events with a small timeout
			events, err := e.mouseDevice.Read()
			if err != nil {
				if !strings.Contains(err.Error(), "resource temporarily unavailable") {
					logger.Errorf("Error reading mouse events: %v", err)
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
						e.handleMouseScroll(0, float64(event.Value))
					case evdev.REL_HWHEEL:
						e.handleMouseScroll(float64(event.Value), 0)
					}
				case evdev.EV_KEY:
					if event.Code >= evdev.BTN_LEFT && event.Code <= evdev.BTN_TASK {
						e.handleMouseButton(event.Code, event.Value)
					}
				}
			}
		}
	}
}

// captureKeyboardEvents captures keyboard events from the device
func (e *EvdevCapture) captureKeyboardEvents() {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Keyboard capture panic: %v", r)
		}
	}()

	logger.Debug("Starting keyboard event capture")

	for {
		select {
		case <-e.ctx.Done():
			logger.Debug("Keyboard capture context cancelled")
			return
		default:
			events, err := e.keyboardDevice.Read()
			if err != nil {
				if !strings.Contains(err.Error(), "resource temporarily unavailable") {
					logger.Errorf("Error reading keyboard events: %v", err)
				}
				time.Sleep(5 * time.Millisecond)
				continue
			}

			for _, event := range events {
				if event.Type == evdev.EV_KEY {
					e.handleKeyboardKey(event.Code, event.Value)
				}
			}
		}
	}
}

// handleMouseButton handles mouse button events
func (e *EvdevCapture) handleMouseButton(code uint16, value int32) {
	e.mu.RLock()
	target := e.currentTarget
	callback := e.onInputEvent
	e.mu.RUnlock()

	if target == "" || callback == nil {
		return
	}

	// Map evdev button codes to protocol button numbers
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
		// Unknown button
		return
	}

	event := &protocol.InputEvent{
		Event: &protocol.InputEvent_MouseButton{
			MouseButton: &protocol.MouseButtonEvent{
				Button:  button,
				Pressed: value == 1,
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "evdev-capture",
	}

	callback(event)
	logger.Debugf("Mouse button %d %s", button, map[bool]string{true: "pressed", false: "released"}[value == 1])
}

// handleMouseScroll handles mouse scroll events
func (e *EvdevCapture) handleMouseScroll(dx, dy float64) {
	e.mu.RLock()
	target := e.currentTarget
	callback := e.onInputEvent
	e.mu.RUnlock()

	if target == "" || callback == nil {
		return
	}

	event := &protocol.InputEvent{
		Event: &protocol.InputEvent_MouseScroll{
			MouseScroll: &protocol.MouseScrollEvent{
				Dx: dx,
				Dy: dy,
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "evdev-capture",
	}

	callback(event)
	logger.Debugf("Mouse scroll: dx=%.1f, dy=%.1f", dx, dy)
}

// handleKeyboardKey handles keyboard key events
func (e *EvdevCapture) handleKeyboardKey(code uint16, value int32) {
	e.mu.RLock()
	target := e.currentTarget
	callback := e.onInputEvent
	e.mu.RUnlock()

	if target == "" || callback == nil {
		return
	}

	// Only handle key press and release events (not repeat)
	if value != 0 && value != 1 {
		return
	}

	event := &protocol.InputEvent{
		Event: &protocol.InputEvent_Keyboard{
			Keyboard: &protocol.KeyboardEvent{
				Key:     uint32(code),
				Pressed: value == 1,
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "evdev-capture",
	}

	callback(event)
	logger.Debugf("Keyboard key %d (0x%X) %s", code, code, map[bool]string{true: "pressed", false: "released"}[value == 1])
}

// IsAvailable checks if evdev is available on this system
func IsEvdevAvailable() bool {
	// Check if /dev/input directory exists
	if _, err := os.Stat("/dev/input"); os.IsNotExist(err) {
		return false
	}

	// Try to list input devices
	devices, err := evdev.ListInputDevices("/dev/input/event*")
	if err != nil {
		return false
	}

	return len(devices) > 0
}