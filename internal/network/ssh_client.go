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
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"
)

// SSHClient handles SSH connections to the server
type SSHClient struct {
	client  *ssh.Client
	session *ssh.Session
	writer  io.Writer

	mu        sync.Mutex
	connected bool

	// SSH key paths
	privateKeyPath string
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Implement proper host key checking
		Timeout:         10 * time.Second,
	}

	// Log connection attempt details
	logger.Debugf("SSH client config: user=%s, timeout=%v, auth_method=publickey, key_path=%s", config.User, config.Timeout, c.privateKeyPath)
	
	// If serverAddr contains "localhost", try to use 127.0.0.1 explicitly
	if strings.Contains(serverAddr, "localhost") {
		originalAddr := serverAddr
		serverAddr = strings.Replace(serverAddr, "localhost", "127.0.0.1", 1)
		logger.Debugf("Replaced 'localhost' with '127.0.0.1' in server address: %s -> %s", originalAddr, serverAddr)
	}
	
	logger.Debugf("Attempting SSH dial to %s", serverAddr)

	// First, try a simple TCP connection to verify connectivity
	logger.Debugf("Testing TCP connectivity to %s", serverAddr)
	tcpConn, err := net.DialTimeout("tcp", serverAddr, 2*time.Second)
	if err != nil {
		logger.Errorf("Failed to establish TCP connection to %s: %v", serverAddr, err)
		return fmt.Errorf("failed to establish TCP connection: %w", err)
	}
	tcpConn.Close()
	logger.Debug("TCP connectivity test successful")

	// Create a channel to track dial completion
	dialDone := make(chan struct{})
	var dialErr error
	var client *ssh.Client

	// Perform dial in a goroutine so we can add extra logging
	go func() {
		logger.Debug("Starting SSH dial...")
		client, dialErr = ssh.Dial("tcp", serverAddr, config)
		close(dialDone)
	}()

	// Wait for dial to complete or timeout
	select {
	case <-dialDone:
		if dialErr != nil {
			logger.Errorf("SSH dial failed: %v", dialErr)
			return fmt.Errorf("failed to connect to SSH server: %w", dialErr)
		}
		logger.Debug("SSH dial successful, creating session...")
	case <-time.After(15 * time.Second):
		logger.Error("SSH dial timed out after 15 seconds (config timeout was 10s)")
		return fmt.Errorf("SSH dial timed out")
	}

	// Create session
	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to create SSH session: %w", err)
	}

	// Get stdin pipe for sending data
	writer, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	// Get stdout to check for rejection messages
	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start the session
	if err := session.Shell(); err != nil {
		session.Close()
		client.Close()
		return fmt.Errorf("failed to start SSH session: %w", err)
	}

	// Check for connection status from server
	buf := make([]byte, 512)
	// Set a short timeout for the initial read
	readChan := make(chan int, 1)
	go func() {
		n, _ := stdout.Read(buf)
		readChan <- n
	}()
	
	select {
	case n := <-readChan:
		if n > 0 {
			response := string(buf[:n])
			if strings.Contains(response, "maximum number of active clients") {
				session.Close()
				client.Close()
				return fmt.Errorf("connection rejected: server has maximum number of active clients")
			}
			// Wait for either approval or established message
			if !strings.Contains(response, "Waymon SSH connection established") {
				// Keep reading for final status
				n2, _ := stdout.Read(buf)
				if n2 > 0 {
					response += string(buf[:n2])
				}
			}
			if !strings.Contains(response, "Waymon SSH connection established") {
				session.Close()
				client.Close()
				return fmt.Errorf("connection not established: waiting for server approval")
			}
		}
	case <-time.After(2 * time.Second):
		// No immediate response, assume we're waiting for approval
		session.Close()
		client.Close()
		return fmt.Errorf("connection pending: waiting for server approval")
	}

	c.client = client
	c.session = session
	c.writer = writer
	c.connected = true

	// Monitor connection
	go c.monitorConnection()

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
		c.session.Close()
		c.session = nil
	}

	if c.client != nil {
		c.client.Close()
		c.client = nil
	}

	c.writer = nil

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

// monitorConnection monitors the SSH connection health
func (c *SSHClient) monitorConnection() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
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
				c.Disconnect()
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
