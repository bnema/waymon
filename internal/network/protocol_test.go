package network

import (
	"bytes"
	"testing"
	"time"

	"github.com/bnema/waymon/internal/proto"
	pb "google.golang.org/protobuf/proto"
)

func TestWriteMessage(t *testing.T) {
	tests := []struct {
		name    string
		event   *proto.MouseEvent
		wantErr bool
	}{
		{
			name: "valid event",
			event: &proto.MouseEvent{
				Type:        proto.EventType_EVENT_TYPE_MOVE,
				X:           100,
				Y:           200,
				TimestampMs: time.Now().UnixMilli(),
			},
			wantErr: false,
		},
		{
			name:    "nil event",
			event:   nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := WriteMessage(buf, tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteMessage() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && buf.Len() == 0 {
				t.Error("WriteMessage() wrote no data")
			}
		})
	}
}

func TestReadMessage(t *testing.T) {
	// Create a test event
	original := &proto.MouseEvent{
		Type:        proto.EventType_EVENT_TYPE_CLICK,
		X:           300,
		Y:           400,
		Button:      proto.MouseButton_MOUSE_BUTTON_RIGHT,
		IsPressed:   true,
		TimestampMs: time.Now().UnixMilli(),
	}

	// Write to buffer
	buf := &bytes.Buffer{}
	err := WriteMessage(buf, original)
	if err != nil {
		t.Fatalf("Failed to write message: %v", err)
	}

	// Read back
	decoded, err := ReadMessage(buf)
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	// Compare
	if !pb.Equal(original, decoded) {
		t.Error("Messages are not equal after round trip")
		t.Errorf("Original: %+v", original)
		t.Errorf("Decoded:  %+v", decoded)
	}
}

func TestReadMessage_InvalidData(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "empty buffer",
			data:    []byte{},
			wantErr: true,
		},
		{
			name:    "invalid length header",
			data:    []byte{0xFF, 0xFF, 0xFF, 0xFF}, // Max uint32
			wantErr: true,
		},
		{
			name:    "truncated data",
			data:    []byte{0x00, 0x00, 0x00, 0x10, 0x01, 0x02}, // Says 16 bytes but only has 2
			wantErr: true,
		},
		{
			name:    "invalid protobuf",
			data:    []byte{0x00, 0x00, 0x00, 0x04, 0xFF, 0xFF, 0xFF, 0xFF}, // Invalid protobuf data
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := bytes.NewBuffer(tt.data)
			_, err := ReadMessage(buf)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProtocolRoundTrip(t *testing.T) {
	events := []*proto.MouseEvent{
		{
			Type:        proto.EventType_EVENT_TYPE_MOVE,
			X:           123.456,
			Y:           789.012,
			TimestampMs: time.Now().UnixMilli(),
		},
		{
			Type:        proto.EventType_EVENT_TYPE_SCROLL,
			X:           500,
			Y:           600,
			Direction:   proto.ScrollDirection_SCROLL_DIRECTION_UP,
			TimestampMs: time.Now().UnixMilli() + 10,
		},
		{
			Type:        proto.EventType_EVENT_TYPE_CLICK,
			X:           700,
			Y:           800,
			Button:      proto.MouseButton_MOUSE_BUTTON_MIDDLE,
			IsPressed:   false,
			TimestampMs: time.Now().UnixMilli() + 20,
		},
	}

	buf := &bytes.Buffer{}

	// Write all events
	for _, event := range events {
		err := WriteMessage(buf, event)
		if err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	// Read all events back
	for i, expected := range events {
		decoded, err := ReadMessage(buf)
		if err != nil {
			t.Fatalf("Failed to read event %d: %v", i, err)
		}

		if !pb.Equal(expected, decoded) {
			t.Errorf("Event %d not equal after round trip", i)
		}
	}

	// Buffer should be empty
	_, err := ReadMessage(buf)
	if err == nil {
		t.Error("Expected error reading from empty buffer")
	}
}
