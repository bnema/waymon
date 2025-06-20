package input

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bnema/wayland-virtual-input-go/keyboard_shortcuts_inhibitor"
	"github.com/bnema/wayland-virtual-input-go/pointer_constraints"
	"github.com/bnema/wayland-virtual-input-go/virtual_keyboard"
	"github.com/bnema/wayland-virtual-input-go/virtual_pointer"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/protocol"
	"github.com/rajveermalviya/go-wayland/wayland/client"
)

// WaylandVirtualInput implements input capture using Wayland virtual input protocols
// This backend is designed for Hyprland and other wlroots-based compositors
type WaylandVirtualInput struct {
	// Virtual input for injection (server mode)
	pointerMgr  *virtual_pointer.VirtualPointerManager
	keyboardMgr *virtual_keyboard.VirtualKeyboardManager
	virtualPtr  *virtual_pointer.VirtualPointer
	virtualKbd  *virtual_keyboard.VirtualKeyboard

	// Wayland capture infrastructure
	display    *client.Display
	registry   *client.Registry //nolint:unused // part of wayland infrastructure, may be used in future
	seat       *client.Seat
	pointer    *client.Pointer
	keyboard   *client.Keyboard //nolint:unused // part of wayland infrastructure, may be used in future
	surface    *client.Surface
	compositor *client.Compositor //nolint:unused // part of wayland infrastructure, may be used in future

	// Pointer constraints for exclusive capture
	constraintsMgr pointer_constraints.PointerConstraintsManager
	lockedPointer  pointer_constraints.LockedPointer

	// Keyboard shortcuts inhibitor for exclusive keyboard capture
	shortcutsInhibitorMgr keyboard_shortcuts_inhibitor.KeyboardShortcutsInhibitorManager
	shortcutsInhibitor    keyboard_shortcuts_inhibitor.KeyboardShortcutsInhibitor

	onInputEvent  func(*protocol.InputEvent)
	currentTarget string
	capturing     bool
	mu            sync.RWMutex
	cancel        context.CancelFunc
}

// NewWaylandVirtualInput creates a new Wayland virtual input backend
func NewWaylandVirtualInput() (*WaylandVirtualInput, error) {
	w := &WaylandVirtualInput{}

	// Connect to Wayland display
	display, err := client.Connect("")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Wayland display: %w", err)
	}
	w.display = display

	// Get registry and bind to required globals
	if err := w.setupWaylandGlobals(); err != nil {
		if err := display.Destroy(); err != nil {
			logger.Errorf("Failed to destroy display: %v", err)
		}
		return nil, fmt.Errorf("failed to setup Wayland globals: %w", err)
	}

	ctx := context.Background()

	// Create virtual pointer manager for injection
	pointerMgr, err := virtual_pointer.NewVirtualPointerManager(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual pointer manager: %w", err)
	}
	w.pointerMgr = pointerMgr

	// Create virtual keyboard manager for injection
	keyboardMgr, err := virtual_keyboard.NewVirtualKeyboardManager(ctx)
	if err != nil {
		logger.Warn("Failed to create virtual keyboard manager, keyboard support will be limited")
	} else {
		w.keyboardMgr = keyboardMgr
	}

	// Create pointer constraints manager for exclusive capture
	constraintsMgr, err := pointer_constraints.NewPointerConstraintsManager(ctx)
	if err != nil {
		logger.Warn("Failed to create pointer constraints manager, exclusive pointer capture will be limited")
	} else {
		w.constraintsMgr = constraintsMgr
	}

	// Create keyboard shortcuts inhibitor manager for exclusive keyboard capture
	shortcutsInhibitorMgr, err := keyboard_shortcuts_inhibitor.NewKeyboardShortcutsInhibitorManager(ctx)
	if err != nil {
		logger.Warn("Failed to create keyboard shortcuts inhibitor manager, exclusive keyboard capture will be limited")
	} else {
		w.shortcutsInhibitorMgr = shortcutsInhibitorMgr
	}

	return w, nil
}

