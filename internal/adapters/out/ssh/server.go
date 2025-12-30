package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/adapters/out/ssh/proto"
	"github.com/bnema/waymon/internal/domain"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/rs/zerolog"
	gossh "golang.org/x/crypto/ssh"
)

// ServerAdapter implements NetworkServerPort using SSH transport.
type ServerAdapter struct {
	port         int
	bindAddress  string
	hostKeyPath  string
	authKeysPath string
	maxClients   int

	server *ssh.Server
	ctx    context.Context

	mu      sync.RWMutex
	clients map[string]*serverClient // sessionID -> client

	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	// Callbacks
	onClientConnected    func(addr, publicKey string)
	onClientDisconnected func(addr string)
	onAuthRequest        func(addr, publicKey, fingerprint string) bool
	onInputEvent         func(event *domain.InputEvent)
}

// serverClient represents a connected client.
type serverClient struct {
	session   ssh.Session
	addr      string
	publicKey string
	writer    io.Writer
}

// ServerConfig holds configuration for creating a new SSH server.
type ServerConfig struct {
	Port         int
	BindAddress  string
	HostKeyPath  string
	AuthKeysPath string
	MaxClients   int
}

// NewServerAdapter creates a new SSH server adapter.
func NewServerAdapter(cfg ServerConfig) *ServerAdapter {
	return &ServerAdapter{
		port:         cfg.Port,
		bindAddress:  cfg.BindAddress,
		hostKeyPath:  cfg.HostKeyPath,
		authKeysPath: cfg.AuthKeysPath,
		maxClients:   cfg.MaxClients,
		clients:      make(map[string]*serverClient),
		stop:         make(chan struct{}),
	}
}

// Start starts the SSH server.
func (s *ServerAdapter) Start(ctx context.Context) error {
	log := zerolog.Ctx(ctx)
	log.Debug().Int("port", s.port).Msg("starting SSH server")

	address := fmt.Sprintf("%s:%d", s.bindAddress, s.port)

	server, err := wish.NewServer(
		wish.WithAddress(address),
		wish.WithHostKeyPath(s.hostKeyPath),
		wish.WithPublicKeyAuth(s.publicKeyAuth),
		wish.WithMiddleware(
			s.sessionHandler(),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create SSH server: %w", err)
	}

	s.server = server
	s.ctx = ctx

	// Start listening
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		log.Info().Str("address", address).Msg("SSH server listening")
		if err := server.ListenAndServe(); err != nil && err != ssh.ErrServerClosed {
			log.Error().Err(err).Msg("SSH server error")
		}
	}()

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}

// Stop stops the SSH server.
func (s *ServerAdapter) Stop() {
	s.stopOnce.Do(func() {
		close(s.stop)

		if s.server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.server.Shutdown(ctx)
		}

		// Close all active sessions
		s.mu.Lock()
		for _, client := range s.clients {
			_ = client.session.Close()
		}
		s.clients = make(map[string]*serverClient)
		s.mu.Unlock()

		s.wg.Wait()
	})
}

