// Package server implements the server-side business logic.
// It implements boundaries/in.ServerUseCase and uses boundaries/out ports.
package server

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/bnema/waymon/internal/boundaries/in"
	"github.com/bnema/waymon/internal/boundaries/out"
	"github.com/bnema/waymon/internal/domain"
)

// Compile-time interface check
var _ in.ServerUseCase = (*UseCaseImpl)(nil)

// UseCaseImpl implements the ServerUseCase interface.
// It coordinates client management, input capture, and event routing.
type UseCaseImpl struct {
	// Dependencies (output ports)
	inputCapture out.InputCapturePort
	network      out.NetworkServerPort
	configRepo   out.ConfigRepository

	// Configuration
	config         *domain.Config
	sshHostKeyPath string
	sshAuthKeyPath string

	// State
	mu               sync.RWMutex
	clients          map[string]*domain.Client
	activeClientID   string
	controllingLocal bool
	running          bool

	// Cursor tracking per client
	clientCursors map[string]*domain.CursorState

	// Emergency release tracking
	emergencyReleaseTime time.Time
	emergencyCooldown    time.Duration

	// UI notification callback and throttling
	onActivity      func(level, message string)
	lastActivityLog time.Time
	activityCount   int
}

// NewServerUseCase creates a new server use case with the given dependencies.
// Returns the ServerUseCase interface to ensure consumers depend on the abstraction.
func NewServerUseCase(
	inputCapture out.InputCapturePort,
	network out.NetworkServerPort,
	configRepo out.ConfigRepository,
) in.ServerUseCase {
	return &UseCaseImpl{
		inputCapture:      inputCapture,
		network:           network,
		configRepo:        configRepo,
		clients:           make(map[string]*domain.Client),
		clientCursors:     make(map[string]*domain.CursorState),
		controllingLocal:  true,
		emergencyCooldown: 5 * time.Second,
	}
}

// Start initializes and starts the server.
func (s *UseCaseImpl) Start(ctx context.Context) error {
	log := zerolog.Ctx(ctx)
	log.Debug().Msg("Starting server usecase")

	s.mu.Lock()
	defer s.mu.Unlock()

	// Load configuration
	config, err := s.configRepo.Load(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load config, using defaults")
		config = &domain.Config{
			Server: domain.ServerCfg{
				Port:       52525,
				MaxClients: 5,
			},
		}
	}
	s.config = config
	s.sshHostKeyPath = config.Server.SSHHostKeyPath
	s.sshAuthKeyPath = config.Server.SSHAuthKeysPath

	// Set max clients on network server
	s.network.SetMaxClients(config.Server.MaxClients)

	// Set up input capture callback
	s.inputCapture.SetEventCallback(func(event *domain.InputEvent) {
		s.handleInputEventInternal(ctx, event)
	})

	// Start input capture
	if err := s.inputCapture.Start(ctx); err != nil {
		return err
	}

	s.running = true
	log.Info().Msg("Server usecase started")

	return nil
}

// Stop stops the server and releases all resources.
func (s *UseCaseImpl) Stop(ctx context.Context) error {
	log := zerolog.Ctx(ctx)
	log.Debug().Msg("Stopping server usecase")

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	// Stop input capture
	if s.inputCapture != nil {
		if err := s.inputCapture.Stop(); err != nil {
			log.Error().Err(err).Msg("Failed to stop input capture")
		}
	}

	// Stop network server
	if s.network != nil {
		s.network.Stop()
	}

	// Clear state
	s.clients = make(map[string]*domain.Client)
	s.clientCursors = make(map[string]*domain.CursorState)
	s.activeClientID = ""
	s.controllingLocal = true
	s.running = false

	log.Info().Msg("Server usecase stopped")
	return nil
}

// StartNetworking starts the SSH server for accepting client connections.
func (s *UseCaseImpl) StartNetworking(ctx context.Context) error {
	log := zerolog.Ctx(ctx)
	log.Debug().Msg("Starting server networking")

	// Set up network callbacks
	s.network.SetOnClientConnected(func(addr, publicKey string) {
		s.RegisterClient(ctx, addr, addr, addr)
	})

	s.network.SetOnClientDisconnected(func(addr string) {
		s.UnregisterClient(ctx, addr)
	})

	s.network.SetOnInputEvent(func(event *domain.InputEvent) {
		s.HandleInputEvent(ctx, event)
	})

	// Start the network server
	if err := s.network.Start(ctx); err != nil {
		return err
	}

	log.Info().Int("port", s.network.Port()).Msg("Server networking started")
	return nil
}

// GetNetworkServer returns the network server port for adapters that need it.
func (s *UseCaseImpl) GetNetworkServer() out.NetworkServerPort {
	return s.network
}

// GetSSHHostKeyPath returns the path to the SSH host key.
func (s *UseCaseImpl) GetSSHHostKeyPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sshHostKeyPath
}

// GetSSHAuthKeysPath returns the path to the SSH authorized keys file.
func (s *UseCaseImpl) GetSSHAuthKeysPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sshAuthKeyPath
}

// SetOnActivity sets a callback for activity notifications to the UI.
func (s *UseCaseImpl) SetOnActivity(callback func(level, message string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onActivity = callback
}

// NotifyShutdown sends shutdown notification to all connected clients.
func (s *UseCaseImpl) NotifyShutdown(ctx context.Context) {
	log := zerolog.Ctx(ctx)
	log.Debug().Msg("Notifying clients of shutdown")

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.network == nil {
		log.Warn().Msg("Cannot notify clients of shutdown - network server not available")
		return
	}

	// Create shutdown control event
	shutdownEvent := &domain.InputEvent{
		Timestamp: time.Now().UnixNano(),
		SourceID:  "server",
		Control: &domain.ControlEvent{
			Type: domain.ControlServerShutdown,
		},
	}

	// Send to all connected clients
	for _, client := range s.clients {
		if err := s.network.SendEventToClient(ctx, client.Address, shutdownEvent); err != nil {
			log.Error().Err(err).Str("client", client.Name).Msg("Failed to send shutdown notification")
		} else {
			log.Info().Str("client", client.Name).Msg("Sent shutdown notification")
		}
	}

	// Give clients a moment to process the shutdown notification
	if len(s.clients) > 0 {
		log.Info().Msg("Waiting for clients to process shutdown notification...")
		time.Sleep(1 * time.Second)
	}
}

// MarkEmergencyRelease marks that an emergency release has occurred.
func (s *UseCaseImpl) MarkEmergencyRelease(ctx context.Context) {
	log := zerolog.Ctx(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emergencyReleaseTime = time.Now()
	log.Info().Dur("cooldown", s.emergencyCooldown).Msg("Emergency release marked")
}

// notifyActivity sends an activity notification to the UI if a callback is set.
func (s *UseCaseImpl) notifyActivity(level, message string) {
	if s.onActivity != nil {
		s.onActivity(level, message)
	}
}

// getServerName returns the server hostname.
func (s *UseCaseImpl) getServerName() string {
	serverName, err := os.Hostname()
	if err != nil {
		serverName = "waymon-server"
	}
	return serverName
}

// handleInputEventInternal is the internal handler called from input capture callback.
func (s *UseCaseImpl) handleInputEventInternal(ctx context.Context, event *domain.InputEvent) {
	s.HandleInputEvent(ctx, event)
}