// setupWaylandGlobals binds to required Wayland globals
func (w *WaylandVirtualInput) setupWaylandGlobals() error {
	// For now, we'll provide a simplified setup that doesn't require complex protocol binding
	// This allows the backend to be created and tested, even if exclusive capture isn't fully functional yet
	logger.Info("Wayland globals setup - using simplified implementation")

	// The actual protocol binding would require proper integration with a Wayland client library
	// For now, we'll set up stub components to allow the backend to function for injection

	return nil
}

// Start begins the input backend (for clients: injection only)
func (w *WaylandVirtualInput) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.capturing {
		return fmt.Errorf("already started")
	}

	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.capturing = true

	// Create virtual pointer for injection
	if w.pointerMgr != nil {
		virtualPtr, err := w.pointerMgr.CreatePointer()
		if err != nil {
			logger.Warnf("Failed to create virtual pointer: %v", err)
		} else {
			w.virtualPtr = virtualPtr
			logger.Info("Virtual pointer created successfully")
		}
	}

	// Create virtual keyboard for injection
	if w.keyboardMgr != nil {
		virtualKbd, err := w.keyboardMgr.CreateKeyboard()
		if err != nil {
			logger.Warnf("Failed to create virtual keyboard: %v", err)
		} else {
			w.virtualKbd = virtualKbd
			logger.Info("Virtual keyboard created successfully")
		}
	}

	logger.Info("Wayland virtual input backend started")

	// Monitor context for shutdown
	go func() {
		<-ctx.Done()
		if err := w.Stop(); err != nil {
			logger.Errorf("Failed to stop Wayland backend: %v", err)
		}
	}()

	return nil
}

// Note: This backend is designed for CLIENT input injection only.
// Input capture is handled by the evdev backend on the server side.
// The Wayland virtual input protocols are for creating fake input devices, not capturing real input.

// enableExclusiveCapture enables exclusive pointer and keyboard capture
func (w *WaylandVirtualInput) enableExclusiveCapture() error {
	// Lock pointer to current position for exclusive capture
	if w.constraintsMgr != nil && w.surface != nil && w.pointer != nil {
		lockedPointer, err := pointer_constraints.LockPointerAtCurrentPosition(w.constraintsMgr, w.surface, w.pointer)
		if err != nil {
			logger.Warnf("Failed to lock pointer: %v", err)
		} else {
			w.lockedPointer = lockedPointer
			logger.Info("Pointer locked for exclusive capture")
		}
	}

	// Inhibit keyboard shortcuts for exclusive keyboard capture
	if w.shortcutsInhibitorMgr != nil && w.surface != nil && w.seat != nil {
		inhibitor, err := w.shortcutsInhibitorMgr.InhibitShortcuts(w.surface, w.seat)
		if err != nil {
			logger.Warnf("Failed to inhibit keyboard shortcuts: %v", err)
		} else {
			w.shortcutsInhibitor = inhibitor
			logger.Info("Keyboard shortcuts inhibited for exclusive capture")
		}
	}

	return nil
}

// disableExclusiveCapture disables exclusive pointer and keyboard capture
func (w *WaylandVirtualInput) disableExclusiveCapture() error {
	// Unlock pointer
	if w.lockedPointer != nil {
		if err := w.lockedPointer.Destroy(); err != nil {
			logger.Warnf("Failed to destroy locked pointer: %v", err)
		} else {
			logger.Info("Pointer unlocked - exclusive capture disabled")
		}
		w.lockedPointer = nil
	}

	// Re-enable keyboard shortcuts
	if w.shortcutsInhibitor != nil {
		if err := w.shortcutsInhibitor.Destroy(); err != nil {
			logger.Warnf("Failed to destroy shortcuts inhibitor: %v", err)
		} else {
			logger.Info("Keyboard shortcuts re-enabled")
		}
		w.shortcutsInhibitor = nil
	}

	return nil
}

