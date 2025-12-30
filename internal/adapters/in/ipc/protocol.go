// Package ipc provides Unix socket-based IPC for CLI commands.
package ipc

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"

	"github.com/bnema/waymon/internal/boundaries/in"
)

// MessageType represents the type of IPC message.
type MessageType string

// MessageType constants define the available IPC message types.
const (
	// MessageTypeSwitch is a request to switch control.
	MessageTypeSwitch MessageType = "switch"
	// MessageTypeStatus is a request for status information.
	MessageTypeStatus MessageType = "status"
	// MessageTypeRelease is a request to release control.
	MessageTypeRelease MessageType = "release"
	// MessageTypeConnect is a request to connect to a slot.
	MessageTypeConnect MessageType = "connect"
	// MessageTypeStop is a request to stop the server.
	MessageTypeStop MessageType = "stop"

	// MessageTypeStatusResponse is a response containing status information.
	MessageTypeStatusResponse MessageType = "status_response"
	// MessageTypeError is a response indicating an error.
	MessageTypeError MessageType = "error"
	// MessageTypeOK is a response indicating success.
	MessageTypeOK MessageType = "ok"
)

// Message is the envelope for all IPC messages.
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// SwitchRequest contains the switch command parameters.
type SwitchRequest struct {
	Action string `json:"action"` // "next", "previous", "enable", "disable"
}

// ConnectRequest contains the connect command parameters.
type ConnectRequest struct {
	Slot int32 `json:"slot"`
}

// StatusResponse contains the status information.
type StatusResponse struct {
	Active        bool     `json:"active"`
	Connected     bool     `json:"connected"`
	ServerHost    string   `json:"server_host,omitempty"`
	CurrentIndex  int32    `json:"current_index"`
	TotalCount    int32    `json:"total_count"`
	ComputerNames []string `json:"computer_names,omitempty"`
}

// ErrorResponse contains error information.
type ErrorResponse struct {
	Error string `json:"error"`
}

// NewSwitchMessage creates a switch request message.
func NewSwitchMessage(action in.SwitchAction) (*Message, error) {
	actionStr := ""
	switch action {
	case in.SwitchActionNext:
		actionStr = "next"
	case in.SwitchActionPrevious:
		actionStr = "previous"
	case in.SwitchActionEnable:
		actionStr = "enable"
	case in.SwitchActionDisable:
		actionStr = "disable"
	default:
		return nil, fmt.Errorf("unknown switch action: %d", action)
	}

	payload, err := json.Marshal(SwitchRequest{Action: actionStr})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal switch request: %w", err)
	}

	return &Message{
		Type:    MessageTypeSwitch,
		Payload: payload,
	}, nil
}

// NewStatusMessage creates a status request message.
func NewStatusMessage() *Message {
	return &Message{Type: MessageTypeStatus}
}

// NewReleaseMessage creates a release request message.
func NewReleaseMessage() *Message {
	return &Message{Type: MessageTypeRelease}
}

// NewConnectMessage creates a connect request message.
func NewConnectMessage(slot int32) (*Message, error) {
	payload, err := json.Marshal(ConnectRequest{Slot: slot})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal connect request: %w", err)
	}

	return &Message{
		Type:    MessageTypeConnect,
		Payload: payload,
	}, nil
}

// NewStopMessage creates a stop request message.
func NewStopMessage() *Message {
	return &Message{Type: MessageTypeStop}
}

// NewStatusResponseMessage creates a status response message.
func NewStatusResponseMessage(resp *in.StatusResponse) (*Message, error) {
	payload, err := json.Marshal(StatusResponse{
		Active:        resp.Active,
		Connected:     resp.Connected,
		ServerHost:    resp.ServerHost,
		CurrentIndex:  resp.CurrentIndex,
		TotalCount:    resp.TotalCount,
		ComputerNames: resp.ComputerNames,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal status response: %w", err)
	}

	return &Message{
		Type:    MessageTypeStatusResponse,
		Payload: payload,
	}, nil
}

// NewErrorMessage creates an error response message.
func NewErrorMessage(errMsg string) (*Message, error) {
	payload, err := json.Marshal(ErrorResponse{Error: errMsg})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal error response: %w", err)
	}

	return &Message{
		Type:    MessageTypeError,
		Payload: payload,
	}, nil
}

// NewOKMessage creates an OK response message.
func NewOKMessage() *Message {
	return &Message{Type: MessageTypeOK}
}

// ParseSwitchRequest parses a switch request from a message.
func ParseSwitchRequest(msg *Message) (in.SwitchAction, error) {
	if msg.Type != MessageTypeSwitch {
		return 0, fmt.Errorf("message is not a switch request")
	}

	var req SwitchRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return 0, fmt.Errorf("failed to unmarshal switch request: %w", err)
	}

	switch req.Action {
	case "next":
		return in.SwitchActionNext, nil
	case "previous":
		return in.SwitchActionPrevious, nil
	case "enable":
		return in.SwitchActionEnable, nil
	case "disable":
		return in.SwitchActionDisable, nil
	default:
		return 0, fmt.Errorf("unknown switch action: %s", req.Action)
	}
}

// ParseConnectRequest parses a connect request from a message.
func ParseConnectRequest(msg *Message) (int32, error) {
	if msg.Type != MessageTypeConnect {
		return 0, fmt.Errorf("message is not a connect request")
	}

	var req ConnectRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return 0, fmt.Errorf("failed to unmarshal connect request: %w", err)
	}

	return req.Slot, nil
}

// ParseStatusResponse parses a status response from a message.
func ParseStatusResponse(msg *Message) (*in.StatusResponse, error) {
	if msg.Type != MessageTypeStatusResponse {
		return nil, fmt.Errorf("message is not a status response")
	}

	var resp StatusResponse
	if err := json.Unmarshal(msg.Payload, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal status response: %w", err)
	}

	return &in.StatusResponse{
		Active:        resp.Active,
		Connected:     resp.Connected,
		ServerHost:    resp.ServerHost,
		CurrentIndex:  resp.CurrentIndex,
		TotalCount:    resp.TotalCount,
		ComputerNames: resp.ComputerNames,
	}, nil
}

// ParseErrorResponse parses an error response from a message.
func ParseErrorResponse(msg *Message) (string, error) {
	if msg.Type != MessageTypeError {
		return "", fmt.Errorf("message is not an error response")
	}

	var resp ErrorResponse
	if err := json.Unmarshal(msg.Payload, &resp); err != nil {
		return "", fmt.Errorf("failed to unmarshal error response: %w", err)
	}

	return resp.Error, nil
}

// WriteMessage writes a message to the connection with length-prefix framing.
func WriteMessage(conn net.Conn, msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Write length prefix (4 bytes, big endian)
	length := uint32(len(data)) //nolint:gosec // message length within uint32 range
	if err := binary.Write(conn, binary.BigEndian, length); err != nil {
		return fmt.Errorf("failed to write message length: %w", err)
	}

	// Write message data
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to write message data: %w", err)
	}

	return nil
}

// ReadMessage reads a message from the connection with length-prefix framing.
func ReadMessage(conn net.Conn) (*Message, error) {
	// Read length prefix (4 bytes, big endian)
	var length uint32
	if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	// Sanity check on length
	if length > 1024*1024 { // 1MB max
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}

	// Read message data
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, fmt.Errorf("failed to read message data: %w", err)
	}

	// Unmarshal message
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}
