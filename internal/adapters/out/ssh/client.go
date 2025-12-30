package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/adapters/out/ssh/proto"
	"github.com/bnema/waymon/internal/domain"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
)

// ClientAdapter implements NetworkClientPort using SSH transport.
type ClientAdapter struct {
	mu        sync.RWMutex
	conn      net.Conn
	sshClient *ssh.Client
	session   *ssh.Session
	stdin     io.WriteCloser
	stdout    io.Reader
	connected bool

	ctx    context.Context
	cancel context.CancelFunc

	onInputEvent   func(event *domain.InputEvent)
	onDisconnected func(err error)
}

// NewClientAdapter creates a new SSH client adapter.
func NewClientAdapter() *ClientAdapter {
	return &ClientAdapter{}
}

// Connect connects to an SSH server.
func (c *ClientAdapter) Connect(ctx context.Context, addr string, privateKeyPath string) error {
	log := zerolog.Ctx(ctx)
	log.Debug().Str("addr", addr).Msg("connecting to SSH server")

	// Load private key
	// Note: privateKeyPath is user-provided via config/CLI - this is intentional
	keyData, err := os.ReadFile(privateKeyPath) //nolint:gosec // G304: Path is user-configured SSH private key location
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Configure SSH client
	config := &ssh.ClientConfig{
		User: "waymon",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		//nolint:gosec // G106: TODO - Implement proper host key verification with known_hosts
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// Connect
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// SSH handshake
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		_ = conn.Close() // Best effort cleanup on handshake failure
		return fmt.Errorf("SSH handshake failed: %w", err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)

	// Open session
	session, err := client.NewSession()
	if err != nil {
		_ = client.Close() // Best effort cleanup on session creation failure
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Get stdin/stdout pipes
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close() // Best effort cleanup
		_ = client.Close()
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = stdin.Close() // Best effort cleanup
		_ = session.Close()
		_ = client.Close()
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start shell
	if err := session.Shell(); err != nil {
		_ = stdin.Close() // Best effort cleanup
		_ = session.Close()
		_ = client.Close()
		return fmt.Errorf("failed to start shell: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.sshClient = client
	c.session = session
	c.stdin = stdin
	c.stdout = stdout
	c.connected = true
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.mu.Unlock()

	// Start reading events
	go c.readEvents()

	log.Info().Str("addr", addr).Msg("connected to SSH server")
	return nil
}

// Disconnect disconnects from the server.
func (c *ClientAdapter) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false

	if c.cancel != nil {
		c.cancel()
	}

	// Close resources in reverse order of acquisition
	// Errors are intentionally ignored during disconnect cleanup
	if c.stdin != nil {
		_ = c.stdin.Close()
	}

	if c.session != nil {
		_ = c.session.Close()
	}

	if c.sshClient != nil {
		_ = c.sshClient.Close()
	}

	if c.conn != nil {
		_ = c.conn.Close()
	}

	return nil
}

// IsConnected returns true if currently connected.
func (c *ClientAdapter) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// SendEvent sends an input event to the server.
func (c *ClientAdapter) SendEvent(_ context.Context, event *domain.InputEvent) error {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return fmt.Errorf("not connected")
	}
	stdin := c.stdin
	c.mu.RUnlock()

	protoEvent := DomainToProto(event)
	return WriteMessage(stdin, protoEvent)
}

// SetOnInputEvent sets the callback for received input events.
func (c *ClientAdapter) SetOnInputEvent(callback func(event *domain.InputEvent)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onInputEvent = callback
}

// SetOnDisconnected sets the callback for disconnection.
func (c *ClientAdapter) SetOnDisconnected(callback func(err error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onDisconnected = callback
}

// readEvents reads events from the server.
func (c *ClientAdapter) readEvents() {
	c.mu.RLock()
	stdout := c.stdout
	ctx := c.ctx
	c.mu.RUnlock()

	log := zerolog.Ctx(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Read message
		var inputEvent proto.InputEvent
		if err := ReadMessage(stdout, &inputEvent); err != nil {
			if err == io.EOF || err == io.ErrClosedPipe {
				log.Debug().Msg("server disconnected")
			} else {
				log.Error().Err(err).Msg("error reading from server")
			}

			c.mu.Lock()
			c.connected = false
			onDisconnected := c.onDisconnected
			c.mu.Unlock()

			if onDisconnected != nil {
				onDisconnected(err)
			}
			return
		}

		// Handle ping
		if IsPing(&inputEvent) {
			c.mu.RLock()
			stdin := c.stdin
			c.mu.RUnlock()

			hostname, _ := os.Hostname()
			pong := NewPongEvent(hostname)
			_ = WriteMessage(stdin, pong)
			continue
		}

		// Handle pong
		if IsPong(&inputEvent) {
			log.Debug().Msg("received pong from server")
			continue
		}

		// Convert and forward
		c.mu.RLock()
		onInputEvent := c.onInputEvent
		c.mu.RUnlock()

		if onInputEvent != nil {
			domainEvent := ProtoToDomain(&inputEvent)
			onInputEvent(domainEvent)
		}
	}
}

// SendPing sends a ping to the server.
func (c *ClientAdapter) SendPing() error {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return fmt.Errorf("not connected")
	}
	stdin := c.stdin
	c.mu.RUnlock()

	hostname, _ := os.Hostname()
	ping := NewPingEvent(hostname)
	return WriteMessage(stdin, ping)
}