// Stop stops capturing input events
func (w *WaylandVirtualInput) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.capturing {
		return nil
	}

	w.capturing = false

	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}

	// Clean up virtual devices
	if w.virtualPtr != nil {
		if err := w.virtualPtr.Close(); err != nil {
			logger.Errorf("Failed to close virtual pointer: %v", err)
		}
		w.virtualPtr = nil
	}

	if w.virtualKbd != nil {
		if err := w.virtualKbd.Close(); err != nil {
			logger.Errorf("Failed to close virtual keyboard: %v", err)
		}
		w.virtualKbd = nil
	}

	// Disable exclusive capture first
	if err := w.disableExclusiveCapture(); err != nil {
		logger.Errorf("Failed to disable exclusive capture: %v", err)
	}

	// Clean up managers
	if w.pointerMgr != nil {
		if err := w.pointerMgr.Close(); err != nil {
			logger.Errorf("Failed to close pointer manager: %v", err)
		}
		w.pointerMgr = nil
	}

	if w.keyboardMgr != nil {
		if err := w.keyboardMgr.Close(); err != nil {
			logger.Errorf("Failed to close keyboard manager: %v", err)
		}
		w.keyboardMgr = nil
	}

	if w.constraintsMgr != nil {
		if err := w.constraintsMgr.Destroy(); err != nil {
			logger.Errorf("Failed to destroy constraints manager: %v", err)
		}
		w.constraintsMgr = nil
	}

	if w.shortcutsInhibitorMgr != nil {
		if err := w.shortcutsInhibitorMgr.Destroy(); err != nil {
			logger.Errorf("Failed to destroy shortcuts inhibitor manager: %v", err)
		}
		w.shortcutsInhibitorMgr = nil
	}

	logger.Info("Wayland virtual input backend stopped")
	return nil
}

// SetTarget sets the target client ID for forwarding events
func (w *WaylandVirtualInput) SetTarget(clientID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	oldTarget := w.currentTarget
	w.currentTarget = clientID

	if clientID == "" {
		// Returning to local control - disable exclusive capture
		if oldTarget != "" {
			if err := w.disableExclusiveCapture(); err != nil {
				logger.Warnf("Failed to disable exclusive capture: %v", err)
			}
		}
		logger.Info("Wayland virtual input: control returned to local system")
	} else {
		// Switching to client control - enable exclusive capture
		if oldTarget == "" {
			if err := w.enableExclusiveCapture(); err != nil {
				logger.Warnf("Failed to enable exclusive capture: %v", err)
			}
		}
		logger.Infof("Wayland virtual input: forwarding events to client %s", clientID)
		// Note: This backend is used for CLIENT injection only.
		// Server-side input capture is handled by the evdev backend.
	}

	return nil
}

// OnInputEvent sets the callback for captured input events
// Note: This backend is for CLIENT injection only - it doesn't capture input
func (w *WaylandVirtualInput) OnInputEvent(callback func(*protocol.InputEvent)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onInputEvent = callback
	logger.Debug("Wayland virtual input: OnInputEvent callback set (used for client injection only)")
}

// InjectMouseMove injects a mouse move event
func (w *WaylandVirtualInput) InjectMouseMove(dx, dy float64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	logger.Debugf("[WAYLAND-INPUT] InjectMouseMove called: capturing=%v, virtualPtr=%v", w.capturing, w.virtualPtr != nil)

	if !w.capturing || w.virtualPtr == nil {
		return fmt.Errorf("virtual pointer not available (capturing=%v, virtualPtr=%v)", w.capturing, w.virtualPtr != nil)
	}

	// Use relative motion for mouse movement
	if err := w.virtualPtr.Motion(time.Now(), dx, dy); err != nil {
		return fmt.Errorf("failed to inject mouse motion: %w", err)
	}

	// Frame the event
	if err := w.virtualPtr.Frame(); err != nil {
		return fmt.Errorf("failed to frame mouse motion: %w", err)
	}

	logger.Debugf("[WAYLAND-INPUT] Successfully injected mouse move")
	return nil
}

// InjectMousePosition injects an absolute mouse position event
func (w *WaylandVirtualInput) InjectMousePosition(x, y uint32) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	logger.Debugf("[WAYLAND-INPUT] InjectMousePosition called: capturing=%v, virtualPtr=%v", w.capturing, w.virtualPtr != nil)

	if !w.capturing || w.virtualPtr == nil {
		return fmt.Errorf("virtual pointer not available (capturing=%v, virtualPtr=%v)", w.capturing, w.virtualPtr != nil)
	}

	// Use absolute motion for positioning
	if err := w.virtualPtr.MotionAbsolute(time.Now(), x, y, 1920, 1080); err != nil {
		return fmt.Errorf("failed to inject absolute mouse position: %w", err)
	}

	// Frame the event
	if err := w.virtualPtr.Frame(); err != nil {
		return fmt.Errorf("failed to frame absolute mouse position: %w", err)
	}

	logger.Debugf("[WAYLAND-INPUT] Successfully injected absolute mouse position")
	return nil
}

