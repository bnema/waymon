// Package input provides input injection using Wayland virtual input protocols.
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
	"github.com/bnema/waymon/internal/domain"
	"github.com/rajveermalviya/go-wayland/wayland/client"
	"github.com/rs/zerolog"
)

// Compile-time interface check
var _ interface {
	Start(ctx context.Context) error
	Stop() error
	InjectMouseMove(ctx context.Context, dx, dy float64) error
	InjectMousePosition(ctx context.Context, x, y int32) error
	InjectMouseButton(ctx context.Context, button uint32, pressed bool) error
	InjectMouseScroll(ctx context.Context, dx, dy float64, scrollType domain.ScrollType) error
	InjectKeyEvent(ctx context.Context, key uint32, pressed bool, modifiers uint32) error
	InjectCharacter(ctx context.Context, char rune, pressed bool) error
	SetKeyboardLayout(layout domain.KeyboardLayout) error
	GetKeyboardLayout() domain.KeyboardLayout
	SetExclusiveCapture(ctx context.Context, enabled bool) error
} = (*WaylandInjector)(nil)

// WaylandInjector implements InputInjectionPort using Wayland virtual input protocols.
// This is used on the client side to inject mouse/keyboard events received from the server.
type WaylandInjector struct {
	mu sync.RWMutex

	// Wayland connection
	display *client.Display
	seat    *client.Seat
	surface *client.Surface
	pointer *client.Pointer

	// Virtual input managers
	pointerMgr  *virtual_pointer.VirtualPointerManager
	keyboardMgr *virtual_keyboard.VirtualKeyboardManager

	// Virtual devices
	virtualPtr *virtual_pointer.VirtualPointer
	virtualKbd *virtual_keyboard.VirtualKeyboard

	// Pointer constraints for exclusive capture
	constraintsMgr pointer_constraints.PointerConstraintsManager
	lockedPointer  pointer_constraints.LockedPointer

	// Keyboard shortcuts inhibitor for exclusive keyboard capture
	shortcutsInhibitorMgr keyboard_shortcuts_inhibitor.KeyboardShortcutsInhibitorManager
	shortcutsInhibitor    keyboard_shortcuts_inhibitor.KeyboardShortcutsInhibitor

	// Keyboard layout support
	keyboardLayout     domain.KeyboardLayout
	keyboardLayoutPort keyboardLayoutPort

	// State
	ctx       context.Context
	cancel    context.CancelFunc
	running   bool
	exclusive bool

	// Screen dimensions for absolute positioning (TODO: get from display)
	screenWidth  uint32
	screenHeight uint32
}

// keyboardLayoutPort is a local interface to avoid circular imports.
// It matches the KeyboardLayoutPort interface in boundaries/out.
type keyboardLayoutPort interface {
	CharToKeySequence(ctx context.Context, char rune, layout domain.KeyboardLayout) ([]domain.KeySequence, error)
}

// New creates a new WaylandInjector instance.
func New(ctx context.Context) (*WaylandInjector, error) {
	log := zerolog.Ctx(ctx)

	w := &WaylandInjector{
		screenWidth:  1920, // Default, should be updated from display info
		screenHeight: 1080,
	}

	// Connect to Wayland display
	display, err := client.Connect("")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Wayland display: %w", err)
	}
	w.display = display

	// Setup Wayland globals (simplified - full setup not required for virtual input)
	log.Info().Msg("wayland connection established")

	// Create virtual pointer manager
	pointerMgr, err := virtual_pointer.NewVirtualPointerManager(ctx)
	if err != nil {
		if err := display.Destroy(); err != nil {
			log.Error().Err(err).Msg("failed to destroy display")
		}
		return nil, fmt.Errorf("failed to create virtual pointer manager: %w", err)
	}
	w.pointerMgr = pointerMgr

	// Create virtual keyboard manager (optional - some systems may not support it)
	keyboardMgr, err := virtual_keyboard.NewVirtualKeyboardManager(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("failed to create virtual keyboard manager, keyboard support limited")
	} else {
		w.keyboardMgr = keyboardMgr
	}

	// Create pointer constraints manager (optional - for exclusive capture)
	constraintsMgr, err := pointer_constraints.NewPointerConstraintsManager(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("failed to create pointer constraints manager, exclusive capture limited")
	} else {
		w.constraintsMgr = constraintsMgr
	}

	// Create keyboard shortcuts inhibitor manager (optional - for exclusive capture)
	shortcutsInhibitorMgr, err := keyboard_shortcuts_inhibitor.NewKeyboardShortcutsInhibitorManager(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("failed to create shortcuts inhibitor manager, exclusive capture limited")
	} else {
		w.shortcutsInhibitorMgr = shortcutsInhibitorMgr
	}

	log.Info().Msg("wayland injector created successfully")
	return w, nil
}