// SendEventToClient sends an input event to a specific client by address.
func (s *ServerAdapter) SendEventToClient(ctx context.Context, clientAddr string, event *domain.InputEvent) error {
	log := zerolog.Ctx(ctx)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, client := range s.clients {
		if client.addr == clientAddr {
			protoEvent := DomainToProto(event)
			if err := WriteMessage(client.writer, protoEvent); err != nil {
				log.Error().Err(err).Str("addr", clientAddr).Msg("failed to send event to client")
				return fmt.Errorf("failed to send event: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("client not found: %s", clientAddr)
}

// SetMaxClients sets the maximum number of concurrent clients.
func (s *ServerAdapter) SetMaxClients(max int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxClients = max
}

// Port returns the port the server is listening on.
func (s *ServerAdapter) Port() int {
	return s.port
}

// SetOnClientConnected sets the callback for when a client connects.
func (s *ServerAdapter) SetOnClientConnected(callback func(addr, publicKey string)) {
	s.onClientConnected = callback
}

// SetOnClientDisconnected sets the callback for when a client disconnects.
func (s *ServerAdapter) SetOnClientDisconnected(callback func(addr string)) {
	s.onClientDisconnected = callback
}

// SetOnAuthRequest sets the callback for authentication requests.
func (s *ServerAdapter) SetOnAuthRequest(callback func(addr, publicKey, fingerprint string) bool) {
	s.onAuthRequest = callback
}

// SetOnInputEvent sets the callback for received input events.
func (s *ServerAdapter) SetOnInputEvent(callback func(event *domain.InputEvent)) {
	s.onInputEvent = callback
}

// publicKeyAuth handles SSH public key authentication.
func (s *ServerAdapter) publicKeyAuth(ctx ssh.Context, key ssh.PublicKey) bool {
	log := zerolog.Ctx(s.ctx)

	var goKey gossh.PublicKey
	if wishKey, ok := key.(gossh.PublicKey); ok {
		goKey = wishKey
	} else {
		parsedKey, err := gossh.ParsePublicKey(key.Marshal())
		if err != nil {
			log.Error().Err(err).Msg("failed to parse public key")
			return false
		}
		goKey = parsedKey
	}

	fingerprint := gossh.FingerprintSHA256(goKey)
	addr := ctx.RemoteAddr().String()

	log.Info().
		Str("addr", addr).
		Str("user", ctx.User()).
		Str("fingerprint", fingerprint).
		Msg("SSH authentication attempt")

	if s.onAuthRequest != nil {
		publicKeyStr := string(gossh.MarshalAuthorizedKey(goKey))
		return s.onAuthRequest(addr, publicKeyStr, fingerprint)
	}

	// Default: allow all
	return true
}

// sessionHandler handles SSH sessions.
func (s *ServerAdapter) sessionHandler() wish.Middleware {
	return func(h ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			log := zerolog.Ctx(s.ctx)
			addr := sess.RemoteAddr().String()

			log.Info().Str("addr", addr).Msg("new SSH session")

			// Check max clients
			s.mu.Lock()
			if s.maxClients > 0 && len(s.clients) >= s.maxClients {
				s.mu.Unlock()
				log.Warn().Str("addr", addr).Msg("max clients reached, rejecting connection")
				_ = sess.Exit(1)
				_ = sess.Close()
				return
			}

			// Get client info
			var publicKey string
			if sess.PublicKey() != nil {
				publicKey = gossh.FingerprintSHA256(sess.PublicKey())
			}

			// Register client
			client := &serverClient{
				session:   sess,
				addr:      addr,
				publicKey: publicKey,
				writer:    sess,
			}
			s.clients[sess.Context().SessionID()] = client
			s.mu.Unlock()

			// Notify connection
			if s.onClientConnected != nil {
				s.onClientConnected(addr, publicKey)
			}

			// Handle disconnection
			defer func() {
				s.mu.Lock()
				delete(s.clients, sess.Context().SessionID())
				s.mu.Unlock()

				if s.onClientDisconnected != nil {
					s.onClientDisconnected(addr)
				}
			}()

			// Handle events
			s.handleEvents(sess)
		}
	}
}

// handleEvents reads and processes events from the SSH session.
func (s *ServerAdapter) handleEvents(sess ssh.Session) {
	log := zerolog.Ctx(s.ctx)
	addr := sess.RemoteAddr().String()

	// Monitor for shutdown
	go func() {
		select {
		case <-s.ctx.Done():
			_ = sess.Close()
		case <-s.stop:
			_ = sess.Close()
		}
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.stop:
			return
		default:
		}

		// Read length prefix
		length, err := ReadLengthPrefix(sess)
		if err != nil {
			if err == io.EOF || err == io.ErrClosedPipe {
				log.Debug().Str("addr", addr).Msg("client disconnected")
			} else {
				log.Error().Err(err).Str("addr", addr).Msg("error reading message")
			}
			return
		}

		// Read message data
		var inputEvent proto.InputEvent
		if err := ReadMessageData(sess, length, &inputEvent); err != nil {
			log.Error().Err(err).Str("addr", addr).Msg("error reading message data")
			continue
		}

		// Handle ping/pong
		if IsPing(&inputEvent) {
			hostname, _ := os.Hostname()
			pong := NewPongEvent(hostname)
			_ = WriteMessage(sess, pong)
			continue
		}

		if IsPong(&inputEvent) {
			log.Debug().Str("addr", addr).Msg("received pong")
			continue
		}

		// Convert and forward event
		if s.onInputEvent != nil {
			domainEvent := ProtoToDomain(&inputEvent)
			s.onInputEvent(domainEvent)
		}
	}
}

// GetConnectedClients returns a list of connected client addresses.
func (s *ServerAdapter) GetConnectedClients() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addrs := make([]string, 0, len(s.clients))
	for _, client := range s.clients {
		addrs = append(addrs, client.addr)
	}
	return addrs
}
