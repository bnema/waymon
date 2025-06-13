// Package server implements the Waymon server with privilege separation
package server

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/network"
)

// Server represents the main server with privilege separation
type Server struct {
	config       *config.Config
	display      *display.Display
	inputHandler input.Handler
	sshServer    *network.SSHServer
	
	// Privilege separation
	isPrivileged bool
	actualUser   *user.User
	
	// Synchronization
	wg     sync.WaitGroup
}

// New creates a new server instance
func New(cfg *config.Config) (*Server, error) {
	s := &Server{
		config:       cfg,
		isPrivileged: os.Geteuid() == 0,
	}
	
	// If running with sudo, get the actual user info
	if s.isPrivileged {
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
			u, err := user.Lookup(sudoUser)
			if err != nil {
				return nil, fmt.Errorf("failed to lookup sudo user %s: %w", sudoUser, err)
			}
			s.actualUser = u
		}
	}
	
	return s, nil
}

// Start starts the server with appropriate privilege separation
func (s *Server) Start(ctx context.Context) error {
	// Initialize display detection (runs as normal user if possible)
	if err := s.initDisplay(); err != nil {
		return fmt.Errorf("failed to initialize display: %w", err)
	}
	
	// Initialize input handler (requires root)
	if err := s.initInput(); err != nil {
		return fmt.Errorf("failed to initialize input: %w", err)
	}
	
	// Start network server (can run as normal user but we keep it in main process)
	if err := s.initNetwork(); err != nil {
		return fmt.Errorf("failed to initialize network: %w", err)
	}
	
	// Start all components
	s.wg.Add(1)
	go s.runNetworkServer(ctx)
	
	return nil
}

// initDisplay initializes display detection, running as normal user if possible
func (s *Server) initDisplay() error {
	// The display.New() function will automatically use the sudo backend
	// when running with sudo, which handles privilege separation
	disp, err := display.New()
	if err != nil {
		return err
	}
	s.display = disp
	return nil
}

// initInput initializes the input handler (requires root)
func (s *Server) initInput() error {
	if !s.isPrivileged {
		return fmt.Errorf("input handler requires root privileges")
	}
	
	handler, err := input.NewHandler()
	if err != nil {
		return err
	}
	s.inputHandler = handler
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
	if s.inputHandler == nil {
		return fmt.Errorf("input handler not initialized")
	}
	
	// The network.MouseEvent wraps proto.MouseEvent, so we can pass it directly
	return s.inputHandler.ProcessEvent(event.MouseEvent)
}

// Stop stops the server
func (s *Server) Stop() {
	if s.sshServer != nil {
		s.sshServer.Stop()
	}
	
	if s.inputHandler != nil {
		s.inputHandler.Close()
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
		// When running with sudo, use the actual user's home directory
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
			u, err := user.Lookup(sudoUser)
			if err == nil {
				return filepath.Join(u.HomeDir, path[2:])
			}
		}
		// Fall back to regular home directory
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}