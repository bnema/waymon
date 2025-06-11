package input

import (
	"bytes"
	"fmt"
	"os/exec"
	"sync"

	"github.com/bnema/waymon/internal/proto"
)

// toolHandler implements Handler using external tools (dotool/ydotool)
type toolHandler struct {
	tool   string
	mu     sync.Mutex
	closed bool
}

// newToolHandler creates a new tool-based handler
func newToolHandler() (*toolHandler, error) {
	// Try to find available tool
	tools := []string{"dotool", "ydotool"}
	var availableTool string

	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err == nil {
			availableTool = tool
			break
		}
	}

	if availableTool == "" {
		return nil, fmt.Errorf("no input tool found (tried: %v)", tools)
	}

	return &toolHandler{
		tool: availableTool,
	}, nil
}

// ProcessEvent processes a single mouse event
func (h *toolHandler) ProcessEvent(event *proto.MouseEvent) error {
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
		return h.handleMove(event)
	case proto.EventType_EVENT_TYPE_LEAVE:
		return nil
	default:
		return fmt.Errorf("%w: unknown event type %v", ErrInvalidEvent, event.Type)
	}
}

// ProcessBatch processes multiple events
func (h *toolHandler) ProcessBatch(batch *proto.EventBatch) error {
	if batch == nil {
		return ErrInvalidEvent
	}

	// For tools, it's more efficient to batch commands
	var cmds []string
	for _, event := range batch.Events {
		cmd, err := h.eventToCommand(event)
		if err != nil {
			return err
		}
		if cmd != "" {
			cmds = append(cmds, cmd)
		}
	}

	if len(cmds) == 0 {
		return nil
	}

	// Execute batched commands
	return h.executeCommands(cmds)
}

// Close closes the handler
func (h *toolHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.closed = true
	return nil
}

func (h *toolHandler) handleMove(event *proto.MouseEvent) error {
	cmd := fmt.Sprintf("mousemove %d %d", int(event.X), int(event.Y))
	return h.executeCommand(cmd)
}

func (h *toolHandler) handleClick(event *proto.MouseEvent) error {
	// Move to position first
	if err := h.handleMove(event); err != nil {
		return err
	}

	// Map button
	buttonName := h.mapButton(event.Button)
	
	var action string
	if event.IsPressed {
		action = "mousedown"
	} else {
		action = "mouseup"
	}

	cmd := fmt.Sprintf("%s %s", action, buttonName)
	return h.executeCommand(cmd)
}

func (h *toolHandler) handleScroll(event *proto.MouseEvent) error {
	// Move to position first
	if err := h.handleMove(event); err != nil {
		return err
	}

	var direction string
	switch event.Direction {
	case proto.ScrollDirection_SCROLL_DIRECTION_UP:
		direction = "up"
	case proto.ScrollDirection_SCROLL_DIRECTION_DOWN:
		direction = "down"
	case proto.ScrollDirection_SCROLL_DIRECTION_LEFT:
		direction = "left"
	case proto.ScrollDirection_SCROLL_DIRECTION_RIGHT:
		direction = "right"
	default:
		return fmt.Errorf("%w: unknown scroll direction %v", ErrInvalidEvent, event.Direction)
	}

	cmd := fmt.Sprintf("wheel %s", direction)
	return h.executeCommand(cmd)
}

func (h *toolHandler) mapButton(button proto.MouseButton) string {
	switch button {
	case proto.MouseButton_MOUSE_BUTTON_LEFT:
		return "left"
	case proto.MouseButton_MOUSE_BUTTON_RIGHT:
		return "right"
	case proto.MouseButton_MOUSE_BUTTON_MIDDLE:
		return "middle"
	case proto.MouseButton_MOUSE_BUTTON_BACK:
		return "4" // X11 button 4
	case proto.MouseButton_MOUSE_BUTTON_FORWARD:
		return "5" // X11 button 5
	default:
		return "left"
	}
}

func (h *toolHandler) eventToCommand(event *proto.MouseEvent) (string, error) {
	switch event.Type {
	case proto.EventType_EVENT_TYPE_MOVE:
		return fmt.Sprintf("mousemove %d %d", int(event.X), int(event.Y)), nil
	case proto.EventType_EVENT_TYPE_CLICK:
		buttonName := h.mapButton(event.Button)
		var action string
		if event.IsPressed {
			action = "mousedown"
		} else {
			action = "mouseup"
		}
		return fmt.Sprintf("%s %s", action, buttonName), nil
	case proto.EventType_EVENT_TYPE_SCROLL:
		var direction string
		switch event.Direction {
		case proto.ScrollDirection_SCROLL_DIRECTION_UP:
			direction = "up"
		case proto.ScrollDirection_SCROLL_DIRECTION_DOWN:
			direction = "down"
		case proto.ScrollDirection_SCROLL_DIRECTION_LEFT:
			direction = "left"
		case proto.ScrollDirection_SCROLL_DIRECTION_RIGHT:
			direction = "right"
		default:
			return "", fmt.Errorf("%w: unknown scroll direction %v", ErrInvalidEvent, event.Direction)
		}
		return fmt.Sprintf("wheel %s", direction), nil
	case proto.EventType_EVENT_TYPE_ENTER:
		return fmt.Sprintf("mousemove %d %d", int(event.X), int(event.Y)), nil
	case proto.EventType_EVENT_TYPE_LEAVE:
		return "", nil
	default:
		return "", fmt.Errorf("%w: unknown event type %v", ErrInvalidEvent, event.Type)
	}
}

func (h *toolHandler) executeCommand(command string) error {
	return h.executeCommands([]string{command})
}

func (h *toolHandler) executeCommands(commands []string) error {
	if len(commands) == 0 {
		return nil
	}

	// Join commands with newlines for batching
	input := bytes.NewBufferString("")
	for _, cmd := range commands {
		fmt.Fprintln(input, cmd)
	}

	// Execute tool with commands
	// #nosec G204 - h.tool is validated in newToolHandler to be one of known tools
	cmd := exec.Command(h.tool)
	cmd.Stdin = input

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute %s: %w", h.tool, err)
	}

	return nil
}