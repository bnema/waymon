package network

import (
	"context"
	"fmt"
	"net"
	"sync"
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

	// Handle the connection (placeholder for now)
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

		// Read loop will be implemented when we add event handling
		buf := make([]byte, 1024)
		for {
			_, err := conn.Read(buf)
			if err != nil {
				return
			}
		}
	}()
}
