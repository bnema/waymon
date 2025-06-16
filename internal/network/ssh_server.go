package network

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/protocol"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"
)

// SSHServer handles incoming connections over SSH
type SSHServer struct {
	port         int
	hostKeyPath  string
	authKeysPath string
	maxClients   int
	sshServer    *ssh.Server
	ctx          context.Context

	// Active connections
	mu      sync.RWMutex
	clients map[string]*sshClient // sessionID -> client

	// Authentication
	pendingAuth map[string]chan bool // fingerprint -> approval channel
	authMu      sync.Mutex           //nolint:unused // kept for future authentication features

	// Lifecycle
	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	// Event handlers
	OnInputEvent         func(event *protocol.InputEvent)
	OnClientConnected    func(addr string, publicKey string)
	OnClientDisconnected func(addr string)
	OnAuthRequest        func(addr, publicKey, fingerprint string) bool // Returns approval
}

type sshClient struct {
	session   ssh.Session
	addr      string
	publicKey string
	writer    io.Writer // For sending input events to client
}

// NewSSHServer creates a new SSH-based server
func NewSSHServer(port int, hostKeyPath, authKeysPath string) *SSHServer {
	return &SSHServer{
		port:         port,
		hostKeyPath:  hostKeyPath,
		authKeysPath: authKeysPath,
		maxClients:   1, // Default to single client
		clients:      make(map[string]*sshClient),
		pendingAuth:  make(map[string]chan bool),
		stop:         make(chan struct{}),
	}
}

// SetAuthHandlers sets the authentication callback handlers
func (s *SSHServer) SetAuthHandlers(onAuthRequest func(addr, publicKey, fingerprint string) bool) {
	s.OnAuthRequest = onAuthRequest
}

// Start begins listening for SSH connections
func (s *SSHServer) Start(ctx context.Context) error {
	// Create SSH server
	server, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf(":%d", s.port)),
		wish.WithHostKeyPath(s.hostKeyPath),
		wish.WithPublicKeyAuth(s.publicKeyAuth),
		wish.WithMiddleware(
			s.loggingMiddleware(),
			activeterm.Middleware(),
			s.sessionHandler(),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create SSH server: %w", err)
	}

	s.sshServer = server

	// Start listening
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		logger.Infof("SSH server listening on port %d (address: ':%d')", s.port, s.port)
		if err := server.ListenAndServe(); err != nil && err != ssh.ErrServerClosed {
			logger.Errorf("SSH server error: %v", err)
		}
	}()

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	// Store context for use in session handlers
	s.ctx = ctx

	return nil
}

// SendEventToClient sends an input event to a specific client by address
func (s *SSHServer) SendEventToClient(clientAddr string, event *protocol.InputEvent) error {
	logger.Debugf("[SSH-SERVER] SendEventToClient called: clientAddr=%s, eventType=%T", clientAddr, event.Event)

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find client by address
	for _, client := range s.clients {
		if client.addr == clientAddr {
			logger.Debugf("[SSH-SERVER] Found client for address %s, writing event", clientAddr)

			// Use the same message format as the client expects
			if err := s.writeInputEvent(client.writer, event); err != nil {
				logger.Errorf("[SSH-SERVER] Failed to write event to client %s: %v", clientAddr, err)
				return fmt.Errorf("failed to send event to client: %w", err)
			}

			logger.Debugf("[SSH-SERVER] Successfully sent event to client %s", clientAddr)
			return nil
		}
	}

	logger.Errorf("[SSH-SERVER] Client not found for address: %s", clientAddr)
	return fmt.Errorf("client not found: %s", clientAddr)
}

// Stop shuts down the SSH server
func (s *SSHServer) Stop() {
	s.stopOnce.Do(func() {
		close(s.stop)

		if s.sshServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.sshServer.Shutdown(ctx)
		}

		// Close all active sessions
		s.mu.Lock()
		for _, client := range s.clients {
			_ = client.session.Close()
		}
		s.clients = make(map[string]*sshClient)
		s.mu.Unlock()

		s.wg.Wait()
	})
}

