package ipc

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"sync"

	"github.com/bnema/waymon/internal/logger"
	pb "github.com/bnema/waymon/internal/proto"
	"google.golang.org/protobuf/proto"
)

// SocketServer handles incoming IPC connections
type SocketServer struct {
	mu       sync.Mutex
	listener net.Listener
	socketPath string
	handler  MessageHandler
	wg       sync.WaitGroup
	cancel   context.CancelFunc
	running  bool
}

// MessageHandler defines the interface for handling IPC messages
type MessageHandler interface {
	HandleSwitchCommand(cmd *pb.SwitchCommand) (*pb.IPCMessage, error)
	HandleStatusQuery(query *pb.StatusQuery) (*pb.IPCMessage, error)
}

// NewSocketServer creates a new socket server
func NewSocketServer(handler MessageHandler) (*SocketServer, error) {
	socketPath, err := getSocketPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get socket path: %w", err)
	}

	return &SocketServer{
		socketPath: socketPath,
		handler:    handler,
	}, nil
}

// Start starts the socket server
func (s *SocketServer) Start() error {
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
	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Create Unix socket listener
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket listener: %w", err)
	}

	// Set socket permissions (user only)
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	s.listener = listener
	s.running = true

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	s.wg.Add(1)
	go s.acceptConnections(ctx)

	logger.Infof("IPC socket server started at %s", s.socketPath)
	return nil
}

// Stop stops the socket server
func (s *SocketServer) Stop() {
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
		s.listener.Close()
	}

	s.wg.Wait()

	// Clean up socket file
	os.RemoveAll(s.socketPath)

	logger.Info("IPC socket server stopped")
}

// acceptConnections accepts and handles incoming connections
func (s *SocketServer) acceptConnections(ctx context.Context) {
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
					logger.Errorf("Failed to accept connection: %v", err)
					continue
				}
			}

			s.wg.Add(1)
			go s.handleConnection(ctx, conn)
		}
	}
}

// handleConnection handles a single client connection
func (s *SocketServer) handleConnection(ctx context.Context, conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	logger.Debug("New IPC connection established")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := s.readMessage(conn)
			if err != nil {
				logger.Debugf("Connection closed or read error: %v", err)
				return
			}

			response := s.handleMessage(msg)
			if err := s.writeMessage(conn, response); err != nil {
				logger.Errorf("Failed to send response: %v", err)
				return
			}
		}
	}
}

// handleMessage processes a single message and returns a response
func (s *SocketServer) handleMessage(msg *pb.IPCMessage) *pb.IPCMessage {
	switch msg.Type {
	case pb.IPCMessageType_IPC_MESSAGE_TYPE_SWITCH:
		cmd, err := GetSwitchCommand(msg)
		if err != nil {
			errMsg, _ := NewErrorMessage(fmt.Sprintf("Invalid switch command: %v", err))
			return errMsg
		}
		
		response, err := s.handler.HandleSwitchCommand(cmd)
		if err != nil {
			errMsg, _ := NewErrorMessage(err.Error())
			return errMsg
		}
		return response

	case pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS:
		query, err := GetStatusQuery(msg)
		if err != nil {
			errMsg, _ := NewErrorMessage(fmt.Sprintf("Invalid status query: %v", err))
			return errMsg
		}
		
		response, err := s.handler.HandleStatusQuery(query)
		if err != nil {
			errMsg, _ := NewErrorMessage(err.Error())
			return errMsg
		}
		return response

	default:
		errMsg, _ := NewErrorMessage(fmt.Sprintf("Unknown message type: %s", msg.Type))
		return errMsg
	}
}

// readMessage reads a protobuf message from the connection
func (s *SocketServer) readMessage(conn net.Conn) (*pb.IPCMessage, error) {
	// Read message length (4 bytes, big endian)
	var length uint32
	if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	// Read message data
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, fmt.Errorf("failed to read message data: %w", err)
	}

	// Unmarshal protobuf message
	var msg pb.IPCMessage
	if err := proto.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}

// writeMessage writes a protobuf message to the connection
func (s *SocketServer) writeMessage(conn net.Conn, msg *pb.IPCMessage) error {
	// Marshal protobuf message
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Write message length (4 bytes, big endian)
	length := uint32(len(data))
	if err := binary.Write(conn, binary.BigEndian, length); err != nil {
		return fmt.Errorf("failed to write message length: %w", err)
	}

	// Write message data
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to write message data: %w", err)
	}

	return nil
}

// getSocketPath returns the path for the Unix socket
func getSocketPath() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	// Use /tmp/waymon-{username}.sock
	socketPath := filepath.Join("/tmp", fmt.Sprintf("waymon-%s.sock", currentUser.Username))
	return socketPath, nil
}

// GetSocketPath returns the socket path (for use by clients)
func GetSocketPath() (string, error) {
	return getSocketPath()
}