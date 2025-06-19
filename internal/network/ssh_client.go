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
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/protobuf/proto"
)

// SSHClient handles SSH connections to the server
type SSHClient struct {
	client         *ssh.Client
	session        *ssh.Session
	writer         io.Writer
	reader         io.Reader
	bufferedWriter *BufferedWriter

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

	// Get authentication methods - try SSH agent first, then private key files
	var authMethods []ssh.AuthMethod

	// Try SSH agent first
	if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
		conn, err := net.Dial("unix", sshAuthSock)
		if err == nil {
			agentClient := agent.NewClient(conn)
			signers, err := agentClient.Signers()
			if err == nil && len(signers) > 0 {
				logger.Debugf("Using SSH agent with %d key(s)", len(signers))
				authMethods = append(authMethods, ssh.PublicKeys(signers...))
			} else {
				logger.Debugf("SSH agent available but no keys loaded")
			}
			conn.Close()
		} else {
			logger.Debugf("Failed to connect to SSH agent: %v", err)
		}
	} else {
		logger.Debug("SSH_AUTH_SOCK not set, SSH agent not available")
	}

	// If we have a specific private key path configured, use it
	if c.privateKeyPath != "" {
		signer, err := c.loadPrivateKey(c.privateKeyPath)
		if err != nil {
			// If a specific key is configured but fails to load, that's an error
			return fmt.Errorf("failed to load configured private key: %w", err)
		}
		logger.Debugf("Using configured SSH private key: %s", c.privateKeyPath)
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if len(authMethods) == 0 {
		// No SSH agent and no configured key - try default locations
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		// Try standard SSH key locations in order of preference
		defaultPaths := []string{
			filepath.Join(homeDir, ".ssh", "id_ed25519"),
			filepath.Join(homeDir, ".ssh", "id_rsa"),
			filepath.Join(homeDir, ".ssh", "id_ecdsa"),
		}

		for _, path := range defaultPaths {
			if signer, err := c.loadPrivateKeyIfExists(path); err == nil && signer != nil {
				logger.Debugf("Using SSH private key: %s", path)
				authMethods = append(authMethods, ssh.PublicKeys(signer))
				break
			}
		}
	}

	if len(authMethods) == 0 {
		return fmt.Errorf("no SSH authentication methods available. Please start ssh-agent, configure ssh_private_key, or create a key at ~/.ssh/id_ed25519")
	}

	// Get the username
	username := os.Getenv("USER")
	if username == "" {
		username = "waymon"
	}

	// Create SSH client config
	config := &ssh.ClientConfig{
		User: username,
		Auth: authMethods,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			// For now, accept any host key
			// TODO: Implement proper host key verification
			logger.Debugf("Accepting host key for %s: %s", hostname, ssh.FingerprintSHA256(key))
			return nil
		},
		Timeout: 10 * time.Second,
	}

	// Connect to SSH server with TCP keepalive
	conn, err := net.DialTimeout("tcp", serverAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	// Enable TCP keepalive and optimize for low latency
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		// Disable Nagle's algorithm for low latency
		if err := tcpConn.SetNoDelay(true); err != nil {
			logger.Warnf("Failed to disable Nagle's algorithm: %v", err)
		} else {
			logger.Debug("TCP_NODELAY enabled (Nagle's algorithm disabled)")
		}

		// Increase write buffer for better throughput
		if err := tcpConn.SetWriteBuffer(65536); err != nil {
			logger.Warnf("Failed to set TCP write buffer: %v", err)
		} else {
			logger.Debug("TCP write buffer set to 64KB")
		}

		// Enable keepalive for connection monitoring
		if err := tcpConn.SetKeepAlive(true); err != nil {
			logger.Warnf("Failed to enable TCP keepalive: %v", err)
		} else {
			logger.Debug("TCP keepalive enabled")
			// Set keepalive interval to 30 seconds
			if err := tcpConn.SetKeepAlivePeriod(30 * time.Second); err != nil {
				logger.Warnf("Failed to set TCP keepalive period: %v", err)
			} else {
				logger.Debug("TCP keepalive period set to 30 seconds")
			}
		}
	}

	// Create SSH connection over the TCP connection
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, serverAddr, config)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create SSH connection: %w", err)
	}

	// Create SSH client from connection
	client := ssh.NewClient(sshConn, chans, reqs)

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

	// Don't start a shell - we're using raw stdin/stdout for protocol buffers
	// Starting a shell would send shell prompts and other text that interferes with our protocol
	logger.Debug("[SSH-CLIENT] Using raw session without shell for clean protocol communication")

	// Start the session without a command (raw mode)
	// This is necessary to establish the data channels
	go func() {
		if err := session.Run(""); err != nil {
			// This is expected to return an error when session closes
			logger.Debugf("[SSH-CLIENT] Session ended: %v", err)
		}
	}()

	// No handshake messages expected from server anymore
	// Server only sends protocol buffer messages or error text

	c.client = client
	c.session = session
	c.writer = writer
	c.reader = reader
	// Create buffered writer for low-latency batching (1ms delay, 64KB buffer)
	c.bufferedWriter = NewBufferedWriter(writer, 1*time.Millisecond, 65536)
	c.connected = true

	// Start receiving input events from server
	logger.Info("[SSH-CLIENT] Starting receiveInputEvents goroutine")
	go c.receiveInputEvents(ctx)

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

	// Close buffered writer first
	if c.bufferedWriter != nil {
		c.bufferedWriter.Close()
		c.bufferedWriter = nil
	}

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
	bufferedWriter := c.bufferedWriter
	connected := c.connected
	c.mu.Unlock()

	if !connected || bufferedWriter == nil {
		return fmt.Errorf("not connected")
	}

	return writeInputMessage(bufferedWriter, event)
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
	messageCount := 0

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

			logger.Debugf("[SSH-CLIENT] Waiting for message #%d", messageCount+1)

			// Read length prefix (4 bytes) - blocking read
			lengthBuf := make([]byte, 4)

			// Use a goroutine to make the read cancellable
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
			}

			// Decode length
			length := int(lengthBuf[0])<<24 | int(lengthBuf[1])<<16 | int(lengthBuf[2])<<8 | int(lengthBuf[3])

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

			// Read message data
			msgBuf := make([]byte, length)

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
			}

			// Unmarshal input event
			var inputEvent protocol.InputEvent
			if err := proto.Unmarshal(msgBuf, &inputEvent); err != nil {
				logger.Errorf("[SSH-CLIENT] Failed to unmarshal input event: %v", err)
				continue
			}

			messageCount++
			logger.Debugf("[SSH-CLIENT] Successfully received message #%d: type=%T, sourceId=%s",
				messageCount, inputEvent.Event, inputEvent.SourceId)

			// Call the callback if set
			c.mu.Lock()
			callback := c.onInputEvent
			c.mu.Unlock()

			if callback != nil {
				logger.Debugf("[SSH-CLIENT] Calling onInputEvent callback for message #%d", messageCount)
				callback(&inputEvent)
			} else {
				logger.Warn("[SSH-CLIENT] No onInputEvent callback set")
			}
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

// loadPrivateKey loads and parses a private key from the given path
func (c *SSHClient) loadPrivateKey(keyPath string) (ssh.Signer, error) {
	// Handle ~ expansion
	if strings.HasPrefix(keyPath, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		keyPath = filepath.Join(homeDir, keyPath[1:])
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key from %s: %w", keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		// Check if it's encrypted
		if strings.Contains(err.Error(), "encrypted") {
			return nil, fmt.Errorf("SSH key is encrypted - encrypted keys are not supported. Please use ssh-agent or an unencrypted key")
		}
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return signer, nil
}

// loadPrivateKeyIfExists loads a private key if the file exists, returns nil if not
func (c *SSHClient) loadPrivateKeyIfExists(keyPath string) (ssh.Signer, error) {
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return nil, nil // File doesn't exist, not an error
	}

	return c.loadPrivateKey(keyPath)
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
