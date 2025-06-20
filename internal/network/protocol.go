package network

import (
	"fmt"
	"io"

	"github.com/bnema/waymon/internal/protocol"
	"google.golang.org/protobuf/proto"
)

// writeInputMessage writes an InputEvent message with length prefix
func writeInputMessage(w io.Writer, event *protocol.InputEvent) error {
	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal input event: %w", err)
	}

	// Write length prefix (4 bytes, big-endian)
	length := len(data)
	lengthBuf := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}

	if _, err := w.Write(lengthBuf); err != nil {
		return fmt.Errorf("failed to write length: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Force flush if the writer supports it
	if flusher, ok := w.(interface{ Flush() error }); ok {
		_ = flusher.Flush() // Best effort flush, already wrote data
	}

	return nil
}