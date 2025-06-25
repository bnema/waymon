package ipc

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/bnema/waymon/internal/logger"
	pb "github.com/bnema/waymon/internal/proto"
	"google.golang.org/protobuf/proto"
)

// Client handles IPC communication with a running waymon instance
type Client struct {
	socketPath string
	timeout    time.Duration
}

// NewClient creates a new IPC client
func NewClient() (*Client, error) {
	// Try server socket first (predictable location)
	serverSocketPath := "/tmp/waymon.sock"

	// Check if server socket exists
	if _, err := net.DialTimeout("unix", serverSocketPath, 100*time.Millisecond); err == nil {
		return &Client{
			socketPath: serverSocketPath,
			timeout:    5 * time.Second,
		}, nil
	}

	// Fall back to user-specific socket path
	socketPath, err := GetSocketPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get socket path: %w", err)
	}

	return &Client{
		socketPath: socketPath,
		timeout:    5 * time.Second,
	}, nil
}

// NewClientWithTimeout creates a new IPC client with custom timeout
func NewClientWithTimeout(timeout time.Duration) (*Client, error) {
	client, err := NewClient()
	if err != nil {
		return nil, err
	}
	client.timeout = timeout
	return client, nil
}

// SendSwitch sends a switch command to the running waymon instance
func (c *Client) SendSwitch(action pb.SwitchAction) (*pb.StatusResponse, error) {
	msg, err := NewSwitchMessage(action)
	if err != nil {
		return nil, fmt.Errorf("failed to create switch message: %w", err)
	}

	response, err := c.sendMessage(msg)
	if err != nil {
		return nil, err
	}

	switch response.Type {
	case pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS_RESPONSE:
		return GetStatusResponse(response)
	case pb.IPCMessageType_IPC_MESSAGE_TYPE_ERROR:
		errResp, _ := GetErrorResponse(response)
		return nil, fmt.Errorf("server error: %s", errResp.Error)
	default:
		return nil, fmt.Errorf("unexpected response type: %s", response.Type)
	}
}

// SendSwitchNext sends a "switch to next computer" command
func (c *Client) SendSwitchNext() (*pb.StatusResponse, error) {
	return c.SendSwitch(pb.SwitchAction_SWITCH_ACTION_NEXT)
}

// SendSwitchPrevious sends a "switch to previous computer" command
func (c *Client) SendSwitchPrevious() (*pb.StatusResponse, error) {
	return c.SendSwitch(pb.SwitchAction_SWITCH_ACTION_PREVIOUS)
}

// SendSwitchLegacy sends a legacy switch command (for backward compatibility)
func (c *Client) SendSwitchLegacy(enable *bool) (*pb.StatusResponse, error) {
	msg, err := NewSwitchMessageLegacy(enable)
	if err != nil {
		return nil, fmt.Errorf("failed to create switch message: %w", err)
	}

	response, err := c.sendMessage(msg)
	if err != nil {
		return nil, err
	}

	switch response.Type {
	case pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS_RESPONSE:
		return GetStatusResponse(response)
	case pb.IPCMessageType_IPC_MESSAGE_TYPE_ERROR:
		errResp, _ := GetErrorResponse(response)
		return nil, fmt.Errorf("server error: %s", errResp.Error)
	default:
		return nil, fmt.Errorf("unexpected response type: %s", response.Type)
	}
}

// SendStatus sends a status query to the running waymon instance
func (c *Client) SendStatus() (*pb.StatusResponse, error) {
	msg, err := NewStatusMessage()
	if err != nil {
		return nil, fmt.Errorf("failed to create status message: %w", err)
	}

	response, err := c.sendMessage(msg)
	if err != nil {
		return nil, err
	}

	switch response.Type {
	case pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS_RESPONSE:
		return GetStatusResponse(response)
	case pb.IPCMessageType_IPC_MESSAGE_TYPE_ERROR:
		errResp, _ := GetErrorResponse(response)
		return nil, fmt.Errorf("server error: %s", errResp.Error)
	default:
		return nil, fmt.Errorf("unexpected response type: %s", response.Type)
	}
}

