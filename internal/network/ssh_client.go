package network

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/protocol"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"
)

// SSHClient handles SSH connections to the server
type SSHClient struct {
	client  *ssh.Client
	session *ssh.Session
	writer  io.Writer
	reader  io.Reader

	mu        sync.Mutex
	connected bool

	// SSH key paths
	privateKeyPath string

	// Event handling
	onInputEvent func(*protocol.InputEvent)
}

// NewSSHClient creates a new SSH client
func NewSSHClient(privateKeyPath string) *SSHClient {
	return &SSHClient{
		privateKeyPath: privateKeyPath,
	}
}

// Connect establishes an SSH connection to the server
func (c *SSHClient) Connect(ctx context.Context, serverAddr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("already connected")
	}

	// Load private key
	keyData, err := os.ReadFile(c.privateKeyPath)
	if err != nil {
		// Try expanding the path if it starts with ~
		if strings.HasPrefix(c.privateKeyPath, "~") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			expandedPath := filepath.Join(homeDir, c.privateKeyPath[1:])
			keyData, err = os.ReadFile(expandedPath)
			if err != nil {
				return fmt.Errorf("failed to read private key from %s: %w", expandedPath, err)
			}
		} else {
			return fmt.Errorf("failed to read private key: %w", err)
		}
	}

	// Parse private key
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		// Check if it's encrypted
		if strings.Contains(err.Error(), "encrypted") {
			return fmt.Errorf("SSH key is encrypted - encrypted keys are not supported")
		}
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Get the username
	username := os.Getenv("USER")
	if username == "" {
		username = "waymon"
	}

	// Create SSH client config
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			// For now, accept any host key
			// TODO: Implement proper host key verification
			logger.Debugf("Accepting host key for %s: %s", hostname, ssh.FingerprintSHA256(key))
			return nil
		},
		Timeout: 10 * time.Second,
	}

	// Connect to SSH server
	client, err := ssh.Dial("tcp", serverAddr, config)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	// Create a session
	session, err := client.NewSession()
	if err != nil {
		if err := client.Close(); err != nil {
			logger.Errorf("Failed to close SSH client: %v", err)
		}
		return fmt.Errorf("failed to create SSH session: %w", err)
	}

	// Get stdin pipe for sending data
	writer, err := session.StdinPipe()
	if err != nil {
		if err := session.Close(); err != nil {
			logger.Errorf("Failed to close SSH session: %v", err)
		}
		if err := client.Close(); err != nil {
			logger.Errorf("Failed to close SSH client: %v", err)
		}
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	// Get stdout for receiving input events from server
	reader, err := session.StdoutPipe()
	if err != nil {
		if err := session.Close(); err != nil {
			logger.Errorf("Failed to close SSH session: %v", err)
		}
		if err := client.Close(); err != nil {
			logger.Errorf("Failed to close SSH client: %v", err)
		}
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start the session
	if err := session.Shell(); err != nil {
		if err := session.Close(); err != nil {
			logger.Errorf("Failed to close SSH session: %v", err)
		}
		if err := client.Close(); err != nil {
			logger.Errorf("Failed to close SSH client: %v", err)
		}
		return fmt.Errorf("failed to start SSH session: %w", err)
	}

	// No handshake messages expected from server anymore
	// Server only sends protocol buffer messages or error text

	c.client = client
	c.session = session
	c.writer = writer
	c.reader = reader
	c.connected = true

	// Start receiving input events from server
	logger.Info("[SSH-CLIENT] Starting receiveInputEvents goroutine")
	go c.receiveInputEvents(ctx)

	// Monitor connection
	go c.monitorConnection(ctx)

	return nil
}

// Disconnect closes the SSH connection
func (c *SSHClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false

	if c.session != nil {
		if err := c.session.Close(); err != nil {
			logger.Errorf("Failed to close SSH session: %v", err)
		}
		c.session = nil
	}

	if c.client != nil {
		if err := c.client.Close(); err != nil {
			logger.Errorf("Failed to close SSH client: %v", err)
		}
		c.client = nil
	}

	c.writer = nil
	c.reader = nil

	return nil
}

// IsConnected returns true if connected to the server
func (c *SSHClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// Reconnect attempts to reconnect to the server
func (c *SSHClient) Reconnect(ctx context.Context, serverAddr string) error {
	if err := c.Disconnect(); err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	return c.Connect(ctx, serverAddr)
}


// SendInputEvent sends an input event to the server
func (c *SSHClient) SendInputEvent(event *protocol.InputEvent) error {
	c.mu.Lock()
	writer := c.writer
	connected := c.connected
	c.mu.Unlock()

	if !connected || writer == nil {
		return fmt.Errorf("not connected")
	}

	return writeInputMessage(writer, event)
}

// OnInputEvent sets the callback for receiving input events from server
func (c *SSHClient) OnInputEvent(callback func(*protocol.InputEvent)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onInputEvent = callback
}

// receiveInputEvents continuously receives input events from the server
func (c *SSHClient) receiveInputEvents(ctx context.Context) {
	logger.Info("[SSH-CLIENT] Starting receiveInputEvents goroutine")

	// Buffer to store any text messages
	textBuffer := make([]byte, 0, 1024)

	for {
		select {
		case <-ctx.Done():
			logger.Info("[SSH-CLIENT] Context cancelled, stopping receiveInputEvents")
			return
		default:
			c.mu.Lock()
			reader := c.reader
			connected := c.connected
			c.mu.Unlock()

			if !connected || reader == nil {
				logger.Info("[SSH-CLIENT] Connection lost or reader nil, stopping receiveInputEvents")
				return
			}

			// Read length prefix (4 bytes) with timeout
			lengthBuf := make([]byte, 4)
			logger.Debug("[SSH-CLIENT] Reading length prefix (4 bytes)...")
			
			// Use a timeout to avoid hanging on read
			readChan := make(chan error, 1)
			go func() {
				_, err := io.ReadFull(reader, lengthBuf)
				readChan <- err
			}()

			select {
			case <-ctx.Done():
				logger.Info("[SSH-CLIENT] Context cancelled during length read")
				return
			case err := <-readChan:
				if err != nil {
					if err == io.EOF || err == io.ErrClosedPipe {
						logger.Info("[SSH-CLIENT] Connection closed (EOF/ErrClosedPipe)")
						return // Connection closed
					}
					logger.Errorf("[SSH-CLIENT] Failed to read message length: %v", err)
					continue
				}
			case <-time.After(2 * time.Second):
				logger.Debug("[SSH-CLIENT] Read timeout, checking context")
				continue
			}

			// Decode length
			length := int(lengthBuf[0])<<24 | int(lengthBuf[1])<<16 | int(lengthBuf[2])<<8 | int(lengthBuf[3])
			logger.Debugf("[SSH-CLIENT] Decoded message length: %d bytes (raw bytes: %02x %02x %02x %02x)", 
				length, lengthBuf[0], lengthBuf[1], lengthBuf[2], lengthBuf[3])

			// Check if this looks like text instead of a protocol buffer length
			// Protocol buffer lengths are typically small and positive
			if length <= 0 || length > 4096 {
				// This might be text data - check if the bytes are printable ASCII
				isText := true
				for _, b := range lengthBuf {
					if b < 32 || b > 126 {
						isText = false
						break
					}
				}

				if isText {
					logger.Warnf("[SSH-CLIENT] Detected text data instead of protocol buffer: %q", string(lengthBuf))
					// Append to text buffer
					textBuffer = append(textBuffer, lengthBuf...)
					
					// Read more text until we find a newline or hit a limit
					tempBuf := make([]byte, 1)
					for len(textBuffer) < 1024 {
						n, err := reader.Read(tempBuf)
						if err != nil || n == 0 {
							break
						}
						textBuffer = append(textBuffer, tempBuf[0])
						if tempBuf[0] == '\n' {
							break
						}
					}
					
					// Process the text message
					textMsg := string(textBuffer)
					logger.Infof("[SSH-CLIENT] Server message: %s", strings.TrimSpace(textMsg))
					
					// Check for specific error messages
					if strings.Contains(textMsg, "maximum number of active clients") {
						logger.Error("[SSH-CLIENT] Server rejected connection: max clients reached")
						c.mu.Lock()
						c.connected = false
						c.mu.Unlock()
						return
					}
					
					// Clear text buffer and continue
					textBuffer = textBuffer[:0]
					continue
				} else {
					logger.Errorf("[SSH-CLIENT] Invalid message length: %d (raw bytes: %02x %02x %02x %02x)", 
						length, lengthBuf[0], lengthBuf[1], lengthBuf[2], lengthBuf[3])
					continue
				}
			}

			// Read message data with timeout
			msgBuf := make([]byte, length)
			logger.Debugf("[SSH-CLIENT] Reading message data (%d bytes)...", length)
			
			readChan = make(chan error, 1)
			go func() {
				_, err := io.ReadFull(reader, msgBuf)
				readChan <- err
			}()

			select {
			case <-ctx.Done():
				logger.Info("[SSH-CLIENT] Context cancelled during message read")
				return
			case err := <-readChan:
				if err != nil {
					logger.Errorf("[SSH-CLIENT] Failed to read message data: %v", err)
					continue
				}
			case <-time.After(2 * time.Second):
				logger.Debug("[SSH-CLIENT] Message read timeout, checking context")
				continue
			}

			// Unmarshal input event
			var inputEvent protocol.InputEvent
			logger.Debug("[SSH-CLIENT] Unmarshaling protocol buffer...")
			if err := proto.Unmarshal(msgBuf, &inputEvent); err != nil {
				logger.Errorf("[SSH-CLIENT] Failed to unmarshal input event: %v", err)
				continue
			}

			logger.Debugf("[SSH-CLIENT] Received event: type=%T, timestamp=%d, sourceId=%s",
				inputEvent.Event, inputEvent.Timestamp, inputEvent.SourceId)

			// Call the callback if set
			c.mu.Lock()
			callback := c.onInputEvent
			c.mu.Unlock()

			if callback != nil {
				logger.Debug("[SSH-CLIENT] Calling onInputEvent callback")
				callback(&inputEvent)
			} else {
				logger.Warn("[SSH-CLIENT] No onInputEvent callback set")
			}
		}
	}
}

// monitorConnection monitors the SSH connection health
func (c *SSHClient) monitorConnection(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			connected := c.connected
			c.mu.Unlock()

			if !connected {
				return
			}

			// Try to send a ping/keepalive
			// SSH client has built-in keepalive, but we can add application-level checks here
			logger.Debug("[SSH-CLIENT] Connection health check")
		}
	}
}

// writeMessage writes a protobuf message with length prefix
func writeMessage(w io.Writer, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
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

	return nil
}

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

	return nil
}