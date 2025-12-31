// Package client implements the client-side business logic.
// It implements boundaries/in.ClientUseCase and uses boundaries/out ports.
package client

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"

	"github.com/bnema/waymon/internal/boundaries/in"
	"github.com/bnema/waymon/internal/boundaries/out"
	"github.com/bnema/waymon/internal/domain"
)

// Compile-time interface check
var _ in.ClientUseCase = (*UseCaseImpl)(nil)

// UseCaseImpl implements the ClientUseCase interface.
// It manages connection to server and input injection.
type UseCaseImpl struct {
	// Dependencies (output ports)
	inputInjection out.InputInjectionPort
	network        out.NetworkClientPort
	display        out.DisplayPort
	configRepo     out.ConfigRepository
	keyboardLayout out.KeyboardLayoutPort

	// Configuration
	config        *domain.Config
	serverAddress string
	clientID      string

	// State
	mu            sync.RWMutex
	connected     bool
	controlStatus domain.ControlStatus

	// Callbacks
	onControlChanged        func(status domain.ControlStatus)
	onConnectionStateChange func(connected bool, serverName string)

	// Reconnection
	reconnectEnabled bool
	reconnectCtx     context.Context
	reconnectCancel  context.CancelFunc
}

// NewClientUseCase creates a new client use case with the given dependencies.
// Returns the ClientUseCase interface to ensure consumers depend on the abstraction.
func NewClientUseCase(
	inputInjection out.InputInjectionPort,
	network out.NetworkClientPort,
	display out.DisplayPort,
	configRepo out.ConfigRepository,
	keyboardLayout out.KeyboardLayoutPort,
) in.ClientUseCase {
	// Get hostname for client ID
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-client"
	}

	return &UseCaseImpl{
		inputInjection: inputInjection,
		network:        network,
		display:        display,
		configRepo:     configRepo,
		keyboardLayout: keyboardLayout,
		clientID:       hostname,
	}
}

// SetServerAddress sets the server address to connect to.
// This overrides any address from the config file.
func (c *UseCaseImpl) SetServerAddress(addr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serverAddress = addr
}

// Connect connects to the server and starts receiving input.
func (c *UseCaseImpl) Connect(ctx context.Context) error {
	log := zerolog.Ctx(ctx)
	log.Debug().Msg("Connecting to server")

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return domain.ErrAlreadyConnected
	}

	// Load configuration
	config, err := c.configRepo.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	c.config = config

	// Use pre-set server address if available, otherwise use config
	if c.serverAddress == "" {
		c.serverAddress = config.Client.ServerAddress
	}

	// Validate configuration before attempting connection
	if err := c.validateConfig(ctx); err != nil {
		return err
	}

	// Initialize input injection
	if err := c.inputInjection.Start(ctx); err != nil {
		return fmt.Errorf("failed to initialize input injection: %w", err)
	}

	// Set up network event handler
	c.network.SetOnInputEvent(func(event *domain.InputEvent) {
		c.processInputEvent(ctx, event)
	})

	c.network.SetOnDisconnected(func(err error) {
		c.handleDisconnection(ctx, err)
	})

	// Connect to server
	if err := c.network.Connect(ctx, c.serverAddress, config.Client.SSHPrivateKey); err != nil {
		if stopErr := c.inputInjection.Stop(); stopErr != nil {
			log.Error().Err(stopErr).Msg("Failed to stop input injection")
		}
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	c.connected = true

	// Send client configuration to server
	if err := c.sendClientConfiguration(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to send client configuration")
		// Don't fail the connection for this
	}

	// Enable reconnection
	c.reconnectEnabled = true
	c.reconnectCtx, c.reconnectCancel = context.WithCancel(ctx)

	log.Info().Str("server", c.serverAddress).Msg("Connected to server")

	// Notify connection state change
	if c.onConnectionStateChange != nil {
		go c.onConnectionStateChange(true, c.serverAddress)
	}

	return nil
}

// Disconnect disconnects from the server.
func (c *UseCaseImpl) Disconnect(ctx context.Context) error {
	log := zerolog.Ctx(ctx)
	log.Debug().Msg("Disconnecting from server")

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	// Disable reconnection first
	c.reconnectEnabled = false
	if c.reconnectCancel != nil {
		c.reconnectCancel()
		c.reconnectCancel = nil
	}

	// Disconnect from network
	if err := c.network.Disconnect(); err != nil {
		log.Error().Err(err).Msg("Failed to disconnect from network")
	}

	// Stop input injection
	if err := c.inputInjection.Stop(); err != nil {
		log.Error().Err(err).Msg("Failed to stop input injection")
	}

	c.connected = false
	c.controlStatus = domain.ControlStatus{}

	// Notify status change
	if c.onControlChanged != nil {
		go c.onControlChanged(c.controlStatus)
	}

	// Notify connection state change
	if c.onConnectionStateChange != nil {
		go c.onConnectionStateChange(false, "")
	}

	log.Info().Msg("Disconnected from server")
	return nil
}

