package network

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/bnema/waymon/internal/protocol"
	"google.golang.org/protobuf/proto"
)

// TestHandshakeConsumption tests that the handshake fully consumes welcome messages
func TestHandshakeConsumption(t *testing.T) {
	// Simulate server response with welcome messages followed by protocol data
	var buf bytes.Buffer

	// Write welcome messages (what server sends)
	welcome := "Waymon SSH connection established\nPublic key: SHA256:testkey\n"
	buf.WriteString(welcome)

	// Write a protocol buffer message
	testEvent := &protocol.InputEvent{
		Event: &protocol.InputEvent_Control{
			Control: &protocol.ControlEvent{
				Type: protocol.ControlEvent_CLIENT_CONFIG,
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "server",
	}

	// Marshal the event
	data, err := proto.Marshal(testEvent)
	if err != nil {
		t.Fatalf("Failed to marshal test event: %v", err)
	}

	// Write length prefix
	length := len(data)
	lengthBuf := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}
	buf.Write(lengthBuf)
	buf.Write(data)

	// Now simulate the client's handshake reading
	reader := bytes.NewReader(buf.Bytes())

	// Read like the client does in the handshake
	handshakeBuf := make([]byte, 0, 512)
	tempBuf := make([]byte, 1)

	for {
		n, err := reader.Read(tempBuf)
		if err != nil || n == 0 {
			t.Fatalf("Unexpected error during handshake read: %v", err)
		}
		handshakeBuf = append(handshakeBuf, tempBuf[0])

		response := string(handshakeBuf)
		if strings.Contains(response, "Waymon SSH connection established") &&
			strings.Contains(response, "Public key:") &&
			strings.Count(response, "\n") >= 2 {
			// Handshake complete
			break
		}

		if len(handshakeBuf) > 256 {
			t.Fatal("Handshake buffer too large")
		}
	}

	// Check what's left in the reader
	remaining, _ := io.ReadAll(reader)
	t.Logf("Handshake consumed: %q", string(handshakeBuf))
	t.Logf("Remaining bytes: %d", len(remaining))

	// The remaining should start with the length prefix
	if len(remaining) < 4 {
		t.Fatalf("Not enough remaining bytes for length prefix: %d", len(remaining))
	}

	// Check if the first 4 bytes are a valid length prefix
	remainingLength := int(remaining[0])<<24 | int(remaining[1])<<16 | int(remaining[2])<<8 | int(remaining[3])
	t.Logf("Remaining length prefix: %d (bytes: %02x %02x %02x %02x)",
		remainingLength, remaining[0], remaining[1], remaining[2], remaining[3])

	if remainingLength != len(data) {
		t.Errorf("Invalid length prefix: expected %d, got %d", len(data), remainingLength)
		// Show what we actually got
		if len(remaining) >= 8 {
			t.Logf("First 8 remaining bytes as hex: %02x %02x %02x %02x %02x %02x %02x %02x",
				remaining[0], remaining[1], remaining[2], remaining[3],
				remaining[4], remaining[5], remaining[6], remaining[7])
			t.Logf("First 8 remaining bytes as ASCII: %q", string(remaining[:8]))
		}
	}
}

// TestProtocolReaderAfterText tests reading protocol messages after text
func TestProtocolReaderAfterText(t *testing.T) {
	// Create a buffer with text followed by protocol data
	var buf bytes.Buffer

	// Write some text that might not be fully consumed
	buf.WriteString("server info text that might leak\n")

	// Write a valid protocol message
	testEvent := &protocol.InputEvent{
		Event: &protocol.InputEvent_Control{
			Control: &protocol.ControlEvent{
				Type: protocol.ControlEvent_CLIENT_CONFIG,
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "server",
	}

	data, _ := proto.Marshal(testEvent)
	length := len(data)
	lengthBuf := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}
	buf.Write(lengthBuf)
	buf.Write(data)

	reader := bytes.NewReader(buf.Bytes())

	// Try to read protocol message without consuming text first
	protocolLengthBuf := make([]byte, 4)
	_, err := io.ReadFull(reader, protocolLengthBuf)
	if err != nil {
		t.Fatalf("Failed to read length: %v", err)
	}

	// This will read "serv" as the length prefix
	decodedLength := int(protocolLengthBuf[0])<<24 | int(protocolLengthBuf[1])<<16 |
		int(protocolLengthBuf[2])<<8 | int(protocolLengthBuf[3])

	t.Logf("Read bytes as length: %02x %02x %02x %02x",
		protocolLengthBuf[0], protocolLengthBuf[1], protocolLengthBuf[2], protocolLengthBuf[3])
	t.Logf("Read bytes as ASCII: %q", string(protocolLengthBuf))
	t.Logf("Decoded length: %d", decodedLength)

	// This should be an invalid length
	if decodedLength > 0 && decodedLength <= 4096 {
		t.Errorf("Expected invalid length when reading text as protocol, but got valid length: %d", decodedLength)
	}
}
