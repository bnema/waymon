package network

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/bnema/waymon/internal/proto"
	pb "google.golang.org/protobuf/proto"
)

const (
	// MaxMessageSize is the maximum size of a single message (1MB)
	MaxMessageSize = 1024 * 1024
)

// WriteMessage writes a protobuf message with length prefix
func WriteMessage(w io.Writer, msg *proto.MouseEvent) error {
	if msg == nil {
		return fmt.Errorf("cannot write nil message")
	}

	// Marshal the message
	data, err := pb.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Write length prefix (4 bytes, big endian)
	dataLen := len(data)
	if dataLen > MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes", dataLen)
	}
	length := uint32(dataLen) // Safe conversion, checked above
	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return fmt.Errorf("failed to write length: %w", err)
	}

	// Write the message data
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	return nil
}

// ReadMessage reads a protobuf message with length prefix
func ReadMessage(r io.Reader) (*proto.MouseEvent, error) {
	// Read length prefix
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("failed to read length: %w", err)
	}

	// Sanity check
	if length > MaxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}

	// Read the message data
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	// Unmarshal the message
	msg := &proto.MouseEvent{}
	if err := pb.Unmarshal(data, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return msg, nil
}

// EventSender sends mouse events over a connection
type EventSender struct {
	conn io.Writer
}

// NewEventSender creates a new event sender
func NewEventSender(conn io.Writer) *EventSender {
	return &EventSender{conn: conn}
}

// Send sends a mouse event
func (s *EventSender) Send(event *proto.MouseEvent) error {
	return WriteMessage(s.conn, event)
}

// EventReceiver receives mouse events from a connection
type EventReceiver struct {
	conn io.Reader
}

// NewEventReceiver creates a new event receiver
func NewEventReceiver(conn io.Reader) *EventReceiver {
	return &EventReceiver{conn: conn}
}

// Receive receives the next mouse event
func (r *EventReceiver) Receive() (*proto.MouseEvent, error) {
	return ReadMessage(r.conn)
}