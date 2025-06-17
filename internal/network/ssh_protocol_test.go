package network

import (
	"bytes"
	"io"
	"testing"

	"github.com/bnema/waymon/internal/protocol"
	"google.golang.org/protobuf/proto"
)

// TestProtocolReaderWithMixedContent tests the protocol reader's ability to handle mixed text and protocol data
func TestProtocolReaderWithMixedContent(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantText string
		wantErr  bool
	}{
		{
			name:     "text followed by protocol",
			input:    buildMixedContent("Server message\n", buildProtocolMessage(t)),
			wantText: "Server message\n",
			wantErr:  false,
		},
		{
			name:     "max clients error",
			input:    []byte("Server already has maximum number of active clients\n"),
			wantText: "Server already has maximum number of active clients\n",
			wantErr:  false,
		},
		{
			name:     "pure protocol data",
			input:    buildProtocolMessage(t),
			wantText: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.input)
			
			// Simulate reading like the SSH client does
			lengthBuf := make([]byte, 4)
			n, err := io.ReadFull(reader, lengthBuf)
			if err != nil && err != io.EOF {
				t.Fatalf("Failed to read length: %v", err)
			}
			
			if n < 4 {
				// Not enough data for a length prefix
				return
			}
			
			// Check if it's text
			isText := true
			for _, b := range lengthBuf {
				if b < 32 || b > 126 {
					isText = false
					break
				}
			}
			
			if isText {
				// It's text data
				textBuf := append([]byte{}, lengthBuf...)
				// Read rest of line
				for {
					b := make([]byte, 1)
					n, err := reader.Read(b)
					if err != nil || n == 0 {
						break
					}
					textBuf = append(textBuf, b[0])
					if b[0] == '\n' {
						break
					}
				}
				
				gotText := string(textBuf)
				if gotText != tt.wantText && tt.wantText != "" {
					t.Errorf("Got text %q, want %q", gotText, tt.wantText)
				}
			} else {
				// It's protocol data
				length := int(lengthBuf[0])<<24 | int(lengthBuf[1])<<16 | int(lengthBuf[2])<<8 | int(lengthBuf[3])
				if length <= 0 || length > 4096 {
					if !tt.wantErr {
						t.Errorf("Invalid protocol length: %d", length)
					}
					return
				}
				
				// Read protocol message
				msgBuf := make([]byte, length)
				_, err := io.ReadFull(reader, msgBuf)
				if err != nil {
					t.Fatalf("Failed to read protocol message: %v", err)
				}
				
				// Try to unmarshal
				var inputEvent protocol.InputEvent
				if err := proto.Unmarshal(msgBuf, &inputEvent); err != nil {
					t.Errorf("Failed to unmarshal protocol message: %v", err)
				}
			}
		})
	}
}

func buildProtocolMessage(t *testing.T) []byte {
	// Create a test event
	event := &protocol.InputEvent{
		Event: &protocol.InputEvent_Control{
			Control: &protocol.ControlEvent{
				Type: protocol.ControlEvent_HEALTH_CHECK_PONG,
			},
		},
		Timestamp: 12345,
		SourceId:  "test",
	}
	
	data, err := proto.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal test event: %v", err)
	}
	
	// Add length prefix
	length := len(data)
	lengthBuf := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}
	
	return append(lengthBuf, data...)
}

func buildMixedContent(text string, protocolData []byte) []byte {
	return append([]byte(text), protocolData...)
}