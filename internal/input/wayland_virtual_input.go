package input

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/protocol"
	"github.com/bnema/wayland-virtual-input-go/virtual_pointer"
	"github.com/bnema/wayland-virtual-input-go/virtual_keyboard"
	"github.com/bnema/wayland-virtual-input-go/pointer_constraints"
	"github.com/bnema/wayland-virtual-input-go/keyboard_shortcuts_inhibitor"
	"github.com/rajveermalviya/go-wayland/wayland/client"
)

// WaylandVirtualInput implements input capture using Wayland virtual input protocols
// This backend is designed for Hyprland and other wlroots-based compositors
type WaylandVirtualInput struct {
	// Virtual input for injection (server mode)
	pointerMgr    virtual_pointer.VirtualPointerManager
	keyboardMgr   virtual_keyboard.VirtualKeyboardManager
	virtualPtr    virtual_pointer.VirtualPointer
	virtualKbd    virtual_keyboard.VirtualKeyboard
	
	// Wayland capture infrastructure
	display       *client.Display
	registry      *client.Registry
	seat          *client.Seat
	pointer       *client.Pointer
	keyboard      *client.Keyboard
	surface       *client.Surface
	compositor    *client.Compositor
	
	// Pointer constraints for exclusive capture
	constraintsMgr      pointer_constraints.PointerConstraintsManager
	lockedPointer       pointer_constraints.LockedPointer
	
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
		display.Destroy()
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

// Start begins capturing input events
func (w *WaylandVirtualInput) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.capturing {
		return fmt.Errorf("already capturing")
	}
	
	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.capturing = true
	
	// Set up input capture using Wayland seat
	if err := w.setupInputCapture(); err != nil {
		logger.Warnf("Failed to setup input capture: %v", err)
	}
	
	// Create virtual pointer for injection (server mode)
	if w.pointerMgr != nil {
		virtualPtr, err := w.pointerMgr.CreateVirtualPointer(nil)
		if err != nil {
			logger.Warnf("Failed to create virtual pointer: %v", err)
		} else {
			w.virtualPtr = virtualPtr
			logger.Info("Virtual pointer created successfully")
		}
	}
	
	// Create virtual keyboard for injection (server mode)
	if w.keyboardMgr != nil {
		virtualKbd, err := w.keyboardMgr.CreateVirtualKeyboard(nil)
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
		w.Stop()
	}()
	
	return nil
}

// setupInputCapture creates pointer and keyboard for input capture
func (w *WaylandVirtualInput) setupInputCapture() error {
	// For now, we'll provide a simplified setup
	// The actual input capture would require proper Wayland seat integration
	logger.Info("Input capture setup - using simplified implementation")
	
	// TODO: Implement actual Wayland seat-based input capture
	// This would involve:
	// 1. Getting pointer and keyboard from seat
	// 2. Setting up event handlers
	// 3. Managing focus and enter/leave events
	
	return nil
}

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
		w.virtualPtr.Destroy()
		w.virtualPtr = nil
	}
	
	if w.virtualKbd != nil {
		w.virtualKbd.Destroy()
		w.virtualKbd = nil
	}
	
	// Disable exclusive capture first
	w.disableExclusiveCapture()
	
	// Clean up managers
	if w.pointerMgr != nil {
		w.pointerMgr.Destroy()
		w.pointerMgr = nil
	}
	
	if w.keyboardMgr != nil {
		w.keyboardMgr.Destroy()
		w.keyboardMgr = nil
	}
	
	if w.constraintsMgr != nil {
		w.constraintsMgr.Destroy()
		w.constraintsMgr = nil
	}
	
	if w.shortcutsInhibitorMgr != nil {
		w.shortcutsInhibitorMgr.Destroy()
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
	}
	
	return nil
}

// OnInputEvent sets the callback for captured input events
func (w *WaylandVirtualInput) OnInputEvent(callback func(*protocol.InputEvent)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onInputEvent = callback
}