// InjectMouseButton injects a mouse button event (for server mode)
func (w *WaylandVirtualInput) InjectMouseButton(button uint32, pressed bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	logger.Debugf("[WAYLAND-INPUT] InjectMouseButton called: capturing=%v, virtualPtr=%v", w.capturing, w.virtualPtr != nil)

	if !w.capturing || w.virtualPtr == nil {
		return fmt.Errorf("virtual pointer not available (capturing=%v, virtualPtr=%v)", w.capturing, w.virtualPtr != nil)
	}

	// Convert protocol button numbers to Linux button codes
	var linuxButton uint32
	switch button {
	case 1:
		linuxButton = virtual_pointer.BTN_LEFT
	case 2:
		linuxButton = virtual_pointer.BTN_RIGHT
	case 3:
		linuxButton = virtual_pointer.BTN_MIDDLE
	case 4:
		linuxButton = virtual_pointer.BTN_SIDE
	case 5:
		linuxButton = virtual_pointer.BTN_EXTRA
	default:
		return fmt.Errorf("unsupported button number: %d", button)
	}

	// Convert button state
	var state virtual_pointer.ButtonState
	if pressed {
		state = virtual_pointer.ButtonStatePressed
	} else {
		state = virtual_pointer.ButtonStateReleased
	}

	// Inject button event
	if err := w.virtualPtr.Button(time.Now(), linuxButton, state); err != nil {
		return fmt.Errorf("failed to inject mouse button: %w", err)
	}

	// Frame the event
	return w.virtualPtr.Frame()
}

// InjectMouseScroll injects a mouse scroll event (for server mode)
func (w *WaylandVirtualInput) InjectMouseScroll(dx, dy float64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.capturing || w.virtualPtr == nil {
		return fmt.Errorf("virtual pointer not available")
	}

	// Set axis source to wheel
	if err := w.virtualPtr.AxisSource(virtual_pointer.AxisSourceWheel); err != nil {
		return fmt.Errorf("failed to set axis source: %w", err)
	}

	now := time.Now()

	// Inject vertical scroll if dy != 0
	if dy != 0 {
		if err := w.virtualPtr.Axis(now, virtual_pointer.AxisVertical, -dy); err != nil {
			return fmt.Errorf("failed to inject vertical scroll: %w", err)
		}
	}

	// Inject horizontal scroll if dx != 0
	if dx != 0 {
		if err := w.virtualPtr.Axis(now, virtual_pointer.AxisHorizontal, dx); err != nil {
			return fmt.Errorf("failed to inject horizontal scroll: %w", err)
		}
	}

	// Frame the event
	return w.virtualPtr.Frame()
}

// InjectKeyEvent injects a keyboard event (for server mode)
func (w *WaylandVirtualInput) InjectKeyEvent(key uint32, pressed bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.capturing || w.virtualKbd == nil {
		return fmt.Errorf("virtual keyboard not available")
	}

	// Convert key state
	var state virtual_keyboard.KeyState
	if pressed {
		state = virtual_keyboard.KeyStatePressed
	} else {
		state = virtual_keyboard.KeyStateReleased
	}

	// Inject key event
	return w.virtualKbd.Key(time.Now(), key, state)
}

// Input event handlers for capture