// Start initializes the input injection system.
func (w *WaylandInjector) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	log := zerolog.Ctx(ctx)

	if w.running {
		return fmt.Errorf("already running")
	}

	w.ctx, w.cancel = context.WithCancel(ctx)

	// Create virtual pointer
	if w.pointerMgr != nil {
		virtualPtr, err := w.pointerMgr.CreatePointer()
		if err != nil {
			log.Warn().Err(err).Msg("failed to create virtual pointer")
		} else {
			w.virtualPtr = virtualPtr
			log.Info().Msg("virtual pointer created")
		}
	}

	// Create virtual keyboard
	if w.keyboardMgr != nil {
		virtualKbd, err := w.keyboardMgr.CreateKeyboard()
		if err != nil {
			log.Warn().Err(err).Msg("failed to create virtual keyboard")
		} else {
			w.virtualKbd = virtualKbd
			log.Info().Msg("virtual keyboard created")
		}
	}

	w.running = true
	log.Info().Msg("wayland input injection started")

	// Monitor context for shutdown
	go func() {
		<-w.ctx.Done()
		if err := w.Stop(); err != nil {
			log.Error().Err(err).Msg("failed to stop wayland injector")
		}
	}()

	return nil
}

// Stop stops the input injection system.
func (w *WaylandInjector) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	log := zerolog.Ctx(w.ctx)

	if !w.running {
		return nil
	}

	w.running = false

	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}

	// Disable exclusive capture first
	if err := w.disableExclusiveCaptureInternal(); err != nil {
		log.Error().Err(err).Msg("failed to disable exclusive capture")
	}

	// Clean up virtual devices
	if w.virtualPtr != nil {
		if err := w.virtualPtr.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close virtual pointer")
		}
		w.virtualPtr = nil
	}

	if w.virtualKbd != nil {
		if err := w.virtualKbd.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close virtual keyboard")
		}
		w.virtualKbd = nil
	}

	// Clean up managers
	if w.pointerMgr != nil {
		if err := w.pointerMgr.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close pointer manager")
		}
		w.pointerMgr = nil
	}

	if w.keyboardMgr != nil {
		if err := w.keyboardMgr.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close keyboard manager")
		}
		w.keyboardMgr = nil
	}

	if w.constraintsMgr != nil {
		if err := w.constraintsMgr.Destroy(); err != nil {
			log.Error().Err(err).Msg("failed to destroy constraints manager")
		}
		w.constraintsMgr = nil
	}

	if w.shortcutsInhibitorMgr != nil {
		if err := w.shortcutsInhibitorMgr.Destroy(); err != nil {
			log.Error().Err(err).Msg("failed to destroy shortcuts inhibitor manager")
		}
		w.shortcutsInhibitorMgr = nil
	}

	log.Info().Msg("wayland input injection stopped")
	return nil
}

// InjectMouseMove injects a relative mouse movement.
func (w *WaylandInjector) InjectMouseMove(ctx context.Context, dx, dy float64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	log := zerolog.Ctx(ctx)

	if !w.running || w.virtualPtr == nil {
		return fmt.Errorf("virtual pointer not available (running=%v, ptr=%v)", w.running, w.virtualPtr != nil)
	}

	// Inject relative motion
	if err := w.virtualPtr.Motion(time.Now(), dx, dy); err != nil {
		return fmt.Errorf("failed to inject mouse motion: %w", err)
	}

	// Frame the event
	if err := w.virtualPtr.Frame(); err != nil {
		return fmt.Errorf("failed to frame mouse motion: %w", err)
	}

	log.Debug().Float64("dx", dx).Float64("dy", dy).Msg("injected mouse move")
	return nil
}

