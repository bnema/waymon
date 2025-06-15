package input

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gvalkov/golang-evdev"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/protocol"
)

// EvdevCapture captures input events from evdev devices
type EvdevCapture struct {
	mouseDevice    *evdev.InputDevice
	keyboardDevice *evdev.InputDevice
	onInputEvent   func(*protocol.InputEvent)
	currentTarget  string
	capturing      bool
	ctx            context.Context
	cancel         context.CancelFunc
	mousePath      string // Configured mouse device path
	keyboardPath   string // Configured keyboard device path
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

	logger.Info("Started evdev input capture")
	return nil
}

// Stop stops capturing input events
func (e *EvdevCapture) Stop() error {
	if !e.capturing {
		return nil
	}

	// Cancel context to stop goroutines
	if e.cancel != nil {
		e.cancel()
	}

	// Close devices
	if e.mouseDevice != nil {
		e.mouseDevice.File.Close()
		e.mouseDevice = nil
	}
	if e.keyboardDevice != nil {
		e.keyboardDevice.File.Close()
		e.keyboardDevice = nil
	}

	e.capturing = false
	logger.Info("Stopped evdev input capture")
	return nil
}

// SetTarget sets the target client ID for input events
func (e *EvdevCapture) SetTarget(clientID string) error {
	e.currentTarget = clientID
	logger.Infof("Set input capture target to client: %s", clientID)
	return nil
}

// OnInputEvent sets the callback for input events
func (e *EvdevCapture) OnInputEvent(callback func(*protocol.InputEvent)) {
	e.onInputEvent = callback
}

