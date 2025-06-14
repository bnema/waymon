package client

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/network"
	"github.com/bnema/waymon/internal/protocol"
)

// InputReceiver manages receiving and injecting input from the server
type InputReceiver struct {
	mu             sync.RWMutex
	connected      bool
	serverAddress  string
	sshConnection  *network.SSHClient
	inputBackend   input.InputBackend
	controlStatus  ControlStatus
	onStatusChange func(ControlStatus)
	clientID       string // The client identifier (hostname)

	// Reconnection state
	reconnectEnabled    bool
	reconnectCtx        context.Context
	reconnectCancel     context.CancelFunc
	privateKeyPath      string
	onReconnectStatus   func(status string) // Callback for reconnection status updates
	reconnectInProgress bool                // Prevent multiple concurrent reconnection attempts

	// Health check state
	lastHealthCheckResponse time.Time
	healthCheckTimeout      time.Duration
}

// ControlStatus represents the current control status of the client
type ControlStatus struct {
	BeingControlled bool
	ControllerName  string
	ControllerID    string
	ConnectedAt     int64
}

// NewInputReceiver creates a new input receiver for the client
func NewInputReceiver(serverAddress string) (*InputReceiver, error) {
	// Create client input backend for injection
	backend, err := input.CreateClientBackend()
	if err != nil {
		return nil, fmt.Errorf("failed to create input backend: %w", err)
	}

	// Get hostname for client ID
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-client"
	}

	return &InputReceiver{
		serverAddress:      serverAddress,
		inputBackend:       backend,
		connected:          false,
		clientID:           hostname,
		healthCheckTimeout: 30 * time.Second, // 30 second timeout for health checks
	}, nil
}

// Connect connects to the server and starts receiving input
func (ir *InputReceiver) Connect(ctx context.Context, privateKeyPath string) error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	if ir.connected {
		return fmt.Errorf("already connected")
	}

	// Store private key path for reconnection
	ir.privateKeyPath = privateKeyPath

	// Initialize input backend
	if err := ir.inputBackend.Start(ctx); err != nil {
		return fmt.Errorf("failed to initialize input backend: %w", err)
	}

	// Create SSH connection to server
	sshConnection := network.NewSSHClient(privateKeyPath)

	// Connect to server
	if err := sshConnection.Connect(ctx, ir.serverAddress); err != nil {
		ir.inputBackend.Stop()
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	ir.sshConnection = sshConnection
	ir.connected = true

	// Set up input event handler
	logger.Debug("[CLIENT-RECEIVER] Setting up SSH input event handler")
	ir.sshConnection.OnInputEvent(ir.processInputEvent)

	// Send client configuration to server
	if err := ir.sendClientConfiguration(); err != nil {
		logger.Warnf("Failed to send client configuration: %v", err)
		// Don't fail the connection for this
	}

	// Enable reconnection by default
	ir.enableReconnection(ctx)

	// Note: Input events are received automatically by SSH client

	logger.Infof("Connected to server: %s", ir.serverAddress)
	return nil
}

// Disconnect disconnects from the server
func (ir *InputReceiver) Disconnect() error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	if !ir.connected {
		return nil
	}

	// Disable reconnection first
	ir.reconnectEnabled = false
	if ir.reconnectCancel != nil {
		ir.reconnectCancel()
		ir.reconnectCancel = nil
	}

	// Disconnect SSH connection
	if ir.sshConnection != nil {
		ir.sshConnection.Disconnect()
		ir.sshConnection = nil
	}

	// Stop input backend
	ir.inputBackend.Stop()

	ir.connected = false
	ir.controlStatus = ControlStatus{}

	// Notify status change
	if ir.onStatusChange != nil {
		ir.onStatusChange(ir.controlStatus)
	}

	logger.Info("Disconnected from server")
	return nil
}

// IsConnected returns whether the client is connected to the server
func (ir *InputReceiver) IsConnected() bool {
	ir.mu.RLock()
	defer ir.mu.RUnlock()
	return ir.connected
}

