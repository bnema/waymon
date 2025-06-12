package network

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	Proto "github.com/bnema/waymon/internal/proto"
	"google.golang.org/protobuf/proto"
)

// Server handles incoming mouse event connections
type Server struct {
	port         int
	listener     net.Listener
	client       net.Conn
	mu           sync.Mutex
	connected    chan struct{}
	disconnected chan struct{}
	stop         chan struct{}
	stopOnce     sync.Once
	wg           sync.WaitGroup

	// Event handler
	OnMouseEvent EventHandler
}

// NewServer creates a new server instance
func NewServer(port int) *Server {
	return &Server{
		port:         port,
		connected:    make(chan struct{}, 1),
		disconnected: make(chan struct{}, 1),
		stop:         make(chan struct{}),
	}
}

// Start begins listening for connections
func (s *Server) Start(ctx context.Context) error {
	if s.port < 0 || s.port > 65535 {
		return fmt.Errorf("invalid port: %d", s.port)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	s.listener = listener

	s.wg.Add(1)
	go s.acceptLoop(ctx)

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}

// Stop shuts down the server
func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		close(s.stop)
		if s.listener != nil {
			_ = s.listener.Close()
		}
		s.mu.Lock()
		if s.client != nil {
			_ = s.client.Close()
		}
		s.mu.Unlock()
		s.wg.Wait()
	})
}

// Address returns the server's listening address
func (s *Server) Address() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Port returns the server's port
func (s *Server) Port() int {
	return s.port
}

// Connected returns a channel that signals when a client connects
func (s *Server) Connected() <-chan struct{} {
	return s.connected
}

// Disconnected returns a channel that signals when a client disconnects
func (s *Server) Disconnected() <-chan struct{} {
	return s.disconnected
}

func (s *Server) acceptLoop(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-s.stop:
			return
		case <-ctx.Done():
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				select {
				case <-s.stop:
					return
				default:
					// Log error and continue
					continue
				}
			}

			s.handleConnection(conn)
		}
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	s.mu.Lock()
	if s.client != nil {
		// Already have a client, reject new connection
		s.mu.Unlock()
		_ = conn.Close()
		return
	}
	s.client = conn
	s.mu.Unlock()

	// Notify about connection
	select {
	case s.connected <- struct{}{}:
	default:
	}

	// Handle the connection
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			s.mu.Lock()
			s.client = nil
			s.mu.Unlock()
			_ = conn.Close()

			// Notify about disconnection
			select {
			case s.disconnected <- struct{}{}:
			default:
			}
		}()

		s.handleClientConnection(conn)
	}()
}

func (s *Server) handleClientConnection(conn net.Conn) {
	for {
		select {
		case <-s.stop:
			return
		default:
			// Set read deadline to allow checking stop channel
			_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

			// Read the length prefix (4 bytes)
			lengthBuf := make([]byte, 4)
			_, err := io.ReadFull(conn, lengthBuf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				// Connection error or EOF
				return
			}

			// Decode length
			length := int(lengthBuf[0])<<24 | int(lengthBuf[1])<<16 | int(lengthBuf[2])<<8 | int(lengthBuf[3])
			if length <= 0 || length > 4096 { // Sanity check
				return
			}

			// Read the event data
			eventBuf := make([]byte, length)
			_, err = io.ReadFull(conn, eventBuf)
			if err != nil {
				return
			}

			// Unmarshal the protobuf event
			var protoEvent Proto.MouseEvent
			if err := proto.Unmarshal(eventBuf, &protoEvent); err != nil {
				continue // Skip malformed events
			}

			// Call the event handler if set
			if s.OnMouseEvent != nil {
				event := &MouseEvent{MouseEvent: &protoEvent}
				if err := s.OnMouseEvent(event); err != nil {
					// Log error but continue processing
					continue
				}
			}
		}
	}
}