// findMouseDevice finds the first available mouse device
func (e *EvdevCapture) findMouseDevice() (*evdev.InputDevice, error) {
	devices, err := evdev.ListInputDevices("/dev/input/event*")
	if err != nil {
		return nil, err
	}

	for _, dev := range devices {
		// Check if device has relative axes (mouse movement)
		if dev.Capabilities != nil {
			// Check in the flat capabilities map instead
			if relAxes, ok := dev.CapabilitiesFlat[evdev.EV_REL]; ok && len(relAxes) > 0 {
				// Check for X and Y axes
				hasX := false
				hasY := false
				for _, axis := range relAxes {
					if axis == evdev.REL_X {
						hasX = true
					}
					if axis == evdev.REL_Y {
						hasY = true
					}
				}
				if hasX && hasY {
					logger.Infof("Found mouse device: %s (%s)", dev.Name, dev.Fn)
					return dev, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no mouse device found")
}

// findKeyboardDevice finds the first available keyboard device
func (e *EvdevCapture) findKeyboardDevice() (*evdev.InputDevice, error) {
	devices, err := evdev.ListInputDevices("/dev/input/event*")
	if err != nil {
		return nil, err
	}

	for _, dev := range devices {
		// Check if device has key events and looks like a keyboard
		if dev.Capabilities != nil {
			if keys, ok := dev.CapabilitiesFlat[evdev.EV_KEY]; ok && len(keys) > 50 {
				// Simple heuristic: keyboards have many keys
				if strings.Contains(strings.ToLower(dev.Name), "keyboard") {
					logger.Infof("Found keyboard device: %s (%s)", dev.Name, dev.Fn)
					return dev, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no keyboard device found")
}

// captureMouseEvents captures mouse events from the device
func (e *EvdevCapture) captureMouseEvents() {
	defer logger.Debug("Mouse capture goroutine exited")

	for {
		select {
		case <-e.ctx.Done():
			return
		default:
			// Read event with timeout
			event, err := e.mouseDevice.ReadOne()
			if err != nil {
				if os.IsTimeout(err) {
					continue
				}
				logger.Errorf("Error reading mouse event: %v", err)
				return
			}

			// Only process if we have a target and callback
			if e.currentTarget == "" || e.onInputEvent == nil {
				continue
			}

			// Process the event
			e.processMouseEvent(event)
		}
	}
}

// processMouseEvent processes a single mouse event
func (e *EvdevCapture) processMouseEvent(event *evdev.InputEvent) {
	timestamp := time.Now().UnixNano()

	switch event.Type {
	case evdev.EV_REL:
		// Relative movement
		switch event.Code {
		case evdev.REL_X, evdev.REL_Y:
			// Accumulate movement and send on SYN
			// For now, send immediately
			if event.Code == evdev.REL_X {
				e.sendMouseMove(float64(event.Value), 0, timestamp)
			} else {
				e.sendMouseMove(0, float64(event.Value), timestamp)
			}
		case evdev.REL_WHEEL:
			// Vertical scroll
			e.sendMouseScroll(0, float64(event.Value), timestamp)
		case evdev.REL_HWHEEL:
			// Horizontal scroll
			e.sendMouseScroll(float64(event.Value), 0, timestamp)
		}

	case evdev.EV_KEY:
		// Mouse button events
		switch event.Code {
		case evdev.BTN_LEFT:
			e.sendMouseButton(1, event.Value != 0, timestamp)
		case evdev.BTN_RIGHT:
			e.sendMouseButton(3, event.Value != 0, timestamp)
		case evdev.BTN_MIDDLE:
			e.sendMouseButton(2, event.Value != 0, timestamp)
		}
	}
}

// sendMouseMove sends a mouse move event
func (e *EvdevCapture) sendMouseMove(dx, dy float64, timestamp int64) {
	if e.onInputEvent == nil {
		return
	}

	event := &protocol.InputEvent{
		Event: &protocol.InputEvent_MouseMove{
			MouseMove: &protocol.MouseMoveEvent{
				Dx: dx,
				Dy: dy,
			},
		},
		Timestamp: timestamp,
		SourceId:  "server",
	}

	e.onInputEvent(event)
}

// sendMouseButton sends a mouse button event
func (e *EvdevCapture) sendMouseButton(button uint32, pressed bool, timestamp int64) {
	if e.onInputEvent == nil {
		return
	}

	event := &protocol.InputEvent{
		Event: &protocol.InputEvent_MouseButton{
			MouseButton: &protocol.MouseButtonEvent{
				Button:  button,
				Pressed: pressed,
			},
		},
		Timestamp: timestamp,
		SourceId:  "server",
	}

	e.onInputEvent(event)
}

// sendMouseScroll sends a mouse scroll event
func (e *EvdevCapture) sendMouseScroll(dx, dy float64, timestamp int64) {
	if e.onInputEvent == nil {
		return
	}

	event := &protocol.InputEvent{
		Event: &protocol.InputEvent_MouseScroll{
			MouseScroll: &protocol.MouseScrollEvent{
				Dx: dx,
				Dy: dy,
			},
		},
		Timestamp: timestamp,
		SourceId:  "server",
	}

	e.onInputEvent(event)
}

// captureKeyboardEvents captures keyboard events from the device
func (e *EvdevCapture) captureKeyboardEvents() {
	defer logger.Debug("Keyboard capture goroutine exited")

	for {
		select {
		case <-e.ctx.Done():
			return
		default:
			// Read event with timeout
			event, err := e.keyboardDevice.ReadOne()
			if err != nil {
				if os.IsTimeout(err) {
					continue
				}
				logger.Errorf("Error reading keyboard event: %v", err)
				return
			}

			// Only process if we have a target and callback
			if e.currentTarget == "" || e.onInputEvent == nil {
				continue
			}

			// Process keyboard events
			if event.Type == evdev.EV_KEY {
				e.sendKeyboardEvent(uint32(event.Code), event.Value != 0, time.Now().UnixNano())
			}
		}
	}
}

// sendKeyboardEvent sends a keyboard event
func (e *EvdevCapture) sendKeyboardEvent(key uint32, pressed bool, timestamp int64) {
	if e.onInputEvent == nil {
		return
	}

	event := &protocol.InputEvent{
		Event: &protocol.InputEvent_Keyboard{
			Keyboard: &protocol.KeyboardEvent{
				Key:       key,
				Pressed:   pressed,
				Modifiers: 0, // TODO: Track modifier state
			},
		},
		Timestamp: timestamp,
		SourceId:  "server",
	}

	e.onInputEvent(event)
}