// IsConnected returns whether the client is connected to the server.
func (c *UseCaseImpl) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetControlStatus returns the current control status.
func (c *UseCaseImpl) GetControlStatus(_ context.Context) domain.ControlStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.controlStatus
}

// SetOnControlChanged sets a callback for when control status changes.
func (c *UseCaseImpl) SetOnControlChanged(callback func(status domain.ControlStatus)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onControlChanged = callback
}

// SetOnConnectionStateChanged sets a callback for when connection state changes.
func (c *UseCaseImpl) SetOnConnectionStateChanged(callback func(connected bool, serverName string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onConnectionStateChange = callback
}

// processInputEvent processes a received input event from the server.
func (c *UseCaseImpl) processInputEvent(ctx context.Context, event *domain.InputEvent) {
	log := zerolog.Ctx(ctx)
	log.Debug().
		Int64("timestamp", event.Timestamp).
		Str("sourceId", event.SourceID).
		Msg("Processing input event")

	// Handle control events first
	if event.Control != nil {
		log.Debug().Str("controlType", event.Control.Type.String()).Msg("Event is control event")
		c.handleControlEvent(ctx, event.Control)
		return
	}

	// Only inject input if we're being controlled
	c.mu.RLock()
	beingControlled := c.controlStatus.BeingControlled
	c.mu.RUnlock()

	if !beingControlled {
		log.Debug().Msg("Not being controlled, ignoring input event")
		return
	}

	// Inject the input event
	if err := c.injectEvent(ctx, event); err != nil {
		log.Error().Err(err).Msg("Failed to inject input event")
	}
}

// handleControlEvent processes control events from the server.
func (c *UseCaseImpl) handleControlEvent(ctx context.Context, control *domain.ControlEvent) {
	log := zerolog.Ctx(ctx)
	log.Debug().Str("type", control.Type.String()).Str("targetId", control.TargetID).Msg("Handling control event")

	c.mu.Lock()
	defer c.mu.Unlock()

	switch control.Type {
	case domain.ControlRequestControl:
		// Server is requesting to control this client
		c.controlStatus.BeingControlled = true
		c.controlStatus.ControllerName = control.TargetID
		log.Info().Str("controller", control.TargetID).Msg("Control granted to server")

		// Enable exclusive capture (keyboard shortcuts inhibitor)
		if err := c.inputInjection.SetExclusiveCapture(ctx, true); err != nil {
			log.Warn().Err(err).Msg("Failed to enable exclusive capture")
		}

	case domain.ControlReleaseControl:
		// Server is releasing control
		previousController := c.controlStatus.ControllerName
		c.controlStatus.BeingControlled = false
		c.controlStatus.ControllerName = ""
		log.Info().Str("previousController", previousController).Msg("Control released by server")

		// Disable exclusive capture
		if err := c.inputInjection.SetExclusiveCapture(ctx, false); err != nil {
			log.Warn().Err(err).Msg("Failed to disable exclusive capture")
		}

	case domain.ControlSwitchToLocal:
		// Server switched to local control
		c.controlStatus.BeingControlled = false
		c.controlStatus.ControllerName = ""
		log.Info().Msg("Server switched to local control")

		// Disable exclusive capture
		if err := c.inputInjection.SetExclusiveCapture(ctx, false); err != nil {
			log.Warn().Err(err).Msg("Failed to disable exclusive capture")
		}

	case domain.ControlServerShutdown:
		// Server is shutting down
		log.Info().Msg("Server is shutting down - will attempt to reconnect")
		c.connected = false
		c.controlStatus = domain.ControlStatus{}

	default:
		log.Warn().Str("type", control.Type.String()).Msg("Unknown control event type")
	}

	// Notify status change
	if c.onControlChanged != nil {
		statusCopy := c.controlStatus
		go c.onControlChanged(statusCopy)
	}
}

// injectEvent injects an input event using the input injection port.
func (c *UseCaseImpl) injectEvent(ctx context.Context, event *domain.InputEvent) error {
	log := zerolog.Ctx(ctx)

	switch {
	case event.MouseMove != nil:
		log.Debug().Float64("dx", event.MouseMove.DX).Float64("dy", event.MouseMove.DY).Msg("Injecting mouse move")
		return c.inputInjection.InjectMouseMove(ctx, event.MouseMove.DX, event.MouseMove.DY)

	case event.MousePosition != nil:
		log.Debug().Int32("x", event.MousePosition.X).Int32("y", event.MousePosition.Y).Msg("Injecting mouse position")
		return c.inputInjection.InjectMousePosition(ctx, event.MousePosition.X, event.MousePosition.Y)

	case event.MouseButton != nil:
		log.Debug().Uint32("button", event.MouseButton.Button).Bool("pressed", event.MouseButton.Pressed).Msg("Injecting mouse button")
		return c.inputInjection.InjectMouseButton(ctx, event.MouseButton.Button, event.MouseButton.Pressed)

	case event.MouseScroll != nil:
		log.Debug().Float64("dx", event.MouseScroll.DX).Float64("dy", event.MouseScroll.DY).Msg("Injecting mouse scroll")
		return c.inputInjection.InjectMouseScroll(ctx, event.MouseScroll.DX, event.MouseScroll.DY, event.MouseScroll.Type)

	case event.Keyboard != nil:
		// Use semantic character injection if a character is provided
		if event.Keyboard.Character != nil {
			log.Debug().
				Int32("char", *event.Keyboard.Character).
				Bool("pressed", event.Keyboard.Pressed).
				Msg("Injecting keyboard character")
			return c.inputInjection.InjectCharacter(ctx, *event.Keyboard.Character, event.Keyboard.Pressed)
		}
		// Fall back to raw keycode injection
		log.Debug().Uint32("key", event.Keyboard.Key).Bool("pressed", event.Keyboard.Pressed).Msg("Injecting keyboard event")
		return c.inputInjection.InjectKeyEvent(ctx, event.Keyboard.Key, event.Keyboard.Pressed, event.Keyboard.Modifiers)

	default:
		return domain.ErrInvalidEvent
	}
}

// sendClientConfiguration sends the client's configuration to the server.
func (c *UseCaseImpl) sendClientConfiguration(ctx context.Context) error {
	log := zerolog.Ctx(ctx)

	// Get monitor information
	monitors, err := c.display.GetMonitors(ctx)
	if err != nil {
		return fmt.Errorf("failed to get monitors: %w", err)
	}

	// Detect keyboard layout
	detectedLayout := domain.LayoutUS // Default
	if c.keyboardLayout != nil {
		detectedLayout = c.keyboardLayout.DetectLayout(ctx)
		// Set the layout on the input injection port
		if err := c.inputInjection.SetKeyboardLayout(detectedLayout); err != nil {
			log.Warn().Err(err).Msg("failed to set keyboard layout on injector")
		}
	}

	// Create client capabilities
	capabilities := &domain.ClientCapabilities{
		CanReceiveKeyboard: true,
		CanReceiveMouse:    true,
		CanReceiveScroll:   true,
		WaylandCompositor:  getWaylandCompositor(),
		UInputVersion:      "wayland-virtual-input",
		KeyboardLayout:     detectedLayout,
	}

	// Create client configuration
	clientConfig := &domain.ClientConfig{
		ClientID:     c.clientID,
		ClientName:   c.clientID,
		Monitors:     monitors,
		Capabilities: capabilities,
	}

	// Create control event with client config
	configEvent := &domain.InputEvent{
		Timestamp: time.Now().UnixNano(),
		SourceID:  c.clientID,
		Control: &domain.ControlEvent{
			Type:         domain.ControlClientConfig,
			ClientConfig: clientConfig,
		},
	}

	// Send via network
	if err := c.network.SendEvent(ctx, configEvent); err != nil {
		return fmt.Errorf("failed to send client config: %w", err)
	}

	log.Info().Int("monitors", len(monitors)).Msg("Sent client configuration")
	return nil
}

// handleDisconnection handles unexpected disconnections.
func (c *UseCaseImpl) handleDisconnection(ctx context.Context, err error) {
	log := zerolog.Ctx(ctx)
	log.Warn().Err(err).Msg("Connection lost")

	c.mu.Lock()
	wasConnected := c.connected
	c.connected = false
	c.controlStatus = domain.ControlStatus{}
	reconnectEnabled := c.reconnectEnabled
	c.mu.Unlock()

	// Notify status change
	if c.onControlChanged != nil {
		go c.onControlChanged(domain.ControlStatus{})
	}

	// Notify connection state change
	if c.onConnectionStateChange != nil && wasConnected {
		go c.onConnectionStateChange(false, "")
	}

	// Attempt reconnection if enabled
	if reconnectEnabled {
		go c.attemptReconnection(ctx)
	}
}

// attemptReconnection attempts to reconnect with exponential backoff.
func (c *UseCaseImpl) attemptReconnection(ctx context.Context) {
	log := zerolog.Ctx(ctx)

	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second
	attempt := 1

	for {
		select {
		case <-c.reconnectCtx.Done():
			log.Info().Msg("Reconnection cancelled")
			return
		default:
		}

		c.mu.RLock()
		enabled := c.reconnectEnabled
		c.mu.RUnlock()

		if !enabled {
			return
		}

		log.Info().Int("attempt", attempt).Str("server", c.serverAddress).Msg("Reconnection attempt")

		// Create timeout context for this attempt
		connectCtx, cancel := context.WithTimeout(c.reconnectCtx, 10*time.Second)

		if err := c.reconnectToServer(connectCtx); err != nil {
			cancel()
			log.Warn().Err(err).Int("attempt", attempt).Msg("Reconnection attempt failed")

			select {
			case <-c.reconnectCtx.Done():
				return
			case <-time.After(backoff):
			}

			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			attempt++
		} else {
			cancel()
			log.Info().Msg("Successfully reconnected to server")
			return
		}
	}
}

// reconnectToServer performs the actual reconnection.
func (c *UseCaseImpl) reconnectToServer(ctx context.Context) error {
	log := zerolog.Ctx(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Disconnect any existing connection
	if err := c.network.Disconnect(); err != nil {
		log.Warn().Err(err).Msg("Error disconnecting during reconnect")
	}

	// Connect to server
	if err := c.network.Connect(ctx, c.serverAddress, c.config.Client.SSHPrivateKey); err != nil {
		return err
	}

	c.connected = true

	// Set up event handlers again
	c.network.SetOnInputEvent(func(event *domain.InputEvent) {
		c.processInputEvent(ctx, event)
	})

	c.network.SetOnDisconnected(func(err error) {
		c.handleDisconnection(ctx, err)
	})

	// Send client configuration
	if err := c.sendClientConfiguration(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to send client configuration after reconnect")
	}

	// Notify connection state change
	if c.onConnectionStateChange != nil {
		go c.onConnectionStateChange(true, c.serverAddress)
	}

	return nil
}

// getWaylandCompositor attempts to detect the Wayland compositor.
func getWaylandCompositor() string {
	if compositor := os.Getenv("XDG_CURRENT_DESKTOP"); compositor != "" {
		return compositor
	}
	if compositor := os.Getenv("WAYLAND_DISPLAY"); compositor != "" {
		return "wayland-" + compositor
	}
	if compositor := os.Getenv("DESKTOP_SESSION"); compositor != "" {
		return compositor
	}
	return "unknown"
}

// validateConfig validates that all required configuration is present and valid.
func (c *UseCaseImpl) validateConfig(ctx context.Context) error {
	log := zerolog.Ctx(ctx)

	// Validate server address
	if c.serverAddress == "" {
		return domain.ErrConfigServerAddrEmpty
	}

	// Validate SSH private key path is set
	keyPath := c.config.Client.SSHPrivateKey
	if keyPath == "" {
		return fmt.Errorf("%w: run 'waymon config init' or set client.ssh_private_key in config",
			domain.ErrConfigSSHKeyNotSet)
	}

	// Check key file exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", domain.ErrConfigSSHKeyNotFound, keyPath)
	}

	// Read and validate key
	keyData, err := os.ReadFile(keyPath) //nolint:gosec // G304: Path is user-configured
	if err != nil {
		return fmt.Errorf("cannot read SSH private key %s: %w", keyPath, err)
	}

	// Try to parse the key
	_, err = ssh.ParsePrivateKey(keyData)
	if err != nil {
		// Check if it's a passphrase error
		if _, ok := err.(*ssh.PassphraseMissingError); ok {
			return fmt.Errorf("%w: %s", domain.ErrConfigSSHKeyEncrypted, keyPath)
		}
		return fmt.Errorf("%w at %s: %v", domain.ErrConfigSSHKeyInvalid, keyPath, err)
	}

	log.Debug().Str("sshKey", keyPath).Msg("configuration validated")
	return nil
}
