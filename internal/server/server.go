// Package server implements the Waymon server with privilege separation
package server

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"sync"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/network"
)

// Server represents the main server with privilege separation
type Server struct {
	config       *config.Config
	display      *display.Display
	inputHandler input.Handler
	netServer    *network.Server
	
	// Privilege separation
	isPrivileged bool
	actualUser   *user.User
	
	// Synchronization
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new server instance
func New(cfg *config.Config) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	s := &Server{
		config:       cfg,
		ctx:          ctx,
		cancel:       cancel,
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
func (s *Server) Start() error {
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
	go s.runNetworkServer()
	
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

// initNetwork initializes the network server
func (s *Server) initNetwork() error {
	port := s.config.Server.Port
	if s.config.Server.BindAddress != "" {
		port = s.config.Server.Port
	}
	
	s.netServer = network.NewServer(port)
	
	// Set up event handler
	s.netServer.OnMouseEvent = s.handleMouseEvent
	
	return nil
}


// runNetworkServer runs the network server
func (s *Server) runNetworkServer() {
	defer s.wg.Done()
	
	if err := s.netServer.Start(s.ctx); err != nil {
		fmt.Printf("Network server error: %v\n", err)
	}
}

// handleMouseEvent processes incoming mouse events
func (s *Server) handleMouseEvent(event *network.MouseEvent) error {
	if s.inputHandler == nil {
		return fmt.Errorf("input handler not initialized")
	}
	
	// Convert network event to input event and inject
	// This is a placeholder - you'll need to implement the actual conversion
	return nil
}

// Stop stops the server
func (s *Server) Stop() {
	s.cancel()
	
	if s.netServer != nil {
		s.netServer.Stop()
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
	if s.netServer != nil {
		return s.netServer.Port()
	}
	return s.config.Server.Port
}

// GetName returns the server name
func (s *Server) GetName() string {
	return s.config.Server.Name
}