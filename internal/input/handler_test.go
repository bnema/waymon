package input

import (
	"testing"
	"time"

	"github.com/bnema/waymon/internal/proto"
)

func TestNewHandler(t *testing.T) {
	handler, err := NewHandler()
	if err != nil {
		// This might fail if running without proper permissions
		t.Skipf("Cannot create handler (likely permission issue): %v", err)
	}
	defer func() { _ = handler.Close() }()

	if handler == nil {
		t.Error("Expected non-nil handler")
	}
}

func TestHandler_ProcessEvent(t *testing.T) {
	handler := &MockHandler{}
	defer func() { _ = handler.Close() }()

	tests := []struct {
		name    string
		event   *proto.MouseEvent
		wantErr bool
	}{
		{
			name: "move event",
			event: &proto.MouseEvent{
				Type:        proto.EventType_EVENT_TYPE_MOVE,
				X:           100,
				Y:           200,
				TimestampMs: time.Now().UnixMilli(),
			},
			wantErr: false,
		},
		{
			name: "click press event",
			event: &proto.MouseEvent{
				Type:        proto.EventType_EVENT_TYPE_CLICK,
				X:           300,
				Y:           400,
				Button:      proto.MouseButton_MOUSE_BUTTON_LEFT,
				IsPressed:   true,
				TimestampMs: time.Now().UnixMilli(),
			},
			wantErr: false,
		},
		{
			name: "click release event",
			event: &proto.MouseEvent{
				Type:        proto.EventType_EVENT_TYPE_CLICK,
				X:           300,
				Y:           400,
				Button:      proto.MouseButton_MOUSE_BUTTON_LEFT,
				IsPressed:   false,
				TimestampMs: time.Now().UnixMilli(),
			},
			wantErr: false,
		},
		{
			name: "scroll event",
			event: &proto.MouseEvent{
				Type:        proto.EventType_EVENT_TYPE_SCROLL,
				X:           500,
				Y:           600,
				Direction:   proto.ScrollDirection_SCROLL_DIRECTION_DOWN,
				TimestampMs: time.Now().UnixMilli(),
			},
			wantErr: false,
		},
		{
			name:    "nil event",
			event:   nil,
			wantErr: true,
		},
		{
			name: "invalid event type",
			event: &proto.MouseEvent{
				Type:        proto.EventType_EVENT_TYPE_UNSPECIFIED,
				TimestampMs: time.Now().UnixMilli(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.ProcessEvent(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("Handler.ProcessEvent() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && tt.event != nil {
				// Verify the event was recorded
				lastEvent := handler.LastEvent()
				if lastEvent == nil {
					t.Error("Expected event to be recorded")
				} else if lastEvent.Type != tt.event.Type {
					t.Errorf("Event type mismatch: got %v, want %v", lastEvent.Type, tt.event.Type)
				}
			}
		})
	}
}

func TestHandler_ProcessBatch(t *testing.T) {
	handler := &MockHandler{}
	defer func() { _ = handler.Close() }()

	batch := &proto.EventBatch{
		Events: []*proto.InputEvent{
			{
				Event: &proto.InputEvent_Mouse{
					Mouse: &proto.MouseEvent{
						Type:        proto.EventType_EVENT_TYPE_MOVE,
						X:           100,
						Y:           100,
						TimestampMs: time.Now().UnixMilli(),
					},
				},
			},
			{
				Event: &proto.InputEvent_Mouse{
					Mouse: &proto.MouseEvent{
						Type:        proto.EventType_EVENT_TYPE_MOVE,
						X:           150,
						Y:           150,
						TimestampMs: time.Now().UnixMilli() + 10,
					},
				},
			},
			{
				Event: &proto.InputEvent_Mouse{
					Mouse: &proto.MouseEvent{
						Type:        proto.EventType_EVENT_TYPE_CLICK,
						X:           150,
						Y:           150,
						Button:      proto.MouseButton_MOUSE_BUTTON_LEFT,
						IsPressed:   true,
						TimestampMs: time.Now().UnixMilli() + 20,
					},
				},
			},
		},
	}

	err := handler.ProcessBatch(batch)
	if err != nil {
		t.Errorf("ProcessBatch failed: %v", err)
	}

	// Verify all events were processed
	if handler.EventCount() != len(batch.Events) {
		t.Errorf("Expected %d events, got %d", len(batch.Events), handler.EventCount())
	}
}

// MockHandler for testing without actual uinput
type MockHandler struct {
	events []*proto.MouseEvent
	closed bool
}

func (m *MockHandler) ProcessEvent(event *proto.MouseEvent) error {
	if m.closed {
		return ErrHandlerClosed
	}
	if event == nil {
		return ErrInvalidEvent
	}
	if event.Type == proto.EventType_EVENT_TYPE_UNSPECIFIED {
		return ErrInvalidEvent
	}
	m.events = append(m.events, event)
	return nil
}

func (m *MockHandler) ProcessBatch(batch *proto.EventBatch) error {
	if batch == nil {
		return ErrInvalidEvent
	}
	for _, event := range batch.Events {
		if event.GetMouse() != nil {
			if err := m.ProcessEvent(event.GetMouse()); err != nil {
				return err
			}
		}
		// Skip keyboard events for now in the mock handler
	}
	return nil
}

func (m *MockHandler) Close() error {
	m.closed = true
	return nil
}

func (m *MockHandler) LastEvent() *proto.MouseEvent {
	if len(m.events) == 0 {
		return nil
	}
	return m.events[len(m.events)-1]
}

func (m *MockHandler) EventCount() int {
	return len(m.events)
}