// GetControlStatus returns the current control status
func (ir *InputReceiver) GetControlStatus() ControlStatus {
	ir.mu.RLock()
	defer ir.mu.RUnlock()
	return ir.controlStatus
}

// OnStatusChange sets a callback for when control status changes
func (ir *InputReceiver) OnStatusChange(callback func(ControlStatus)) {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	ir.onStatusChange = callback
}

// receiveInputEvents is no longer needed - input events are handled by SSH client callback

// processInputEvent processes a received input event
func (ir *InputReceiver) processInputEvent(event *protocol.InputEvent) {
	logger.Debugf("[CLIENT-RECEIVER] Processing input event: type=%T, timestamp=%d, sourceId=%s", 
		event.Event, event.Timestamp, event.SourceId)
	
	// Handle control events first
	if controlEvent := event.GetControl(); controlEvent != nil {
		logger.Debugf("[CLIENT-RECEIVER] Event is control event: type=%v", controlEvent.Type)
		ir.handleControlEvent(controlEvent)
		return
	}

	// Only inject input if we're being controlled
	ir.mu.RLock()
	beingControlled := ir.controlStatus.BeingControlled
	controllerName := ir.controlStatus.ControllerName
	ir.mu.RUnlock()

	logger.Debugf("[CLIENT-RECEIVER] Control status: beingControlled=%v, controller=%s", 
		beingControlled, controllerName)

	if !beingControlled {
		logger.Debug("[CLIENT-RECEIVER] Not being controlled, ignoring input event")
		return
	}

	// Inject the input event based on type
	logger.Debugf("[CLIENT-RECEIVER] Injecting event type: %T", event.Event)
	if err := ir.injectEvent(event); err != nil {
		logger.Errorf("[CLIENT-RECEIVER] Failed to inject input event: %v", err)
	} else {
		logger.Debugf("[CLIENT-RECEIVER] Successfully injected event")
	}
}

// handleControlEvent processes control events from the server
func (ir *InputReceiver) handleControlEvent(control *protocol.ControlEvent) {
	logger.Debugf("[CLIENT-RECEIVER] Handling control event: type=%v, targetId=%s", control.Type, control.TargetId)
	
	ir.mu.Lock()
	defer ir.mu.Unlock()

	switch control.Type {
	case protocol.ControlEvent_REQUEST_CONTROL:
		// Server is requesting to control this client
		ir.controlStatus.BeingControlled = true
		ir.controlStatus.ControllerName = control.TargetId // Server ID/name
		ir.controlStatus.ConnectedAt = time.Now().Unix()
		logger.Infof("[CLIENT-RECEIVER] Control granted to server: %s", control.TargetId)

	case protocol.ControlEvent_RELEASE_CONTROL:
		// Server is releasing control of this client
		ir.controlStatus.BeingControlled = false
		ir.controlStatus.ControllerName = ""
		logger.Info("[CLIENT-RECEIVER] Control released by server")

	case protocol.ControlEvent_SWITCH_TO_LOCAL:
		// Server switched to local control (we're no longer being controlled)
		ir.controlStatus.BeingControlled = false
		ir.controlStatus.ControllerName = ""
		logger.Info("[CLIENT-RECEIVER] Server switched to local control")

	case protocol.ControlEvent_SERVER_SHUTDOWN:
		// Server is shutting down gracefully
		logger.Info("[CLIENT-RECEIVER] Server is shutting down - will attempt to reconnect")
		// Mark as disconnected so reconnection logic can take over
		ir.connected = false
		// Clear control status
		ir.controlStatus = ControlStatus{}
		// Don't call Disconnect() here as it will cleanup input injector and disable reconnection
		// Just disconnect the SSH connection
		if ir.sshConnection != nil {
			ir.sshConnection.Disconnect()
			ir.sshConnection = nil
		}
		// Notify that we're starting reconnection
		ir.notifyReconnectStatus("Server shutdown detected - will reconnect shortly...")

	case protocol.ControlEvent_HEALTH_CHECK_PONG:
		// Server responded to health check
		ir.lastHealthCheckResponse = time.Now()
		logger.Info("[CLIENT-RECEIVER] Received health check response from server")

	default:
		logger.Warnf("[CLIENT-RECEIVER] Unknown control event type: %v", control.Type)
	}

	// Notify status change
	if ir.onStatusChange != nil {
		logger.Debug("[CLIENT-RECEIVER] Notifying status change callback")
		ir.onStatusChange(ir.controlStatus)
	}
}

