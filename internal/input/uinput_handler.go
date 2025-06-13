package input

import (
	"fmt"
	"sync"

	"github.com/ThomasT75/uinput"
	"github.com/bnema/waymon/internal/proto"
)

// uInputHandler implements Handler using direct uinput bindings
type uInputHandler struct {
	mouse    uinput.Mouse
	keyboard uinput.Keyboard // TODO: For hotkey switching
	mu       sync.Mutex
	closed   bool
	// Track current position for relative movements
	currentX float64
	currentY float64
}

// newUInputHandler creates a new uinput-based handler
func newUInputHandler() (*uInputHandler, error) {
	// Create virtual mouse for all mouse operations
	mouse, err := uinput.CreateMouse("/dev/uinput", []byte("Waymon Virtual Mouse"))
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual mouse: %w", err)
	}

	// TODO: Create virtual keyboard for hotkey support
	// keyboard, err := uinput.CreateKeyboard("/dev/uinput", []byte("Waymon Virtual Keyboard"))

	return &uInputHandler{
		mouse:    mouse,
		currentX: 0,
		currentY: 0,
	}, nil
}

// ProcessEvent processes a single mouse event
func (h *uInputHandler) ProcessEvent(event *proto.MouseEvent) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return ErrHandlerClosed
	}

	if event == nil {
		return ErrInvalidEvent
	}

	switch event.Type {
	case proto.EventType_EVENT_TYPE_MOVE:
		return h.handleMove(event)
	case proto.EventType_EVENT_TYPE_CLICK:
		return h.handleClick(event)
	case proto.EventType_EVENT_TYPE_SCROLL:
		return h.handleScroll(event)
	case proto.EventType_EVENT_TYPE_ENTER:
		// Handle screen enter - set position and move there
		h.currentX = event.X
		h.currentY = event.Y
		// TODO: May need to warp cursor to entry point
		return nil
	case proto.EventType_EVENT_TYPE_LEAVE:
		// Handle screen leave - update position
		h.currentX = event.X
		h.currentY = event.Y
		return nil
	default:
		return fmt.Errorf("%w: unknown event type %v", ErrInvalidEvent, event.Type)
	}
}

// ProcessBatch processes multiple events
func (h *uInputHandler) ProcessBatch(batch *proto.EventBatch) error {
	if batch == nil {
		return ErrInvalidEvent
	}

	for _, inputEvent := range batch.Events {
		// For now, only handle mouse events
		if inputEvent.GetMouse() != nil {
			if err := h.ProcessEvent(inputEvent.GetMouse()); err != nil {
				return err
			}
		}
		// TODO: Handle keyboard events when implemented
	}

	return nil
}

// Close closes the handler
func (h *uInputHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil
	}

	h.closed = true

	var err error
	if h.mouse != nil {
		err = h.mouse.Close()
	}
	if h.keyboard != nil {
		if e := h.keyboard.Close(); e != nil && err == nil {
			err = e
		}
	}

	return err
}

func (h *uInputHandler) handleMove(event *proto.MouseEvent) error {
	// Calculate relative movement
	deltaX := int32(event.X - h.currentX)
	deltaY := int32(event.Y - h.currentY)

	// Update tracked position
	h.currentX = event.X
	h.currentY = event.Y

	// Apply relative movement
	if deltaX != 0 || deltaY != 0 {
		return h.mouse.Move(deltaX, deltaY)
	}

	return nil
}

func (h *uInputHandler) handleClick(event *proto.MouseEvent) error {
	// First move to position if needed
	if event.X != h.currentX || event.Y != h.currentY {
		if err := h.handleMove(event); err != nil {
			return err
		}
	}

	// Handle click based on button
	switch event.Button {
	case proto.MouseButton_MOUSE_BUTTON_LEFT:
		if event.IsPressed {
			return h.mouse.LeftPress()
		}
		return h.mouse.LeftRelease()
	case proto.MouseButton_MOUSE_BUTTON_RIGHT:
		if event.IsPressed {
			return h.mouse.RightPress()
		}
		return h.mouse.RightRelease()
	case proto.MouseButton_MOUSE_BUTTON_MIDDLE:
		if event.IsPressed {
			return h.mouse.MiddlePress()
		}
		return h.mouse.MiddleRelease()
	case proto.MouseButton_MOUSE_BUTTON_BACK:
		// TODO: Implement back/forward buttons when uinput library supports them
		return ErrNotImplemented
	case proto.MouseButton_MOUSE_BUTTON_FORWARD:
		// TODO: Implement back/forward buttons when uinput library supports them
		return ErrNotImplemented
	default:
		return fmt.Errorf("%w: unknown button %v", ErrInvalidEvent, event.Button)
	}
}

func (h *uInputHandler) handleScroll(event *proto.MouseEvent) error {
	// First move to position if needed
	if event.X != h.currentX || event.Y != h.currentY {
		if err := h.handleMove(event); err != nil {
			return err
		}
	}

	// Handle scroll based on direction
	switch event.Direction {
	case proto.ScrollDirection_SCROLL_DIRECTION_UP:
		return h.mouse.Wheel(false, 1)
	case proto.ScrollDirection_SCROLL_DIRECTION_DOWN:
		return h.mouse.Wheel(false, -1)
	case proto.ScrollDirection_SCROLL_DIRECTION_LEFT:
		return h.mouse.Wheel(true, -1)
	case proto.ScrollDirection_SCROLL_DIRECTION_RIGHT:
		return h.mouse.Wheel(true, 1)
	default:
		return fmt.Errorf("%w: unknown scroll direction %v", ErrInvalidEvent, event.Direction)
	}
}

// TODO: Future keyboard support for hotkeys
// Example hotkey: Ctrl+Alt+S to switch screens
// func (h *uInputHandler) RegisterHotkey(keys []Key, callback func()) error {
//     // Register hotkey combination
// }
//
// func (h *uInputHandler) ProcessKeyEvent(event *proto.KeyEvent) error {
//     // Handle keyboard events
// }