// InjectMousePosition injects an absolute mouse position.
func (w *WaylandInjector) InjectMousePosition(ctx context.Context, x, y int32) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	log := zerolog.Ctx(ctx)

	if !w.running || w.virtualPtr == nil {
		return fmt.Errorf("virtual pointer not available (running=%v, ptr=%v)", w.running, w.virtualPtr != nil)
	}

	// Inject absolute motion
	// Clamp negative coordinates to 0 to safely convert int32 to uint32
	ux := uint32(0)
	uy := uint32(0)
	if x > 0 {
		ux = uint32(x)
	}
	if y > 0 {
		uy = uint32(y)
	}
	if err := w.virtualPtr.MotionAbsolute(time.Now(), ux, uy, w.screenWidth, w.screenHeight); err != nil {
		return fmt.Errorf("failed to inject absolute mouse position: %w", err)
	}

	// Frame the event
	if err := w.virtualPtr.Frame(); err != nil {
		return fmt.Errorf("failed to frame absolute mouse position: %w", err)
	}

	log.Debug().Int32("x", x).Int32("y", y).Msg("injected mouse position")
	return nil
}

// InjectMouseButton injects a mouse button press or release.
func (w *WaylandInjector) InjectMouseButton(ctx context.Context, button uint32, pressed bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	log := zerolog.Ctx(ctx)

	if !w.running || w.virtualPtr == nil {
		return fmt.Errorf("virtual pointer not available (running=%v, ptr=%v)", w.running, w.virtualPtr != nil)
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
	if err := w.virtualPtr.Frame(); err != nil {
		return fmt.Errorf("failed to frame mouse button: %w", err)
	}

	log.Debug().Uint32("button", button).Bool("pressed", pressed).Msg("injected mouse button")
	return nil
}

// InjectMouseScroll injects a scroll event.
func (w *WaylandInjector) InjectMouseScroll(ctx context.Context, dx, dy float64, scrollType domain.ScrollType) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	log := zerolog.Ctx(ctx)

	if !w.running || w.virtualPtr == nil {
		return fmt.Errorf("virtual pointer not available")
	}

	// Set axis source based on scroll type
	var axisSource virtual_pointer.AxisSource
	switch scrollType {
	case domain.ScrollWheel:
		axisSource = virtual_pointer.AxisSourceWheel
	case domain.ScrollFinger:
		axisSource = virtual_pointer.AxisSourceFinger
	case domain.ScrollContinuous:
		axisSource = virtual_pointer.AxisSourceContinuous
	default:
		axisSource = virtual_pointer.AxisSourceWheel
	}

	if err := w.virtualPtr.AxisSource(axisSource); err != nil {
		return fmt.Errorf("failed to set axis source: %w", err)
	}

	now := time.Now()

	// Inject vertical scroll if dy != 0
	if dy != 0 {
		// Note: Wayland scroll direction may need inversion depending on compositor
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
	if err := w.virtualPtr.Frame(); err != nil {
		return fmt.Errorf("failed to frame scroll: %w", err)
	}

	log.Debug().Float64("dx", dx).Float64("dy", dy).Str("type", scrollType.String()).Msg("injected scroll")
	return nil
}

// InjectKeyEvent injects a keyboard key event.
func (w *WaylandInjector) InjectKeyEvent(ctx context.Context, key uint32, pressed bool, _ uint32) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	log := zerolog.Ctx(ctx)

	if !w.running || w.virtualKbd == nil {
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
	if err := w.virtualKbd.Key(time.Now(), key, state); err != nil {
		return fmt.Errorf("failed to inject key event: %w", err)
	}

	keyName := getKeyName(key)
	log.Debug().Uint32("key", key).Str("name", keyName).Bool("pressed", pressed).Msg("injected key event")
	return nil
}

// SetExclusiveCapture enables or disables exclusive input capture.
func (w *WaylandInjector) SetExclusiveCapture(ctx context.Context, enabled bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	log := zerolog.Ctx(ctx)

	if enabled == w.exclusive {
		return nil // Already in desired state
	}

	if enabled {
		if err := w.enableExclusiveCaptureInternal(); err != nil {
			return err
		}
		w.exclusive = true
		log.Info().Msg("exclusive capture enabled")
	} else {
		if err := w.disableExclusiveCaptureInternal(); err != nil {
			return err
		}
		w.exclusive = false
		log.Info().Msg("exclusive capture disabled")
	}

	return nil
}

// enableExclusiveCaptureInternal enables exclusive pointer and keyboard capture.
// Must be called with lock held.
func (w *WaylandInjector) enableExclusiveCaptureInternal() error {
	log := zerolog.Ctx(w.ctx)

	// Lock pointer to current position for exclusive capture
	if w.constraintsMgr != nil && w.surface != nil && w.pointer != nil {
		lockedPointer, err := pointer_constraints.LockPointerAtCurrentPosition(w.constraintsMgr, w.surface, w.pointer)
		if err != nil {
			log.Warn().Err(err).Msg("failed to lock pointer")
		} else {
			w.lockedPointer = lockedPointer
			log.Info().Msg("pointer locked for exclusive capture")
		}
	}

	// Inhibit keyboard shortcuts for exclusive keyboard capture
	if w.shortcutsInhibitorMgr != nil && w.surface != nil && w.seat != nil {
		inhibitor, err := w.shortcutsInhibitorMgr.InhibitShortcuts(w.surface, w.seat)
		if err != nil {
			log.Warn().Err(err).Msg("failed to inhibit keyboard shortcuts")
		} else {
			w.shortcutsInhibitor = inhibitor
			log.Info().Msg("keyboard shortcuts inhibited for exclusive capture")
		}
	}

	return nil
}

// disableExclusiveCaptureInternal disables exclusive pointer and keyboard capture.
// Must be called with lock held.
func (w *WaylandInjector) disableExclusiveCaptureInternal() error {
	log := zerolog.Ctx(w.ctx)

	// Unlock pointer
	if w.lockedPointer != nil {
		if err := w.lockedPointer.Destroy(); err != nil {
			log.Warn().Err(err).Msg("failed to destroy locked pointer")
		} else {
			log.Info().Msg("pointer unlocked")
		}
		w.lockedPointer = nil
	}

	// Re-enable keyboard shortcuts
	if w.shortcutsInhibitor != nil {
		if err := w.shortcutsInhibitor.Destroy(); err != nil {
			log.Warn().Err(err).Msg("failed to destroy shortcuts inhibitor")
		} else {
			log.Info().Msg("keyboard shortcuts re-enabled")
		}
		w.shortcutsInhibitor = nil
	}

	return nil
}

// SetScreenDimensions sets the screen dimensions for absolute positioning.
func (w *WaylandInjector) SetScreenDimensions(width, height uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.screenWidth = width
	w.screenHeight = height
}

// SetKeyboardLayoutPort sets the keyboard layout port for character injection.
// This must be called before using InjectCharacter.
func (w *WaylandInjector) SetKeyboardLayoutPort(port keyboardLayoutPort) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.keyboardLayoutPort = port
}

