// Package input handles mouse input injection via uinput
package input

import (
	"errors"
	"fmt"

	"github.com/bnema/waymon/internal/proto"
)

var (
	// ErrHandlerClosed is returned when operating on a closed handler
	ErrHandlerClosed = errors.New("handler is closed")
	// ErrInvalidEvent is returned for invalid events
	ErrInvalidEvent = errors.New("invalid event")
	// ErrNotImplemented is returned for unimplemented features
	ErrNotImplemented = errors.New("not implemented")
)

// Handler processes mouse events and injects them into the system
type Handler interface {
	ProcessEvent(event *proto.MouseEvent) error
	ProcessBatch(batch *proto.EventBatch) error
	Close() error
}

// NewHandler creates a new input handler
// This will attempt to use uinput directly, falling back to tool-based approaches
func NewHandler() (Handler, error) {
	// Try native uinput first
	handler, err := newUInputHandler()
	if err == nil {
		return handler, nil
	}

	// If permission denied or not available, try tool-based approach
	toolHandler, toolErr := newToolHandler()
	if toolErr == nil {
		return toolHandler, nil
	}

	// Return the original error if both fail
	return nil, fmt.Errorf("failed to create input handler: uinput: %v, tool: %v", err, toolErr)
}

// Coordinator manages input state and coordinates events
type Coordinator struct {
	handler Handler
	lastX   float64
	lastY   float64
}

// NewCoordinator creates a new input coordinator
func NewCoordinator(handler Handler) *Coordinator {
	return &Coordinator{
		handler: handler,
	}
}

// ProcessEvent processes a mouse event with state tracking
func (c *Coordinator) ProcessEvent(event *proto.MouseEvent) error {
	if event == nil {
		return ErrInvalidEvent
	}

	// Update position for all events that include coordinates
	if event.Type == proto.EventType_EVENT_TYPE_MOVE ||
		event.Type == proto.EventType_EVENT_TYPE_CLICK ||
		event.Type == proto.EventType_EVENT_TYPE_SCROLL {
		c.lastX = event.X
		c.lastY = event.Y
	}

	return c.handler.ProcessEvent(event)
}

// ProcessBatch processes multiple events
func (c *Coordinator) ProcessBatch(batch *proto.EventBatch) error {
	if batch == nil {
		return ErrInvalidEvent
	}

	for _, inputEvent := range batch.Events {
		// For now, only handle mouse events
		if inputEvent.GetMouse() != nil {
			if err := c.ProcessEvent(inputEvent.GetMouse()); err != nil {
				return fmt.Errorf("failed to process mouse event: %w", err)
			}
		}
		// TODO: Handle keyboard events when implemented
	}

	return nil
}

// Close closes the coordinator and underlying handler
func (c *Coordinator) Close() error {
	return c.handler.Close()
}

// GetPosition returns the last known mouse position
func (c *Coordinator) GetPosition() (x, y float64) {
	return c.lastX, c.lastY
}
