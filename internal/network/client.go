// Package network handles TCP networking for mouse event transmission
package network

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	waymonProto "github.com/bnema/waymon/internal/proto"
	"google.golang.org/protobuf/proto"
)

// Client handles outgoing mouse event connections
type Client struct {
	conn     net.Conn
	mu       sync.Mutex
	stopOnce sync.Once
	stop     chan struct{}
	wg       sync.WaitGroup
}

// NewClient creates a new client instance
func NewClient() *Client {
	return &Client{
		stop: make(chan struct{}),
	}
}

// Connect establishes a connection to the server
func (c *Client) Connect(ctx context.Context, address string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return fmt.Errorf("already connected")
	}

	// Create a dialer with context
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.conn = conn
	c.stop = make(chan struct{}) // Reset stop channel for new connection
	c.stopOnce = sync.Once{}     // Reset sync.Once

	// Start read loop
	c.wg.Add(1)
	go c.readLoop()

	return nil
}

// Disconnect closes the connection to the server
func (c *Client) Disconnect() {
	c.stopOnce.Do(func() {
		close(c.stop)

		c.mu.Lock()
		if c.conn != nil {
			_ = c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()

		c.wg.Wait()
	})
}

// IsConnected returns true if the client is connected
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

func (c *Client) readLoop() {
	defer c.wg.Done()
	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			_ = c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
	}()

	buf := make([]byte, 1024)
	for {
		select {
		case <-c.stop:
			return
		default:
			// Set read deadline to allow checking stop channel
			_ = c.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			_, err := c.conn.Read(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				// Connection error, disconnect
				return
			}
			// Process received data (placeholder for now)
		}
	}
}

// SendEvent sends a mouse event to the server
func (c *Client) SendEvent(event *waymonProto.MouseEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	// Marshal the event to protobuf
	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Write the length prefix (4 bytes)
	lengthBuf := make([]byte, 4)
	lengthBuf[0] = byte(len(data) >> 24)
	lengthBuf[1] = byte(len(data) >> 16)
	lengthBuf[2] = byte(len(data) >> 8)
	lengthBuf[3] = byte(len(data))

	// Write length prefix
	if _, err := c.conn.Write(lengthBuf); err != nil {
		return fmt.Errorf("failed to write length: %w", err)
	}

	// Write the event data
	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	return nil
}