// SendStatusUpdate sends a status update to the server
func (ir *InputReceiver) SendStatusUpdate() error {
	if !ir.connected || ir.sshConnection == nil {
		return fmt.Errorf("not connected")
	}

	// TODO: Implement sending status updates via SSH
	logger.Debug("TODO: Send status update to server")
	return nil
}

// RequestControlRelease requests the server to stop controlling this client
func (ir *InputReceiver) RequestControlRelease() error {
	if !ir.connected || ir.sshConnection == nil {
		return fmt.Errorf("not connected")
	}

	// TODO: Implement control release request via SSH
	logger.Debug("TODO: Request control release from server")
	return nil
}

// sendClientConfiguration sends the client's monitor and capability information to the server
func (ir *InputReceiver) sendClientConfiguration() error {

	// Get display information
	disp, err := display.New()
	if err != nil {
		return fmt.Errorf("failed to initialize display for config: %w", err)
	}
	defer disp.Close()

	monitors := disp.GetMonitors()

	// Convert display monitors to protocol monitors
	protocolMonitors := make([]*protocol.Monitor, len(monitors))
	for i, mon := range monitors {
		protocolMonitors[i] = &protocol.Monitor{
			Name:        mon.Name,
			X:           mon.X,
			Y:           mon.Y,
			Width:       mon.Width,
			Height:      mon.Height,
			Primary:     mon.Primary,
			Scale:       mon.Scale,
			RefreshRate: 60, // Default refresh rate, could be detected in future
		}
	}

	// Create client capabilities
	capabilities := &protocol.ClientCapabilities{
		CanReceiveKeyboard: true,
		CanReceiveMouse:    true,
		CanReceiveScroll:   true,
		WaylandCompositor:  getWaylandCompositor(),
		UinputVersion: "wayland-virtual-input-v0.1.3", // Using Wayland virtual input
	}

	// Create client configuration
	clientConfig := &protocol.ClientConfig{
		ClientId:     ir.clientID,
		ClientName:   ir.clientID,
		Monitors:     protocolMonitors,
		Capabilities: capabilities,
	}

	// Create control event with client config
	controlEvent := &protocol.ControlEvent{
		Type:         protocol.ControlEvent_CLIENT_CONFIG,
		ClientConfig: clientConfig,
	}

	// Create input event containing the control event
	inputEvent := &protocol.InputEvent{
		Event: &protocol.InputEvent_Control{
			Control: controlEvent,
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  ir.clientID,
	}

	// Send via SSH connection
	if ir.sshConnection != nil {
		if err := ir.sshConnection.SendInputEvent(inputEvent); err != nil {
			return fmt.Errorf("failed to send client config: %w", err)
		}
		logger.Infof("Sent client configuration: %d monitors, capabilities: keyboard=%v, mouse=%v",
			len(protocolMonitors), capabilities.CanReceiveKeyboard, capabilities.CanReceiveMouse)
	}

	return nil
}

// getWaylandCompositor attempts to detect the Wayland compositor
func getWaylandCompositor() string {
	// Check common environment variables
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

// enableReconnection enables automatic reconnection when connection is lost
func (ir *InputReceiver) enableReconnection(ctx context.Context) {
	ir.reconnectEnabled = true
	ir.reconnectCtx, ir.reconnectCancel = context.WithCancel(ctx)

	// Start connection monitoring goroutine
	go ir.monitorConnection()
}

// disableReconnection disables automatic reconnection
func (ir *InputReceiver) disableReconnection() {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	ir.reconnectEnabled = false
	if ir.reconnectCancel != nil {
		ir.reconnectCancel()
		ir.reconnectCancel = nil
	}
}

// SetOnReconnectStatus sets a callback for reconnection status updates
func (ir *InputReceiver) SetOnReconnectStatus(callback func(status string)) {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	ir.onReconnectStatus = callback
}

// monitorConnection monitors the connection and triggers reconnection when needed
func (ir *InputReceiver) monitorConnection() {
	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
	defer ticker.Stop()

	// Initialize last health check response time
	ir.mu.Lock()
	ir.lastHealthCheckResponse = time.Now()
	ir.mu.Unlock()

	for {
		select {
		case <-ir.reconnectCtx.Done():
			return
		case <-ticker.C:
			ir.mu.RLock()
			connected := ir.connected
			enabled := ir.reconnectEnabled
			lastHealthResponse := ir.lastHealthCheckResponse
			healthTimeout := ir.healthCheckTimeout
			ir.mu.RUnlock()

			if !enabled {
				return
			}

			if !connected {
				ir.mu.Lock()
				inProgress := ir.reconnectInProgress
				if !inProgress {
					ir.reconnectInProgress = true
					logger.Info("Connection lost - starting reconnection attempts")
					ir.notifyReconnectStatus("Connection lost - attempting to reconnect...")
					// Start reconnection in a goroutine so monitoring continues
					go ir.attemptReconnection()
				}
				ir.mu.Unlock()
				// Wait a bit before next check to avoid spam
				time.Sleep(10 * time.Second)
				continue
			}

			// Send health check ping
			if err := ir.sendHealthCheckPing(); err != nil {
				logger.Warnf("Failed to send health check ping: %v", err)
				// Treat as connection loss
				ir.mu.Lock()
				ir.connected = false
				ir.mu.Unlock()
				continue
			} else {
				logger.Infof("Sent health check ping (last response: %v ago)", time.Since(lastHealthResponse))
			}

			// Check if server has responded to health checks recently
			if time.Since(lastHealthResponse) > healthTimeout {
				logger.Warnf("Health check timeout - server not responding (last response: %v ago)", time.Since(lastHealthResponse))
				ir.mu.Lock()
				ir.connected = false
				ir.mu.Unlock()
				// Will trigger reconnection on next iteration
			}
		}
	}
}

// attemptReconnection attempts to reconnect with exponential backoff
func (ir *InputReceiver) attemptReconnection() {
	// Ensure flag is cleared when done
	defer func() {
		ir.mu.Lock()
		ir.reconnectInProgress = false
		ir.mu.Unlock()
	}()

	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second
	attempt := 1

	for {
		select {
		case <-ir.reconnectCtx.Done():
			logger.Info("Reconnection cancelled")
			return
		default:
		}

		ir.mu.RLock()
		enabled := ir.reconnectEnabled
		ir.mu.RUnlock()

		if !enabled {
			return
		}

		logger.Infof("Reconnection attempt %d to %s", attempt, ir.serverAddress)
		ir.notifyReconnectStatus(fmt.Sprintf("Reconnection attempt %d...", attempt))

		// Create a timeout context for this connection attempt
		connectCtx, cancel := context.WithTimeout(ir.reconnectCtx, 10*time.Second)

		if err := ir.reconnectToServer(connectCtx); err != nil {
			cancel()
			logger.Warnf("Reconnection attempt %d failed: %v", attempt, err)

			// Wait with exponential backoff
			ir.notifyReconnectStatus(fmt.Sprintf("Reconnection failed, retrying in %v...", backoff))

			select {
			case <-ir.reconnectCtx.Done():
				return
			case <-time.After(backoff):
			}

			// Increase backoff, but cap it
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			attempt++
		} else {
			cancel()
			logger.Info("Successfully reconnected to server")
			ir.notifyReconnectStatus("Reconnected successfully")
			// Reset health check response time
			ir.mu.Lock()
			ir.lastHealthCheckResponse = time.Now()
			ir.mu.Unlock()
			return
		}
	}
}

// reconnectToServer performs the actual reconnection
func (ir *InputReceiver) reconnectToServer(ctx context.Context) error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	// Clean up any existing connection
	if ir.sshConnection != nil {
		ir.sshConnection.Disconnect()
		ir.sshConnection = nil
	}

	// Create new SSH connection
	sshConnection := network.NewSSHClient(ir.privateKeyPath)

	// Connect to server
	if err := sshConnection.Connect(ctx, ir.serverAddress); err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	ir.sshConnection = sshConnection
	ir.connected = true

	// Set up input event handler
	logger.Debug("[CLIENT-RECEIVER] Setting up SSH input event handler")
	ir.sshConnection.OnInputEvent(ir.processInputEvent)

	// Send client configuration to server
	if err := ir.sendClientConfiguration(); err != nil {
		logger.Warnf("Failed to send client configuration after reconnect: %v", err)
		// Don't fail the reconnection for this
	}

	return nil
}

// notifyReconnectStatus sends reconnection status updates
func (ir *InputReceiver) notifyReconnectStatus(status string) {
	ir.mu.RLock()
	callback := ir.onReconnectStatus
	ir.mu.RUnlock()

	if callback != nil {
		callback(status)
	}
}

// sendHealthCheckPing sends a health check ping to the server
func (ir *InputReceiver) sendHealthCheckPing() error {
	ir.mu.RLock()
	sshConnection := ir.sshConnection
	ir.mu.RUnlock()

	if sshConnection == nil {
		return fmt.Errorf("SSH connection not available")
	}

	// Create health check ping event
	controlEvent := &protocol.ControlEvent{
		Type: protocol.ControlEvent_HEALTH_CHECK_PING,
	}
	inputEvent := &protocol.InputEvent{
		Event: &protocol.InputEvent_Control{
			Control: controlEvent,
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  ir.clientID,
	}

	// Send via SSH connection
	if err := sshConnection.SendInputEvent(inputEvent); err != nil {
		return fmt.Errorf("failed to send health check ping: %w", err)
	}

	return nil
}

// injectEvent injects an input event using the Wayland virtual input backend
func (ir *InputReceiver) injectEvent(event *protocol.InputEvent) error {
	// Cast to WaylandVirtualInput to access injection methods
	backend, ok := ir.inputBackend.(*input.WaylandVirtualInput)
	if !ok {
		logger.Errorf("[CLIENT-RECEIVER] Input backend is not WaylandVirtualInput, got %T", ir.inputBackend)
		return fmt.Errorf("input backend does not support injection")
	}

	switch e := event.Event.(type) {
	case *protocol.InputEvent_MouseMove:
		logger.Debugf("[CLIENT-RECEIVER] Injecting mouse move: dx=%d, dy=%d", e.MouseMove.Dx, e.MouseMove.Dy)
		return backend.InjectMouseMove(e.MouseMove.Dx, e.MouseMove.Dy)
		
	case *protocol.InputEvent_MouseButton:
		logger.Debugf("[CLIENT-RECEIVER] Injecting mouse button: button=%d, pressed=%v", e.MouseButton.Button, e.MouseButton.Pressed)
		return backend.InjectMouseButton(e.MouseButton.Button, e.MouseButton.Pressed)
		
	case *protocol.InputEvent_MouseScroll:
		logger.Debugf("[CLIENT-RECEIVER] Injecting mouse scroll: dx=%d, dy=%d", e.MouseScroll.Dx, e.MouseScroll.Dy)
		return backend.InjectMouseScroll(e.MouseScroll.Dx, e.MouseScroll.Dy)
		
	case *protocol.InputEvent_Keyboard:
		logger.Debugf("[CLIENT-RECEIVER] Injecting keyboard event: key=%d, pressed=%v", e.Keyboard.Key, e.Keyboard.Pressed)
		return backend.InjectKeyEvent(e.Keyboard.Key, e.Keyboard.Pressed)
		
	case *protocol.InputEvent_MousePosition:
		logger.Debugf("[CLIENT-RECEIVER] Received mouse position event: x=%d, y=%d (not implemented)", 
			e.MousePosition.X, e.MousePosition.Y)
		// TODO: Implement absolute positioning if needed
		return nil
		
	default:
		logger.Errorf("[CLIENT-RECEIVER] Unsupported input event type: %T", event.Event)
		return fmt.Errorf("unsupported input event type: %T", event.Event)
	}
}