// publicKeyAuth handles SSH public key authentication
func (s *SSHServer) publicKeyAuth(ctx ssh.Context, key ssh.PublicKey) bool {
	// Convert Wish SSH public key to golang.org/x/crypto/ssh public key
	var goKey gossh.PublicKey
	if wishKey, ok := key.(gossh.PublicKey); ok {
		goKey = wishKey
	} else {
		// Parse if needed
		parsedKey, err := gossh.ParsePublicKey(key.Marshal())
		if err != nil {
			logger.Errorf("Failed to parse public key: %v", err)
			return false
		}
		goKey = parsedKey
	}

	fingerprint := gossh.FingerprintSHA256(goKey)
	addr := ctx.RemoteAddr().String()

	logger.Infof("SSH authentication attempt addr=%s user=%s key=%s", addr, ctx.User(), fingerprint)

	// Check if key is already whitelisted
	if config.IsSSHKeyWhitelisted(fingerprint) {
		logger.Infof("SSH key is whitelisted key=%s", fingerprint)
		return true
	}

	// Check if whitelist-only mode is enabled
	cfg := config.Get()
	if !cfg.Server.SSHWhitelistOnly {
		// If not whitelist-only, accept all keys
		logger.Info("Accepting SSH key (whitelist-only mode disabled)")
		return true
	}

	// Key not whitelisted, request approval
	if s.OnAuthRequest != nil {
		logger.Infof("Requesting approval for SSH key=%s addr=%s", fingerprint, addr)
		approved := s.OnAuthRequest(addr, string(gossh.MarshalAuthorizedKey(goKey)), fingerprint)
		if approved {
			// Add to whitelist
			if err := config.AddSSHKeyToWhitelist(fingerprint); err != nil {
				logger.Errorf("Failed to add key to whitelist: %v", err)
			}
			logger.Infof("SSH key approved and added to whitelist key=%s addr=%s", fingerprint, addr)
			return true
		}
		logger.Infof("SSH key denied key=%s addr=%s", fingerprint, addr)
		return false
	}

	// No auth handler, deny by default in whitelist-only mode
	logger.Infof("SSH key denied (no auth handler) key=%s addr=%s", fingerprint, addr)
	return false
}

// loggingMiddleware provides custom logging using our internal logger
func (s *SSHServer) loggingMiddleware() wish.Middleware {
	return func(h ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			// Log the connection details
			logger.Debugf("SSH session started: user=%s addr=%s", sess.User(), sess.RemoteAddr())

			// Call the next handler
			h(sess)

			// Log disconnection
			logger.Debugf("SSH session ended: addr=%s", sess.RemoteAddr())
		}
	}
}

// sessionHandler handles SSH sessions
func (s *SSHServer) sessionHandler() wish.Middleware {
	return func(h ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			// Check if we already have max clients BEFORE accepting the session
			s.mu.Lock()
			if s.maxClients > 0 && len(s.clients) >= s.maxClients {
				s.mu.Unlock()
				// Reject the session immediately
				logger.Infof("Rejecting client - max clients reached addr=%s", sess.RemoteAddr().String())
				if _, err := fmt.Fprintf(sess, "Server already has maximum number of active clients\n"); err != nil {
					logger.Errorf("Failed to write to SSH session: %v", err)
				}
				if err := sess.Exit(1); err != nil {
					logger.Errorf("Failed to exit SSH session: %v", err)
				}
				if err := sess.Close(); err != nil {
					logger.Errorf("Failed to close SSH session: %v", err)
				}
				return
			}

			// Get client info
			addr := sess.RemoteAddr().String()
			var publicKey string
			if sess.PublicKey() != nil {
				publicKey = gossh.FingerprintSHA256(sess.PublicKey())
			}

			// Get session writer for sending input events to client
			writer := sess

			// Create and register client entry
			client := &sshClient{
				session:   sess,
				addr:      addr,
				publicKey: publicKey,
				writer:    writer,
			}
			s.clients[sess.Context().SessionID()] = client
			s.mu.Unlock()

			// Notify connection
			if s.OnClientConnected != nil {
				s.OnClientConnected(addr, publicKey)
			}

			// Handle disconnection
			defer func() {
				s.mu.Lock()
				delete(s.clients, sess.Context().SessionID())
				s.mu.Unlock()

				if s.OnClientDisconnected != nil {
					s.OnClientDisconnected(addr)
				}
			}()

			// Send welcome message
			if _, err := fmt.Fprintf(sess, "Waymon SSH connection established\n"); err != nil {
				logger.Errorf("Failed to write to SSH session: %v", err)
			}
			if _, err := fmt.Fprintf(sess, "Public key: %s\n", publicKey); err != nil {
				logger.Errorf("Failed to write public key to SSH session: %v", err)
			}

			// Handle mouse events with context
			s.handleMouseEvents(s.ctx, sess)
		}
	}
}