// SetKeyboardLayout sets the target keyboard layout for character injection.
func (w *WaylandInjector) SetKeyboardLayout(layout domain.KeyboardLayout) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.keyboardLayout = layout
	return nil
}

// GetKeyboardLayout returns the currently configured keyboard layout.
func (w *WaylandInjector) GetKeyboardLayout() domain.KeyboardLayout {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.keyboardLayout == "" {
		return domain.LayoutUS // Default
	}
	return w.keyboardLayout
}

// InjectCharacter injects a Unicode character using the appropriate keycode
// sequence for the configured keyboard layout.
func (w *WaylandInjector) InjectCharacter(ctx context.Context, char rune, pressed bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	log := zerolog.Ctx(ctx)

	if !w.running || w.virtualKbd == nil {
		return fmt.Errorf("virtual keyboard not available")
	}

	if w.keyboardLayoutPort == nil {
		return fmt.Errorf("keyboard layout port not configured")
	}

	layout := w.keyboardLayout
	if layout == "" {
		layout = domain.LayoutUS
	}

	// Get the key sequence for this character
	sequences, err := w.keyboardLayoutPort.CharToKeySequence(ctx, char, layout)
	if err != nil {
		return fmt.Errorf("failed to translate character %q: %w", char, err)
	}

	// Inject each keystroke in the sequence
	for _, seq := range sequences {
		if err := w.injectKeySequence(ctx, seq, pressed); err != nil {
			return err
		}
	}

	log.Debug().
		Int32("char", char).
		Str("layout", string(layout)).
		Bool("pressed", pressed).
		Int("sequences", len(sequences)).
		Msg("injected character")

	return nil
}