// InjectMouseMove injects a mouse move event (for server mode)
func (w *WaylandVirtualInput) InjectMouseMove(dx, dy float64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if !w.capturing || w.virtualPtr == nil {
		return fmt.Errorf("virtual pointer not available")
	}
	
	// Use relative motion for mouse movement
	if err := w.virtualPtr.Motion(time.Now(), dx, dy); err != nil {
		return fmt.Errorf("failed to inject mouse motion: %w", err)
	}
	
	// Frame the event
	return w.virtualPtr.Frame()
}

// InjectMouseButton injects a mouse button event (for server mode)
func (w *WaylandVirtualInput) InjectMouseButton(button uint32, pressed bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if !w.capturing || w.virtualPtr == nil {
		return fmt.Errorf("virtual pointer not available")
	}
	
	// Convert button state
	var state uint32
	if pressed {
		state = virtual_pointer.BUTTON_STATE_PRESSED
	} else {
		state = virtual_pointer.BUTTON_STATE_RELEASED
	}
	
	// Inject button event
	if err := w.virtualPtr.Button(time.Now(), button, state); err != nil {
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
	if err := w.virtualPtr.AxisSource(virtual_pointer.AXIS_SOURCE_WHEEL); err != nil {
		return fmt.Errorf("failed to set axis source: %w", err)
	}
	
	now := time.Now()
	
	// Inject vertical scroll if dy != 0
	if dy != 0 {
		if err := w.virtualPtr.Axis(now, virtual_pointer.AXIS_VERTICAL_SCROLL, -dy); err != nil {
			return fmt.Errorf("failed to inject vertical scroll: %w", err)
		}
	}
	
	// Inject horizontal scroll if dx != 0
	if dx != 0 {
		if err := w.virtualPtr.Axis(now, virtual_pointer.AXIS_HORIZONTAL_SCROLL, dx); err != nil {
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
	var state uint32
	if pressed {
		state = virtual_keyboard.KEY_STATE_PRESSED
	} else {
		state = virtual_keyboard.KEY_STATE_RELEASED
	}
	
	// Get current timestamp (simplified)
	timestamp := uint32(time.Now().UnixMilli() & 0xFFFFFFFF)
	
	// Inject key event
	return w.virtualKbd.Key(timestamp, key, state)
}

// Input event handlers for capture

// handlePointerMotion handles pointer motion events
func (w *WaylandVirtualInput) handlePointerMotion(event client.PointerMotionEvent) {
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
func (w *WaylandVirtualInput) handlePointerButton(event client.PointerButtonEvent) {
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
func (w *WaylandVirtualInput) handlePointerAxis(event client.PointerAxisEvent) {
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
func (w *WaylandVirtualInput) handlePointerEnter(event client.PointerEnterEvent) {
	// Could be used for edge detection later
	logger.Debugf("Pointer entered surface at (%.2f, %.2f)", event.SurfaceX, event.SurfaceY)
}

// handlePointerLeave handles pointer leave events
func (w *WaylandVirtualInput) handlePointerLeave(event client.PointerLeaveEvent) {
	// Could be used for edge detection later
	logger.Debug("Pointer left surface")
}

// handleKeyboardKey handles keyboard key events
func (w *WaylandVirtualInput) handleKeyboardKey(event client.KeyboardKeyEvent) {
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
				Modifiers: 0, // TODO: Track modifiers properly
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "wayland-capture",
	}
	
	callback(inputEvent)
}

// handleKeyboardKeymap handles keyboard keymap events
func (w *WaylandVirtualInput) handleKeyboardKeymap(event client.KeyboardKeymapEvent) {
	logger.Debug("Keyboard keymap updated")
}

// handleKeyboardEnter handles keyboard enter events
func (w *WaylandVirtualInput) handleKeyboardEnter(event client.KeyboardEnterEvent) {
	logger.Debug("Keyboard focus entered")
}

// handleKeyboardLeave handles keyboard leave events
func (w *WaylandVirtualInput) handleKeyboardLeave(event client.KeyboardLeaveEvent) {
	logger.Debug("Keyboard focus left")
}

// handleKeyboardModifiers handles keyboard modifier events
func (w *WaylandVirtualInput) handleKeyboardModifiers(event client.KeyboardModifiersEvent) {
	logger.Debugf("Keyboard modifiers: depressed=%d, latched=%d, locked=%d", 
		event.ModsDepressed, event.ModsLatched, event.ModsLocked)
}