// handleMouseEvents reads and processes mouse events from the SSH session
func (s *SSHServer) handleMouseEvents(ctx context.Context, sess ssh.Session) {
	// Create channels for coordinating shutdown
	done := make(chan struct{})
	defer close(done)

	// Channel to receive read results
	type readResult struct {
		data []byte
		err  error
	}
	readCh := make(chan readResult, 1)

	// Start a goroutine to monitor for shutdown
	go func() {
		select {
		case <-ctx.Done():
			// Context cancelled, close the session
			if err := sess.Close(); err != nil {
				logger.Errorf("Failed to close SSH session: %v", err)
			}
		case <-s.stop:
			// Server stopping, close the session
			if err := sess.Close(); err != nil {
				logger.Errorf("Failed to close SSH session: %v", err)
			}
		case <-done:
			// Reading finished normally
		}
	}()

	for {
		// Check if we should stop
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		default:
		}

		// Start async read for length prefix
		go func() {
			lengthBuf := make([]byte, 4)
			_, err := io.ReadFull(sess, lengthBuf)
			select {
			case readCh <- readResult{data: lengthBuf, err: err}:
			case <-ctx.Done():
			}
		}()

		// Wait for read or cancellation
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case result := <-readCh:
			if result.err != nil {
				if result.err == io.EOF || result.err == io.ErrClosedPipe {
					// Connection closed normally
					return
				}
				// Other error
				return
			}

			// Decode length
			lengthBuf := result.data
			length := int(lengthBuf[0])<<24 | int(lengthBuf[1])<<16 | int(lengthBuf[2])<<8 | int(lengthBuf[3])
			if length <= 0 || length > 4096 {
				return
			}

			// Read event data
			eventBuf := make([]byte, length)
			_, err := io.ReadFull(sess, eventBuf)
			if err != nil {
				return
			}

			// Unmarshal protobuf event
			var inputEvent protocol.InputEvent
			if err := proto.Unmarshal(eventBuf, &inputEvent); err != nil {
				logger.Debugf("[SSH-SERVER] Failed to unmarshal input event: %v", err)
				continue
			}

			// Call event handler
			if s.OnInputEvent != nil {
				logger.Debugf("[SSH-SERVER] Forwarding input event: type=%T, sourceId=%s", inputEvent.Event, inputEvent.SourceId)
				s.OnInputEvent(&inputEvent)
			}
		}
	}
}

// SetMaxClients sets the maximum number of concurrent clients
func (s *SSHServer) SetMaxClients(max int) {
	s.maxClients = max
}

// Port returns the server's port
func (s *SSHServer) Port() int {
	return s.port
}

// IsSSHEnabled returns true since this is an SSH server
func (s *SSHServer) IsSSHEnabled() bool {
	return true
}

// SendInputEventToClient sends an input event to a specific client
func (s *SSHServer) SendInputEventToClient(sessionID string, event *protocol.InputEvent) error {
	s.mu.Lock()
	client, exists := s.clients[sessionID]
	s.mu.Unlock()

	if !exists {
		return fmt.Errorf("client not found: %s", sessionID)
	}

	return s.writeInputEvent(client.writer, event)
}

// SendInputEventToAllClients sends an input event to all connected clients
func (s *SSHServer) SendInputEventToAllClients(event *protocol.InputEvent) error {
	s.mu.Lock()
	clients := make([]*sshClient, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}
	s.mu.Unlock()

	var lastErr error
	for _, client := range clients {
		if err := s.writeInputEvent(client.writer, event); err != nil {
			lastErr = err
			logger.Errorf("Failed to send input event to client %s: %v", client.addr, err)
		}
	}

	return lastErr
}

// GetClientSessions returns a map of sessionID -> client address for connected clients
func (s *SSHServer) GetClientSessions() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessions := make(map[string]string)
	for sessionID, client := range s.clients {
		sessions[sessionID] = client.addr
	}
	return sessions
}

// writeInputEvent writes an input event to a client
func (s *SSHServer) writeInputEvent(w io.Writer, event *protocol.InputEvent) error {
	logger.Debugf("[SSH-SERVER] writeInputEvent: marshaling event type=%T", event.Event)

	data, err := proto.Marshal(event)
	if err != nil {
		logger.Errorf("[SSH-SERVER] Failed to marshal input event: %v", err)
		return fmt.Errorf("failed to marshal input event: %w", err)
	}

	// Write length prefix (4 bytes, big-endian)
	length := len(data)
	logger.Debugf("[SSH-SERVER] Writing message: length=%d bytes", length)

	lengthBuf := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}

	if _, err := w.Write(lengthBuf); err != nil {
		logger.Errorf("[SSH-SERVER] Failed to write length prefix: %v", err)
		return fmt.Errorf("failed to write length: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		logger.Errorf("[SSH-SERVER] Failed to write data: %v", err)
		return fmt.Errorf("failed to write data: %w", err)
	}

	logger.Debugf("[SSH-SERVER] Successfully wrote %d bytes to client", length)
	return nil
}