// injectKeySequence injects a single key sequence with modifiers.
// Must be called with lock held.
func (w *WaylandInjector) injectKeySequence(ctx context.Context, seq domain.KeySequence, pressed bool) error {
	log := zerolog.Ctx(ctx)
	now := time.Now()

	// Determine which modifier keys need to be pressed
	needShift := seq.Modifier.HasShift()
	needAltGr := seq.Modifier.HasAltGr()

	// Press modifier keys if needed
	if pressed {
		if needShift {
			if err := w.virtualKbd.Key(now, KeyLeftShift, virtual_keyboard.KeyStatePressed); err != nil {
				return fmt.Errorf("failed to press shift: %w", err)
			}
		}
		if needAltGr {
			if err := w.virtualKbd.Key(now, KeyRightAlt, virtual_keyboard.KeyStatePressed); err != nil {
				return fmt.Errorf("failed to press altgr: %w", err)
			}
		}
	}

	// Press/release the main key
	var state virtual_keyboard.KeyState
	if pressed {
		state = virtual_keyboard.KeyStatePressed
	} else {
		state = virtual_keyboard.KeyStateReleased
	}

	if err := w.virtualKbd.Key(now, uint32(seq.Keycode), state); err != nil {
		return fmt.Errorf("failed to inject key: %w", err)
	}

	// Release modifier keys if we pressed them (only on key release or after press)
	if pressed {
		// For a character press, we need to also release the key and modifiers
		// Actually, for proper typing we should: press modifiers, press key, release key, release modifiers
		if err := w.virtualKbd.Key(now, uint32(seq.Keycode), virtual_keyboard.KeyStateReleased); err != nil {
			log.Warn().Err(err).Msg("failed to release key after press")
		}

		if needAltGr {
			if err := w.virtualKbd.Key(now, KeyRightAlt, virtual_keyboard.KeyStateReleased); err != nil {
				log.Warn().Err(err).Msg("failed to release altgr")
			}
		}
		if needShift {
			if err := w.virtualKbd.Key(now, KeyLeftShift, virtual_keyboard.KeyStateReleased); err != nil {
				log.Warn().Err(err).Msg("failed to release shift")
			}
		}
	}

	return nil
}

// Key codes for modifier keys (from evdev)
const (
	KeyLeftShift uint32 = 42
	KeyRightAlt  uint32 = 100 // AltGr
)

// getKeyName returns a human-readable name for common key codes.
func getKeyName(key uint32) string {
	switch key {
	// Letters
	case 30:
		return "A"
	case 48:
		return "B"
	case 46:
		return "C"
	case 32:
		return "D"
	case 18:
		return "E"
	case 33:
		return "F"
	case 34:
		return "G"
	case 35:
		return "H"
	case 23:
		return "I"
	case 36:
		return "J"
	case 37:
		return "K"
	case 38:
		return "L"
	case 50:
		return "M"
	case 49:
		return "N"
	case 24:
		return "O"
	case 25:
		return "P"
	case 16:
		return "Q"
	case 19:
		return "R"
	case 31:
		return "S"
	case 20:
		return "T"
	case 22:
		return "U"
	case 47:
		return "V"
	case 17:
		return "W"
	case 45:
		return "X"
	case 21:
		return "Y"
	case 44:
		return "Z"
	// Numbers
	case 2:
		return "1"
	case 3:
		return "2"
	case 4:
		return "3"
	case 5:
		return "4"
	case 6:
		return "5"
	case 7:
		return "6"
	case 8:
		return "7"
	case 9:
		return "8"
	case 10:
		return "9"
	case 11:
		return "0"
	// Modifiers
	case 29:
		return "LEFTCTRL"
	case 97:
		return "RIGHTCTRL"
	case 42:
		return "LEFTSHIFT"
	case 54:
		return "RIGHTSHIFT"
	case 56:
		return "LEFTALT"
	case 100:
		return "RIGHTALT"
	case 125:
		return "LEFTMETA"
	case 126:
		return "RIGHTMETA"
	// Special keys
	case 1:
		return "ESC"
	case 14:
		return "BACKSPACE"
	case 15:
		return "TAB"
	case 28:
		return "ENTER"
	case 57:
		return "SPACE"
	case 58:
		return "CAPSLOCK"
	// Function keys
	case 59:
		return "F1"
	case 60:
		return "F2"
	case 61:
		return "F3"
	case 62:
		return "F4"
	case 63:
		return "F5"
	case 64:
		return "F6"
	case 65:
		return "F7"
	case 66:
		return "F8"
	case 67:
		return "F9"
	case 68:
		return "F10"
	case 87:
		return "F11"
	case 88:
		return "F12"
	default:
		return fmt.Sprintf("KEY_%d", key)
	}
}
