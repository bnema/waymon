package proto

import (
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
)

func TestMouseEvent_Serialization(t *testing.T) {
	tests := []struct {
		name  string
		event *MouseEvent
	}{
		{
			name: "move event",
			event: &MouseEvent{
				Type:        EventType_EVENT_TYPE_MOVE,
				X:           100.5,
				Y:           200.5,
				TimestampMs: time.Now().UnixMilli(),
			},
		},
		{
			name: "click event",
			event: &MouseEvent{
				Type:        EventType_EVENT_TYPE_CLICK,
				X:           300,
				Y:           400,
				Button:      MouseButton_MOUSE_BUTTON_LEFT,
				IsPressed:   true,
				TimestampMs: time.Now().UnixMilli(),
			},
		},
		{
			name: "scroll event",
			event: &MouseEvent{
				Type:        EventType_EVENT_TYPE_SCROLL,
				X:           500,
				Y:           600,
				Direction:   ScrollDirection_SCROLL_DIRECTION_DOWN,
				TimestampMs: time.Now().UnixMilli(),
			},
		},
		{
			name: "enter event",
			event: &MouseEvent{
				Type:        EventType_EVENT_TYPE_ENTER,
				X:           0,
				Y:           300,
				TimestampMs: time.Now().UnixMilli(),
			},
		},
		{
			name: "leave event",
			event: &MouseEvent{
				Type:        EventType_EVENT_TYPE_LEAVE,
				X:           1920,
				Y:           500,
				TimestampMs: time.Now().UnixMilli(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			data, err := proto.Marshal(tt.event)
			if err != nil {
				t.Fatalf("Failed to marshal event: %v", err)
			}

			// Ensure data is compact
			if len(data) > 100 {
				t.Logf("Warning: serialized size is %d bytes", len(data))
			}

			// Deserialize
			decoded := &MouseEvent{}
			err = proto.Unmarshal(data, decoded)
			if err != nil {
				t.Fatalf("Failed to unmarshal event: %v", err)
			}

			// Compare
			if !proto.Equal(tt.event, decoded) {
				t.Errorf("Events are not equal after serialization")
				t.Errorf("Original: %+v", tt.event)
				t.Errorf("Decoded:  %+v", decoded)
			}
		})
	}
}

func TestEventBatch_Serialization(t *testing.T) {
	batch := &EventBatch{
		Events: []*InputEvent{
			{
				Event: &InputEvent_Mouse{
					Mouse: &MouseEvent{
						Type:        EventType_EVENT_TYPE_MOVE,
						X:           100,
						Y:           200,
						TimestampMs: time.Now().UnixMilli(),
					},
				},
			},
			{
				Event: &InputEvent_Mouse{
					Mouse: &MouseEvent{
						Type:        EventType_EVENT_TYPE_MOVE,
						X:           150,
						Y:           250,
						TimestampMs: time.Now().UnixMilli() + 10,
					},
				},
			},
			{
				Event: &InputEvent_Mouse{
					Mouse: &MouseEvent{
						Type:        EventType_EVENT_TYPE_CLICK,
						X:           150,
						Y:           250,
						Button:      MouseButton_MOUSE_BUTTON_LEFT,
						IsPressed:   true,
						TimestampMs: time.Now().UnixMilli() + 20,
					},
				},
			},
		},
	}

	// Serialize
	data, err := proto.Marshal(batch)
	if err != nil {
		t.Fatalf("Failed to marshal batch: %v", err)
	}

	// Deserialize
	decoded := &EventBatch{}
	err = proto.Unmarshal(data, decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal batch: %v", err)
	}

	// Compare
	if len(batch.Events) != len(decoded.Events) {
		t.Fatalf("Event count mismatch: %d vs %d", len(batch.Events), len(decoded.Events))
	}

	for i := range batch.Events {
		if !proto.Equal(batch.Events[i], decoded.Events[i]) {
			t.Errorf("Event %d is not equal after serialization", i)
		}
	}
}

func BenchmarkMouseEvent_Marshal(b *testing.B) {
	event := &MouseEvent{
		Type:        EventType_EVENT_TYPE_MOVE,
		X:           123.456,
		Y:           789.012,
		TimestampMs: time.Now().UnixMilli(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := proto.Marshal(event)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMouseEvent_Unmarshal(b *testing.B) {
	event := &MouseEvent{
		Type:        EventType_EVENT_TYPE_MOVE,
		X:           123.456,
		Y:           789.012,
		TimestampMs: time.Now().UnixMilli(),
	}

	data, err := proto.Marshal(event)
	if err != nil {
		b.Fatal(err)
	}

	decoded := &MouseEvent{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := proto.Unmarshal(data, decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}