// handlePointerMotion handles pointer motion events
func (w *WaylandVirtualInput) handlePointerMotion(event client.PointerMotionEvent) { //nolint:unused // event handler kept for future use
	w.mu.RLock()
	target := w.currentTarget
	callback := w.onInputEvent
	w.mu.RUnlock()

	if target == "" || callback == nil {
		return // Only forward if we have a target
	}

	// Create mouse move event
	inputEvent := &protocol.InputEvent{
		Event: &protocol.InputEvent_MouseMove{
			MouseMove: &protocol.MouseMoveEvent{
				Dx: float64(event.SurfaceX), // TODO: Convert to relative movement
				Dy: float64(event.SurfaceY),
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "wayland-capture",
	}

	callback(inputEvent)
}

// handlePointerButton handles pointer button events
func (w *WaylandVirtualInput) handlePointerButton(event client.PointerButtonEvent) { //nolint:unused // event handler kept for future use
	w.mu.RLock()
	target := w.currentTarget
	callback := w.onInputEvent
	w.mu.RUnlock()

	if target == "" || callback == nil {
		return
	}

	// Create mouse button event
	inputEvent := &protocol.InputEvent{
		Event: &protocol.InputEvent_MouseButton{
			MouseButton: &protocol.MouseButtonEvent{
				Button:  event.Button,
				Pressed: event.State == 1, // WL_POINTER_BUTTON_STATE_PRESSED
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "wayland-capture",
	}

	callback(inputEvent)
}

// handlePointerAxis handles pointer axis (scroll) events
func (w *WaylandVirtualInput) handlePointerAxis(event client.PointerAxisEvent) { //nolint:unused // event handler kept for future use
	w.mu.RLock()
	target := w.currentTarget
	callback := w.onInputEvent
	w.mu.RUnlock()

	if target == "" || callback == nil {
		return
	}

	// Create mouse scroll event
	dx, dy := 0.0, 0.0
	if event.Axis == 0 { // WL_POINTER_AXIS_VERTICAL_SCROLL
		dy = float64(event.Value)
	} else { // WL_POINTER_AXIS_HORIZONTAL_SCROLL
		dx = float64(event.Value)
	}

	inputEvent := &protocol.InputEvent{
		Event: &protocol.InputEvent_MouseScroll{
			MouseScroll: &protocol.MouseScrollEvent{
				Dx: dx,
				Dy: dy,
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "wayland-capture",
	}

	callback(inputEvent)
}

// handlePointerEnter handles pointer enter events
func (w *WaylandVirtualInput) handlePointerEnter(event client.PointerEnterEvent) { //nolint:unused // event handler kept for future use
	// Could be used for edge detection later
	logger.Debug("Pointer entered surface")
}

// handlePointerLeave handles pointer leave events
func (w *WaylandVirtualInput) handlePointerLeave(event client.PointerLeaveEvent) { //nolint:unused // event handler kept for future use
	// Could be used for edge detection later
	logger.Debug("Pointer left surface")
}

// handleKeyboardKey handles keyboard key events
func (w *WaylandVirtualInput) handleKeyboardKey(event client.KeyboardKeyEvent) { //nolint:unused // event handler kept for future use
	w.mu.RLock()
	target := w.currentTarget
	callback := w.onInputEvent
	w.mu.RUnlock()

	if target == "" || callback == nil {
		return
	}

	// Create keyboard event
	inputEvent := &protocol.InputEvent{
		Event: &protocol.InputEvent_Keyboard{
			Keyboard: &protocol.KeyboardEvent{
				Key:       event.Key,
				Pressed:   event.State == 1, // WL_KEYBOARD_KEY_STATE_PRESSED
				Modifiers: 0,                // TODO: Track modifiers properly
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "wayland-capture",
	}

	callback(inputEvent)
}

// handleKeyboardKeymap handles keyboard keymap events
func (w *WaylandVirtualInput) handleKeyboardKeymap(event client.KeyboardKeymapEvent) { //nolint:unused // event handler kept for future use
	logger.Debug("Keyboard keymap updated")
}

// handleKeyboardEnter handles keyboard enter events
func (w *WaylandVirtualInput) handleKeyboardEnter(event client.KeyboardEnterEvent) { //nolint:unused // event handler kept for future use
	logger.Debug("Keyboard focus entered")
}

// handleKeyboardLeave handles keyboard leave events
func (w *WaylandVirtualInput) handleKeyboardLeave(event client.KeyboardLeaveEvent) { //nolint:unused // event handler kept for future use
	logger.Debug("Keyboard focus left")
}

// handleKeyboardModifiers handles keyboard modifier events
func (w *WaylandVirtualInput) handleKeyboardModifiers(event client.KeyboardModifiersEvent) { //nolint:unused // event handler kept for future use
	logger.Debug("Keyboard modifiers changed")
}
