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
	if privateKeyPath == "" {
		// Default to ~/.ssh/id_rsa or ~/.ssh/id_ed25519
		homeDir, _ := os.UserHomeDir()
		keyPaths := []string{
			filepath.Join(homeDir, ".ssh", "id_ed25519"),
			filepath.Join(homeDir, ".ssh", "id_rsa"),
		}

		for _, path := range keyPaths {
			if _, err := os.Stat(path); err == nil {
				privateKeyPath = path
				break
			}
		}
	}

	return &SSHClient{
		privateKeyPath: privateKeyPath,
	}
}

// Connect establishes an SSH connection to the server
func (c *SSHClient) Connect(ctx context.Context, serverAddr string) error {
	return c.ConnectWithTimeout(ctx, serverAddr, 30*time.Second)
}

// ConnectWithTimeout establishes an SSH connection to the server with a specific timeout
func (c *SSHClient) ConnectWithTimeout(ctx context.Context, serverAddr string, timeout time.Duration) error {

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("already connected")
	}

	// Load private key
	logger.Debugf("Loading SSH private key from: %s", c.privateKeyPath)
	key, err := os.ReadFile(c.privateKeyPath)
	if err != nil {
		logger.Errorf("Failed to read private key from %s: %v", c.privateKeyPath, err)
		return fmt.Errorf("failed to read private key: %w", err)
	}
	logger.Debugf("Private key loaded, size: %d bytes", len(key))

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		logger.Errorf("Failed to parse private key: %v", err)
		return fmt.Errorf("failed to parse private key: %w", err)
	}
	logger.Debug("Private key parsed successfully")

	// Configure SSH client
	config := &ssh.ClientConfig{
		User: "waymon",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // TODO: Implement proper host key checking
		// Don't set a timeout here - let the context handle it
		// Add client version to help with debugging
		ClientVersion: "SSH-2.0-Waymon-Client",
		// Reduce banner timeout to fail faster if server is unresponsive
		BannerCallback: func(message string) error {
			logger.Debugf("SSH server banner: %s", message)
			return nil
		},
	}

	// Log connection attempt details
	logger.Debugf("SSH client config: user=%s, auth_method=publickey, key_path=%s", config.User, c.privateKeyPath)

	// Parse host and port
	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		return fmt.Errorf("invalid server address format: %w", err)
	}

	// If host is localhost, use 127.0.0.1 explicitly to avoid DNS issues
	if host == "localhost" {
		logger.Debugf("Replacing 'localhost' with '127.0.0.1' to avoid DNS delays")
		host = "127.0.0.1"
		serverAddr = net.JoinHostPort(host, port)
	}

	// If host is not an IP, resolve it first to catch DNS issues early
	if net.ParseIP(host) == nil {
		logger.Debugf("Resolving hostname '%s'...", host)
		resolveStart := time.Now()
		addrs, err := net.LookupHost(host)
		resolveDuration := time.Since(resolveStart)
		if err != nil {
			logger.Errorf("Failed to resolve hostname '%s' after %v: %v", host, resolveDuration, err)
			return fmt.Errorf("failed to resolve hostname: %w", err)
		}
		if len(addrs) == 0 {
			return fmt.Errorf("no addresses found for hostname '%s'", host)
		}
		logger.Debugf("Resolved '%s' to %v in %v", host, addrs, resolveDuration)
		// Use the first resolved address
		serverAddr = net.JoinHostPort(addrs[0], port)
		logger.Debugf("Using resolved address: %s", serverAddr)
	}

	logger.Debugf("Attempting SSH dial to %s", serverAddr)

	// First, try a simple TCP connection to verify connectivity
	logger.Debugf("Testing TCP connectivity to %s", serverAddr)
	tcpStart := time.Now()
	tcpConn, err := net.DialTimeout("tcp", serverAddr, 5*time.Second)
	tcpDuration := time.Since(tcpStart)
	if err != nil {
		logger.Errorf("Failed to establish TCP connection to %s after %v: %v", serverAddr, tcpDuration, err)
		return fmt.Errorf("failed to establish TCP connection: %w", err)
	}
	if err := tcpConn.Close(); err != nil {
		logger.Errorf("Failed to close TCP connection: %v", err)
	}
	logger.Debugf("TCP connectivity test successful after %v", tcpDuration)

	// Create a channel to track dial completion
	dialDone := make(chan struct{})
	var dialErr error
	var client *ssh.Client

	// Create a timeout context only for the dial operation
	dialCtx, dialCancel := context.WithTimeout(ctx, timeout)
	defer dialCancel()

	// Use context-aware dialer
	var dialer net.Dialer

	// Perform dial in a goroutine so we can add extra logging
	go func() {
		logger.Debug("Starting SSH dial...")
		logger.Info("Waiting for server to approve connection...")

		// Log start time for debugging
		dialStart := time.Now()

		// Create a TCP connection with timeout context
		conn, err := dialer.DialContext(dialCtx, "tcp", serverAddr)
		if err != nil {
			dialErr = fmt.Errorf("TCP dial failed: %w", err)
			close(dialDone)
			return
		}

		// Wrap the connection with the SSH client
		sshConn, chans, reqs, err := ssh.NewClientConn(conn, serverAddr, config)
		if err != nil {
			if err := conn.Close(); err != nil {
				logger.Errorf("Failed to close connection: %v", err)
			}
			logger.Errorf("SSH handshake failed: %v", err)
			dialErr = fmt.Errorf("SSH handshake failed: %w", err)
			close(dialDone)
			return
		}

		client = ssh.NewClient(sshConn, chans, reqs)
		dialDuration := time.Since(dialStart)
		logger.Debugf("SSH dial completed successfully after %v", dialDuration)

		close(dialDone)
	}()

	// Wait for dial to complete or timeout
	select {
	case <-dialDone:
		if dialErr != nil {
			logger.Errorf("SSH connection failed: %v", dialErr)
			return dialErr
		}
		logger.Debug("SSH dial successful, creating session...")
	case <-dialCtx.Done():
		logger.Error("SSH connection cancelled or timed out")
		return fmt.Errorf("SSH connection cancelled: %w", dialCtx.Err())
	}

	// Create session
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

	// Check for connection status from server
	buf := make([]byte, 512)
	// Set a short timeout for the initial read
	readChan := make(chan int, 1)
	go func() {
		n, _ := reader.Read(buf)
		readChan <- n
	}()

	select {
	case n := <-readChan:
		if n > 0 {
			response := string(buf[:n])
			if strings.Contains(response, "maximum number of active clients") {
				if err := session.Close(); err != nil {
					logger.Errorf("Failed to close SSH session: %v", err)
				}
				if err := client.Close(); err != nil {
					logger.Errorf("Failed to close SSH client: %v", err)
				}
				return fmt.Errorf("connection rejected: server has maximum number of active clients")
			}
			// Wait for either approval or established message
			if !strings.Contains(response, "Waymon SSH connection established") {
				// Keep reading for final status
				n2, _ := reader.Read(buf)
				if n2 > 0 {
					response += string(buf[:n2])
				}
			}
			if !strings.Contains(response, "Waymon SSH connection established") {
				if err := session.Close(); err != nil {
					logger.Errorf("Failed to close SSH session: %v", err)
				}
				if err := client.Close(); err != nil {
					logger.Errorf("Failed to close SSH client: %v", err)
				}
				return fmt.Errorf("connection not established: waiting for server approval")
			}
		}
	case <-time.After(2 * time.Second):
		// No immediate response, assume we're waiting for approval
		if err := session.Close(); err != nil {
			logger.Errorf("Failed to close SSH session: %v", err)
		}
		if err := client.Close(); err != nil {
			logger.Errorf("Failed to close SSH client: %v", err)
		}
		return fmt.Errorf("connection pending: waiting for server approval")
	}

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

