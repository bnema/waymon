package ipc

import (
	"fmt"
	"net"
	"time"

	"github.com/bnema/waymon/internal/boundaries/in"
)

// Client handles IPC communication with a running waymon instance.
type Client struct {
	socketPath string
	timeout    time.Duration
}

// NewClient creates a new IPC client.
// It will automatically detect the socket path.
func NewClient() (*Client, error) {
	// Try server socket first (predictable location for root)
	serverSocketPath := "/tmp/waymon.sock"

	// Check if server socket exists and is connectable
	conn, err := net.DialTimeout("unix", serverSocketPath, 100*time.Millisecond)
	if err == nil {
		conn.Close()
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

// NewClientWithPath creates a new IPC client with a specific socket path.
// This is useful for testing.
func NewClientWithPath(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
		timeout:    5 * time.Second,
	}
}

// NewClientWithTimeout creates a new IPC client with a custom timeout.
func NewClientWithTimeout(timeout time.Duration) (*Client, error) {
	client, err := NewClient()
	if err != nil {
		return nil, err
	}
	client.timeout = timeout
	return client, nil
}

// SetTimeout sets the timeout for IPC operations.
func (c *Client) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// SocketPath returns the socket path being used.
func (c *Client) SocketPath() string {
	return c.socketPath
}

// SendSwitch sends a switch command to the running waymon instance.
func (c *Client) SendSwitch(action in.SwitchAction) (*in.StatusResponse, error) {
	msg, err := NewSwitchMessage(action)
	if err != nil {
		return nil, fmt.Errorf("failed to create switch message: %w", err)
	}

	response, err := c.sendMessage(msg)
	if err != nil {
		return nil, err
	}

	return c.handleResponse(response)
}

// SendSwitchNext sends a "switch to next computer" command.
func (c *Client) SendSwitchNext() (*in.StatusResponse, error) {
	return c.SendSwitch(in.SwitchActionNext)
}

// SendSwitchPrevious sends a "switch to previous computer" command.
func (c *Client) SendSwitchPrevious() (*in.StatusResponse, error) {
	return c.SendSwitch(in.SwitchActionPrevious)
}

// SendStatus sends a status query to the running waymon instance.
func (c *Client) SendStatus() (*in.StatusResponse, error) {
	msg := NewStatusMessage()

	response, err := c.sendMessage(msg)
	if err != nil {
		return nil, err
	}

	return c.handleResponse(response)
}

// SendRelease sends a release command to return control to the local machine.
func (c *Client) SendRelease() (*in.StatusResponse, error) {
	msg := NewReleaseMessage()

	response, err := c.sendMessage(msg)
	if err != nil {
		return nil, err
	}

	return c.handleResponse(response)
}

// SendConnect sends a connect command to switch to a specific computer slot.
func (c *Client) SendConnect(slot int32) (*in.StatusResponse, error) {
	if slot < 0 {
		return nil, fmt.Errorf("invalid slot number: %d (must be >= 0)", slot)
	}

	msg, err := NewConnectMessage(slot)
	if err != nil {
		return nil, fmt.Errorf("failed to create connect message: %w", err)
	}

	response, err := c.sendMessage(msg)
	if err != nil {
		return nil, err
	}

	return c.handleResponse(response)
}

// SendStop sends a stop command to shut down the running waymon instance.
func (c *Client) SendStop() error {
	msg := NewStopMessage()

	response, err := c.sendMessage(msg)
	if err != nil {
		return err
	}

	switch response.Type {
	case MessageTypeOK:
		return nil
	case MessageTypeError:
		errMsg, _ := ParseErrorResponse(response)
		return fmt.Errorf("server error: %s", errMsg)
	default:
		return fmt.Errorf("unexpected response type: %s", response.Type)
	}
}

// IsRunning checks if a waymon instance is currently running.
func (c *Client) IsRunning() bool {
	_, err := c.SendStatus()
	return err == nil
}

// handleResponse processes a response message.
func (c *Client) handleResponse(response *Message) (*in.StatusResponse, error) {
	switch response.Type {
	case MessageTypeStatusResponse:
		return ParseStatusResponse(response)
	case MessageTypeError:
		errMsg, _ := ParseErrorResponse(response)
		return nil, fmt.Errorf("server error: %s", errMsg)
	default:
		return nil, fmt.Errorf("unexpected response type: %s", response.Type)
	}
}

// sendMessage sends a message and returns the response.
func (c *Client) sendMessage(msg *Message) (*Message, error) {
	// Connect to socket
	conn, err := net.DialTimeout("unix", c.socketPath, c.timeout)
	if err != nil {
		if isConnectionRefused(err) {
			return nil, fmt.Errorf("waymon is not running (socket: %s)", c.socketPath)
		}
		return nil, fmt.Errorf("failed to connect to waymon: %w", err)
	}
	defer conn.Close()

	// Set connection timeout
	if err := conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		// Non-fatal, continue anyway
		_ = err
	}

	// Send message
	if err := WriteMessage(conn, msg); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Read response
	response, err := ReadMessage(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return response, nil
}

// isConnectionRefused checks if the error is a connection refused error.
func isConnectionRefused(err error) bool {
	if netErr, ok := err.(*net.OpError); ok {
		return netErr.Op == "dial"
	}
	return false
}

// IsWaymonRunning is a convenience function to check if waymon is running.
func IsWaymonRunning() bool {
	client, err := NewClient()
	if err != nil {
		return false
	}
	return client.IsRunning()
}