// IsWaymonRunning checks if a waymon instance is currently running
func (c *Client) IsWaymonRunning() bool {
	_, err := c.SendStatus()
	return err == nil
}

// GetStatus is a convenience method that calls SendStatus
func (c *Client) GetStatus() (*pb.StatusResponse, error) {
	return c.SendStatus()
}

// SendRelease sends a release command to return control to the local machine
func (c *Client) SendRelease() error {
	msg, err := NewReleaseMessage()
	if err != nil {
		return fmt.Errorf("failed to create release message: %w", err)
	}

	response, err := c.sendMessage(msg)
	if err != nil {
		return err
	}

	// Check for error response
	if response.Type == pb.IPCMessageType_IPC_MESSAGE_TYPE_ERROR {
		errResp, _ := GetErrorResponse(response)
		if errResp != nil {
			return fmt.Errorf("server error: %s", errResp.Error)
		}
	}

	return nil
}

// SendConnect sends a connect command to switch to a specific computer slot
func (c *Client) SendConnect(slot int32) error {
	if slot < 1 || slot > 5 {
		return fmt.Errorf("invalid slot number: %d (must be 1-5)", slot)
	}

	msg, err := NewConnectMessage(slot)
	if err != nil {
		return fmt.Errorf("failed to create connect message: %w", err)
	}

	response, err := c.sendMessage(msg)
	if err != nil {
		return err
	}

	// Check for error response
	if response.Type == pb.IPCMessageType_IPC_MESSAGE_TYPE_ERROR {
		errResp, _ := GetErrorResponse(response)
		if errResp != nil {
			return fmt.Errorf("server error: %s", errResp.Error)
		}
	}

	return nil
}

// sendMessage sends a message and returns the response
func (c *Client) sendMessage(msg *pb.IPCMessage) (*pb.IPCMessage, error) {
	// Connect to socket
	conn, err := net.DialTimeout("unix", c.socketPath, c.timeout)
	if err != nil {
		if isConnectionRefused(err) {
			return nil, fmt.Errorf("waymon is not running")
		}
		return nil, fmt.Errorf("failed to connect to waymon: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Errorf("Failed to close IPC connection: %v", err)
		}
	}()

	// Set connection timeout
	if err := conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		logger.Warnf("Failed to set connection deadline: %v", err)
	}

	// Send message
	if err := c.writeMessage(conn, msg); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Read response
	response, err := c.readMessage(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return response, nil
}

// readMessage reads a protobuf message from the connection
func (c *Client) readMessage(conn net.Conn) (*pb.IPCMessage, error) {
	// Read message length (4 bytes, big endian)
	var length uint32
	if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	// Read message data
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, fmt.Errorf("failed to read message data: %w", err)
	}

	// Unmarshal protobuf message
	var msg pb.IPCMessage
	if err := proto.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}

// writeMessage writes a protobuf message to the connection
func (c *Client) writeMessage(conn net.Conn, msg *pb.IPCMessage) error {
	// Marshal protobuf message
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Write message length (4 bytes, big endian)
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

// isConnectionRefused checks if the error is a connection refused error
func isConnectionRefused(err error) bool {
	if netErr, ok := err.(*net.OpError); ok {
		if netErr.Op == "dial" {
			return true
		}
	}
	return false
}

// Close closes the client connection
func (c *Client) Close() error {
	// Nothing to close as we create connections per request
	return nil
}

// IsWaymonRunning checks if the waymon server is running
func IsWaymonRunning() bool {
	client, err := NewClient()
	if err != nil {
		return false
	}
	defer func() { 
		if err := client.Close(); err != nil {
			logger.Errorf("Failed to close IPC client: %v", err)
		}
	}()

	// Try to get status - if we can connect, server is running
	_, err = client.SendStatus()
	return err == nil
}
