package network

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/logger"
	Proto "github.com/bnema/waymon/internal/proto"
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
	mu      sync.Mutex
	clients map[string]*sshClient // sessionID -> client

	// Authentication
	pendingAuth map[string]chan bool // fingerprint -> approval channel
	authMu      sync.Mutex

	// Lifecycle
	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	// Event handlers
	OnMouseEvent         EventHandler
	OnClientConnected    func(addr string, publicKey string)
	OnClientDisconnected func(addr string)
	OnAuthRequest        func(addr, publicKey, fingerprint string) bool // Returns approval
}

type sshClient struct {
	session   ssh.Session
	addr      string
	publicKey string
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
				fmt.Fprintf(sess, "Server already has maximum number of active clients\n")
				sess.Exit(1)
				sess.Close()
				return
			}

			// Get client info
			addr := sess.RemoteAddr().String()
			var publicKey string
			if sess.PublicKey() != nil {
				publicKey = gossh.FingerprintSHA256(sess.PublicKey())
			}

			// Create and register client entry
			client := &sshClient{
				session:   sess,
				addr:      addr,
				publicKey: publicKey,
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
			fmt.Fprintf(sess, "Waymon SSH connection established\n")
			fmt.Fprintf(sess, "Public key: %s\n", publicKey)

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
			sess.Close()
		case <-s.stop:
			// Server stopping, close the session
			sess.Close()
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
			var protoEvent Proto.MouseEvent
			if err := proto.Unmarshal(eventBuf, &protoEvent); err != nil {
				continue
			}

			// Call event handler
			if s.OnMouseEvent != nil {
				event := &MouseEvent{MouseEvent: &protoEvent}
				s.OnMouseEvent(event)
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