// SendMouseEvent sends a mouse event to the server
func (c *SSHClient) SendMouseEvent(event *MouseEvent) error {
	c.mu.Lock()
	writer := c.writer
	connected := c.connected
	c.mu.Unlock()

	if !connected || writer == nil {
		return fmt.Errorf("not connected")
	}

	return writeMessage(writer, event.MouseEvent)
}

// SendMouseBatch sends multiple mouse events
func (c *SSHClient) SendMouseBatch(events []*MouseEvent) error {
	for _, event := range events {
		if err := c.SendMouseEvent(event); err != nil {
			return err
		}
	}
	return nil
}

// SendInputEvent sends an input event to the server (not used in redesigned architecture)
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
			logger.Debugf("[SSH-CLIENT] Decoded message length: %d bytes", length)

			if length <= 0 || length > 4096 {
				logger.Errorf("[SSH-CLIENT] Invalid message length: %d", length)
				continue
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
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("[SSH-CLIENT] Context cancelled, stopping connection monitor")
			return
		case <-ticker.C:
			c.mu.Lock()
			if !c.connected || c.client == nil {
				c.mu.Unlock()
				return
			}

			// Check if connection is still alive
			_, _, err := c.client.SendRequest("keepalive@waymon", true, nil)
			if err != nil {
				c.connected = false
				c.mu.Unlock()
				if err := c.Disconnect(); err != nil {
					logger.Errorf("Failed to disconnect SSH client: %v", err)
				}
				return
			}
			c.mu.Unlock()
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
