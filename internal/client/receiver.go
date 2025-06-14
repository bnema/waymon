package client

import (
	"context"
	"fmt"
	"sync"
	"os"
	"time"

	"github.com/bnema/waymon/internal/protocol"
	"github.com/bnema/waymon/internal/wayland"
	"github.com/bnema/waymon/internal/network"
	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/logger"
)

// InputReceiver manages receiving and injecting input from the server
type InputReceiver struct {
	mu               sync.RWMutex
	connected        bool
	serverAddress    string
	sshConnection    *network.SSHClient
	inputInjector    wayland.InputInjector
	controlStatus    ControlStatus
	onStatusChange   func(ControlStatus)
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
	// Create Wayland client for input injection  
	waylandClient, err := wayland.NewWaylandClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create wayland client: %w", err)
	}

	inputInjector := waylandClient.NewInputInjector()

	return &InputReceiver{
		serverAddress: serverAddress,
		inputInjector: inputInjector,
		connected:     false,
	}, nil
}

// Connect connects to the server and starts receiving input
func (ir *InputReceiver) Connect(ctx context.Context, privateKeyPath string) error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	if ir.connected {
		return fmt.Errorf("already connected")
	}

	// Initialize input injector (creates uinput devices)
	if err := ir.inputInjector.Connect(); err != nil {
		return fmt.Errorf("failed to initialize input injector: %w", err)
	}

	// Create SSH connection to server
	sshConnection := network.NewSSHClient(privateKeyPath)

	// Connect to server
	if err := sshConnection.Connect(ctx, ir.serverAddress); err != nil {
		ir.inputInjector.Disconnect()
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	ir.sshConnection = sshConnection
	ir.connected = true

	// Set up input event handler
	ir.sshConnection.OnInputEvent(ir.processInputEvent)

	// Send client configuration to server
	if err := ir.sendClientConfiguration(); err != nil {
		logger.Warnf("Failed to send client configuration: %v", err)
		// Don't fail the connection for this
	}

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

	// Disconnect SSH connection
	if ir.sshConnection != nil {
		ir.sshConnection.Disconnect()
		ir.sshConnection = nil
	}

	// Disconnect input injector
	ir.inputInjector.Disconnect()

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
	// Handle control events first
	if controlEvent := event.GetControl(); controlEvent != nil {
		ir.handleControlEvent(controlEvent)
		return
	}

	// Only inject input if we're being controlled
	ir.mu.RLock()
	beingControlled := ir.controlStatus.BeingControlled
	ir.mu.RUnlock()

	if !beingControlled {
		return
	}

	// Inject the input event
	if err := ir.inputInjector.InjectInputEvent(event); err != nil {
		logger.Errorf("Failed to inject input event: %v", err)
	}
}

// handleControlEvent processes control events from the server
func (ir *InputReceiver) handleControlEvent(control *protocol.ControlEvent) {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	switch control.Type {
	case protocol.ControlEvent_REQUEST_CONTROL:
		// Server is requesting to control this client
		ir.controlStatus.BeingControlled = true
		ir.controlStatus.ControllerName = control.TargetId // Server ID/name
		logger.Infof("Control granted to server: %s", control.TargetId)

	case protocol.ControlEvent_RELEASE_CONTROL:
		// Server is releasing control of this client
		ir.controlStatus.BeingControlled = false
		ir.controlStatus.ControllerName = ""
		logger.Info("Control released by server")

	case protocol.ControlEvent_SWITCH_TO_LOCAL:
		// Server switched to local control (we're no longer being controlled)
		ir.controlStatus.BeingControlled = false
		ir.controlStatus.ControllerName = ""
		logger.Info("Server switched to local control")

	default:
		logger.Warnf("Unknown control event type: %v", control.Type)
	}

	// Notify status change
	if ir.onStatusChange != nil {
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
	// Get hostname for client ID
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-client"
	}

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
		CanReceiveKeyboard:   true,
		CanReceiveMouse:      true, 
		CanReceiveScroll:     true,
		WaylandCompositor:    getWaylandCompositor(),
		UinputVersion:        "1.9.0", // ThomasT75/uinput version
	}

	// Create client configuration
	clientConfig := &protocol.ClientConfig{
		ClientId:     hostname,
		ClientName:   hostname,
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
		SourceId:  hostname,
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