package ipc

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog"

	"github.com/bnema/waymon/internal/boundaries/in"
)

// Server handles incoming IPC connections over a Unix socket.
type Server struct {
	mu         sync.Mutex
	listener   net.Listener
	socketPath string
	handler    in.IPCHandler
	wg         sync.WaitGroup
	cancel     context.CancelFunc
	running    bool

	// OnStop is called when a stop command is received.
	// The server will call this callback before shutting down.
	OnStop func()
}

// NewServer creates a new IPC socket server.
func NewServer(handler in.IPCHandler) (*Server, error) {
	socketPath, err := GetSocketPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get socket path: %w", err)
	}

	return &Server{
		socketPath: socketPath,
		handler:    handler,
	}, nil
}

// NewServerWithPath creates a new IPC socket server with a custom socket path.
// This is useful for testing.
func NewServerWithPath(handler in.IPCHandler, socketPath string) *Server {
	return &Server{
		socketPath: socketPath,
		handler:    handler,
	}
}

// Start starts the socket server.
func (s *Server) Start(ctx context.Context) error {
	log := zerolog.Ctx(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	// Remove existing socket file if it exists
	if err := os.RemoveAll(s.socketPath); err != nil {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create socket directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0750); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Create Unix socket listener
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket listener: %w", err)
	}

	// Set socket permissions
	// For server mode (root), allow all users to connect (0666)
	// For user mode, restrict to owner only (0600)
	perms := os.FileMode(0600)
	if os.Geteuid() == 0 {
		perms = 0666
	}
	if err := os.Chmod(s.socketPath, perms); err != nil {
		if closeErr := listener.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("Failed to close listener after chmod error")
		}
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	s.listener = listener
	s.running = true

	serverCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	s.wg.Add(1)
	go s.acceptConnections(serverCtx)

	log.Info().Str("socket", s.socketPath).Msg("IPC socket server started")
	return nil
}

// Stop stops the socket server.
func (s *Server) Stop(ctx context.Context) {
	log := zerolog.Ctx(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	if s.cancel != nil {
		s.cancel()
	}

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close listener")
		}
	}

	s.wg.Wait()

	// Clean up socket file
	if err := os.RemoveAll(s.socketPath); err != nil {
		log.Error().Err(err).Str("socket", s.socketPath).Msg("Failed to remove socket")
	}

	log.Info().Msg("IPC socket server stopped")
}

// SocketPath returns the path to the Unix socket.
func (s *Server) SocketPath() string {
	return s.socketPath
}

// acceptConnections accepts and handles incoming connections.
func (s *Server) acceptConnections(ctx context.Context) {
	log := zerolog.Ctx(ctx)
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					log.Error().Err(err).Msg("Failed to accept connection")
					continue
				}
			}

			s.wg.Add(1)
			go s.handleConnection(ctx, conn)
		}
	}
}

// handleConnection handles a single client connection.
func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	log := zerolog.Ctx(ctx)
	defer s.wg.Done()
	defer func() {
		if err := conn.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close IPC connection")
		}
	}()

	log.Debug().Msg("New IPC connection established")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := ReadMessage(conn)
			if err != nil {
				if err == io.EOF {
					log.Debug().Msg("IPC connection closed by client")
				} else {
					log.Debug().Err(err).Msg("IPC connection read error")
				}
				return
			}

			response := s.handleMessage(ctx, msg)
			if err := WriteMessage(conn, response); err != nil {
				log.Error().Err(err).Msg("Failed to send IPC response")
				return
			}
		}
	}
}

// handleMessage processes a single message and returns a response.
func (s *Server) handleMessage(ctx context.Context, msg *Message) *Message {
	log := zerolog.Ctx(ctx)

	switch msg.Type {
	case MessageTypeSwitch:
		action, err := ParseSwitchRequest(msg)
		if err != nil {
			errMsg, _ := NewErrorMessage(fmt.Sprintf("Invalid switch request: %v", err))
			return errMsg
		}

		log.Debug().Str("action", msg.Type.String()).Msg("Handling switch command")
		resp, err := s.handler.HandleSwitch(ctx, action)
		if err != nil {
			errMsg, _ := NewErrorMessage(err.Error())
			return errMsg
		}

		respMsg, err := NewStatusResponseMessage(resp)
		if err != nil {
			errMsg, _ := NewErrorMessage(fmt.Sprintf("Failed to create response: %v", err))
			return errMsg
		}
		return respMsg

	case MessageTypeStatus:
		log.Debug().Msg("Handling status query")
		resp, err := s.handler.HandleStatus(ctx)
		if err != nil {
			errMsg, _ := NewErrorMessage(err.Error())
			return errMsg
		}

		respMsg, err := NewStatusResponseMessage(resp)
		if err != nil {
			errMsg, _ := NewErrorMessage(fmt.Sprintf("Failed to create response: %v", err))
			return errMsg
		}
		return respMsg

	case MessageTypeRelease:
		log.Debug().Msg("Handling release command")
		resp, err := s.handler.HandleRelease(ctx)
		if err != nil {
			errMsg, _ := NewErrorMessage(err.Error())
			return errMsg
		}

		respMsg, err := NewStatusResponseMessage(resp)
		if err != nil {
			errMsg, _ := NewErrorMessage(fmt.Sprintf("Failed to create response: %v", err))
			return errMsg
		}
		return respMsg

	case MessageTypeConnect:
		slot, err := ParseConnectRequest(msg)
		if err != nil {
			errMsg, _ := NewErrorMessage(fmt.Sprintf("Invalid connect request: %v", err))
			return errMsg
		}

		log.Debug().Int32("slot", slot).Msg("Handling connect command")
		resp, err := s.handler.HandleConnect(ctx, slot)
		if err != nil {
			errMsg, _ := NewErrorMessage(err.Error())
			return errMsg
		}

		respMsg, err := NewStatusResponseMessage(resp)
		if err != nil {
			errMsg, _ := NewErrorMessage(fmt.Sprintf("Failed to create response: %v", err))
			return errMsg
		}
		return respMsg

	case MessageTypeStop:
		log.Info().Msg("Received stop command via IPC")
		if s.OnStop != nil {
			s.OnStop()
		}
		return NewOKMessage()

	default:
		errMsg, _ := NewErrorMessage(fmt.Sprintf("Unknown message type: %s", msg.Type))
		return errMsg
	}
}

// GetSocketPath returns the path for the Unix socket.
func GetSocketPath() (string, error) {
	// For server mode (running as root), use a predictable socket path
	if os.Geteuid() == 0 {
		return "/tmp/waymon.sock", nil
	}

	// For user mode, include username in socket path
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	// Use /tmp/waymon-{username}.sock
	socketPath := filepath.Join("/tmp", fmt.Sprintf("waymon-%s.sock", currentUser.Username))
	return socketPath, nil
}

// String returns a string representation of the message type.
func (mt MessageType) String() string {
	return string(mt)
}
