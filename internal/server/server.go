// Package server implements the Waymon server with privilege separation
package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/network"
	waymonProto "github.com/bnema/waymon/internal/proto"
)

// Server represents the main server
type Server struct {
	config        *config.Config
	display       *display.Display
	inputBackend  input.InputBackend
	sshServer     *network.SSHServer
	clientManager *ClientManager

	// Synchronization
	wg sync.WaitGroup
}

// New creates a new server instance
func New(cfg *config.Config) (*Server, error) {
	logger.Debug("Server.New: Creating server struct")
	s := &Server{
		config: cfg,
	}
	logger.Debug("Server.New: Server struct created")

	return s, nil
}

// Start starts the server with appropriate privilege separation
func (s *Server) Start(ctx context.Context) error {
	logger.Debug("Server.Start: Starting server initialization")

	// Initialize display detection (runs as normal user if possible)
	logger.Debug("Server.Start: Initializing display")
	if err := s.initDisplay(); err != nil {
		return fmt.Errorf("failed to initialize display: %w", err)
	}

	// Initialize input handler (requires root)
	if err := s.initInput(); err != nil {
		return fmt.Errorf("failed to initialize input: %w", err)
	}

	// Initialize client manager (new redesigned architecture)
	logger.Debug("Server.Start: Initializing client manager")
	if err := s.initClientManager(); err != nil {
		return fmt.Errorf("failed to initialize client manager: %w", err)
	}

	// Initialize network (creates SSH server but doesn't start it yet)
	if err := s.initNetwork(); err != nil {
		return fmt.Errorf("failed to initialize network: %w", err)
	}

	return nil
}

// StartNetworking starts the SSH server after auth handlers are set
func (s *Server) StartNetworking(ctx context.Context) error {
	if s.sshServer == nil {
		return fmt.Errorf("SSH server not initialized")
	}

	// Start network server
	s.wg.Add(1)
	go s.runNetworkServer(ctx)

	return nil
}

// initClientManager initializes the client manager for the redesigned architecture
func (s *Server) initClientManager() error {
	logger.Debug("Server.initClientManager: Creating client manager")
	clientManager, err := NewClientManager()
	if err != nil {
		logger.Errorf("Server.initClientManager: Failed to create client manager: %v", err)
		return err
	}
	logger.Debug("Server.initClientManager: Client manager created successfully")
	s.clientManager = clientManager
	return nil
}

// initDisplay initializes display detection, running as normal user if possible
func (s *Server) initDisplay() error {
	logger.Debug("Server.initDisplay: Starting display detection")
	// The display.New() function will automatically use the sudo backend
	// when running with sudo, which handles privilege separation
	disp, err := display.New()
	if err != nil {
		logger.Errorf("Server.initDisplay: Failed to create display: %v", err)
		return err
	}
	logger.Debug("Server.initDisplay: Display created successfully")
	s.display = disp
	return nil
}

// initInput initializes the input handler
func (s *Server) initInput() error {
	// Input handler will use privileged helper if needed
	backend, err := input.CreateBackend()
	if err != nil {
		return err
	}
	s.inputBackend = backend
	return nil
}

// initNetwork initializes the SSH server
func (s *Server) initNetwork() error {
	// Set up SSH paths
	hostKeyPath := expandPath(s.config.Server.SSHHostKeyPath)
	authKeysPath := expandPath(s.config.Server.SSHAuthKeysPath)

	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(hostKeyPath), 0700); err != nil {
		return fmt.Errorf("failed to create host key directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(authKeysPath), 0700); err != nil {
		return fmt.Errorf("failed to create auth keys directory: %w", err)
	}

	// Create SSH server
	s.sshServer = network.NewSSHServer(s.config.Server.Port, hostKeyPath, authKeysPath)
	s.sshServer.SetMaxClients(s.config.Server.MaxClients)

	// Set up event handler
	s.sshServer.OnMouseEvent = s.handleMouseEvent

	return nil
}

// runNetworkServer runs the SSH server
func (s *Server) runNetworkServer(ctx context.Context) {
	defer s.wg.Done()

	if err := s.sshServer.Start(ctx); err != nil {
		logger.Errorf("Network server error: %v", err)
	}
}

// handleMouseEvent processes incoming mouse events
func (s *Server) handleMouseEvent(event *network.MouseEvent) error {
	if s.inputBackend == nil {
		return fmt.Errorf("input backend not initialized")
	}

	// Log event details
	switch event.MouseEvent.Type {
	case waymonProto.EventType_EVENT_TYPE_ENTER:
		logger.Info("Mouse entered from client - taking control")
	case waymonProto.EventType_EVENT_TYPE_LEAVE:
		logger.Info("Mouse left to client - releasing control")
	case waymonProto.EventType_EVENT_TYPE_MOVE:
		logger.Debugf("Mouse move: dx=%.0f, dy=%.0f", event.MouseEvent.X, event.MouseEvent.Y)
	case waymonProto.EventType_EVENT_TYPE_CLICK:
		action := "pressed"
		if !event.MouseEvent.IsPressed {
			action = "released"
		}
		logger.Debugf("Mouse button %s: %s", event.MouseEvent.Button.String(), action)
	case waymonProto.EventType_EVENT_TYPE_SCROLL:
		logger.Debugf("Mouse scroll: %s", event.MouseEvent.Direction.String())
	}

	// Note: Server-side input processing would be implemented here if needed
	// For now, the server primarily captures and forwards events to clients
	return nil
}

// Stop stops the server
func (s *Server) Stop() {
	// Notify clients about shutdown before stopping services
	if s.clientManager != nil {
		s.clientManager.NotifyShutdown()
	}

	if s.sshServer != nil {
		s.sshServer.Stop()
	}

	if s.inputBackend != nil {
		s.inputBackend.Stop()
	}

	if s.display != nil {
		s.display.Close()
	}

	s.wg.Wait()
}

// GetDisplay returns the display manager
func (s *Server) GetDisplay() *display.Display {
	return s.display
}

// GetPort returns the server port
func (s *Server) GetPort() int {
	if s.sshServer != nil {
		return s.sshServer.Port()
	}
	return s.config.Server.Port
}

// GetName returns the server name
func (s *Server) GetName() string {
	return s.config.Server.Name
}

// GetNetworkServer returns the SSH server instance
func (s *Server) GetNetworkServer() *network.SSHServer {
	return s.sshServer
}

// GetClientManager returns the client manager instance
func (s *Server) GetClientManager() *ClientManager {
	return s.clientManager
}

// GetSSHHostKeyPath returns the expanded SSH host key path
func (s *Server) GetSSHHostKeyPath() string {
	return expandPath(s.config.Server.SSHHostKeyPath)
}

// GetSSHAuthKeysPath returns the expanded SSH authorized keys path
func (s *Server) GetSSHAuthKeysPath() string {
	return expandPath(s.config.Server.SSHAuthKeysPath)
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		// Use regular home directory
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
