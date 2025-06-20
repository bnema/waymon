// Package client provides the client-side implementation for Waymon's mouse sharing functionality.
// It handles receiving input events from the server and injecting them into the local system.
package client

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/network"
	"github.com/bnema/waymon/internal/protocol"
)

// Modifier bit constants for keyboard events
const (
	ModifierShift = 1 << 0 // Shift modifier bit
	ModifierCaps  = 1 << 1 // Caps lock modifier bit
	ModifierCtrl  = 1 << 2 // Control modifier bit
	ModifierAlt   = 1 << 3 // Alt modifier bit
	ModifierMeta  = 1 << 6 // Meta/Super modifier bit
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

	// Hotkey handling state - disabled for now
	// lastHotkeyPress  time.Time
	// hotkeyDebounceMs int64 // Minimum time between hotkey presses in milliseconds
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
		serverAddress: serverAddress,
		inputBackend:  backend,
		connected:     false,
		clientID:      hostname,
		// removed health check timeout
		// hotkeyDebounceMs: 500, // 500ms debounce for hotkey presses - disabled
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
		if err := ir.inputBackend.Stop(); err != nil {
			logger.Errorf("Failed to stop input backend: %v", err)
		}
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
		if err := ir.sshConnection.Disconnect(); err != nil {
			logger.Errorf("Failed to disconnect SSH connection: %v", err)
		}
		ir.sshConnection = nil
	}

	// Stop input backend
	if err := ir.inputBackend.Stop(); err != nil {
		logger.Errorf("Failed to stop input backend: %v", err)
	}

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

	// Hotkey switching disabled for now
	// TODO: Re-enable when hotkey handling is improved

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

		// Show notification to user
		logger.Infof("🖥️  %s is now controlling your system", control.TargetId)

	case protocol.ControlEvent_RELEASE_CONTROL:
		// Server is releasing control of this client
		previousController := ir.controlStatus.ControllerName
		ir.controlStatus.BeingControlled = false
		ir.controlStatus.ControllerName = ""
		logger.Info("[CLIENT-RECEIVER] Control released by server")

		// Show notification to user
		if previousController != "" {
			logger.Infof("✅ %s has stopped controlling your system", previousController)
		} else {
			logger.Info("✅ Control released - your system is now idle")
		}

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
			if err := ir.sshConnection.Disconnect(); err != nil {
				logger.Errorf("Failed to disconnect SSH connection: %v", err)
			}
			ir.sshConnection = nil
		}
		// Notify that we're starting reconnection
		ir.notifyReconnectStatus("Server shutdown detected - will reconnect shortly...")

	default:
		logger.Warnf("[CLIENT-RECEIVER] Unknown control event type: %v", control.Type)
	}

	// Notify status change
	if ir.onStatusChange != nil {
		logger.Debug("[CLIENT-RECEIVER] Notifying status change callback")
		// Make a copy of the status to avoid any potential race conditions
		statusCopy := ir.controlStatus
		logger.Debugf("[CLIENT-RECEIVER] Status to send: BeingControlled=%v, Controller=%s",
			statusCopy.BeingControlled, statusCopy.ControllerName)
		
		// Use goroutine to prevent potential deadlocks with UI thread
		// The copy ensures we don't have race conditions
		go func(status ControlStatus) {
			logger.Debugf("[CLIENT-RECEIVER] Sending status update to UI: BeingControlled=%v, Controller=%s",
				status.BeingControlled, status.ControllerName)
			ir.onStatusChange(status)
		}(statusCopy)
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

// RequestControlRelease sends a control release request to the server
func (ir *InputReceiver) RequestControlRelease() error {
	ir.mu.RLock()
	sshConnection := ir.sshConnection
	clientID := ir.clientID
	ir.mu.RUnlock()

	if sshConnection == nil {
		return fmt.Errorf("SSH connection not available")
	}

	// Create control release event
	controlEvent := &protocol.ControlEvent{
		Type: protocol.ControlEvent_RELEASE_CONTROL,
	}
	inputEvent := &protocol.InputEvent{
		Event: &protocol.InputEvent_Control{
			Control: controlEvent,
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  clientID,
	}

	// Send via SSH connection
	if err := sshConnection.SendInputEvent(inputEvent); err != nil {
		return fmt.Errorf("failed to send control release request: %w", err)
	}

	// Don't update local control state immediately - wait for server confirmation
	// This ensures the UI shows "Requested control release" until the server responds
	
	// The server will send back a RELEASE_CONTROL event which will update our status
	// through handleControlEvent

	logger.Info("[CLIENT-RECEIVER] Control release request sent to server")
	return nil
}

// isControlSwitchHotkey checks if the keyboard event is the switch hotkey based on configuration
// TODO: Re-enable when hotkey handling is improved
/*
func (ir *InputReceiver) isControlSwitchHotkey(keyEvent *protocol.KeyboardEvent) bool {
	// Only trigger on key press, not release
	if !keyEvent.Pressed {
		return false
	}

	// Debounce hotkey presses to prevent rapid switching
	now := time.Now()
	timeSinceLastPress := now.Sub(ir.lastHotkeyPress).Milliseconds()
	if timeSinceLastPress < ir.hotkeyDebounceMs {
		logger.Debugf("[CLIENT-RECEIVER] Hotkey debounced - ignoring press (last press %dms ago)", timeSinceLastPress)
		return false
	}

	// Get configuration
	cfg := config.Get()

	// Parse the configured hotkey
	expectedKey := ir.parseHotkeyKey(cfg.Client.HotkeyKey)
	expectedModifiers := ir.parseHotkeyModifiers(cfg.Client.HotkeyModifier)

	if expectedKey == 0 {
		logger.Warnf("[CLIENT-RECEIVER] Invalid hotkey key configuration: '%s', falling back to 's'", cfg.Client.HotkeyKey)
		expectedKey = 31 // Default to 's' key
	}

	if expectedModifiers == 0 {
		logger.Warnf("[CLIENT-RECEIVER] Invalid hotkey modifier configuration: '%s', falling back to 'ctrl+alt'", cfg.Client.HotkeyModifier)
		expectedModifiers = ModifierCtrl | ModifierAlt // Default to Ctrl+Alt
	}

	// Check if this is the configured key
	if keyEvent.Key != expectedKey {
		return false
	}

	// Check if the required modifiers are active
	if (keyEvent.Modifiers & expectedModifiers) == expectedModifiers {
		// Update last hotkey press time
		ir.lastHotkeyPress = now

		logger.Debugf("[CLIENT-RECEIVER] Switch hotkey detected")
		return true
	}

	return false
}

// parseHotkeyKey converts a key name to its key code
func (ir *InputReceiver) parseHotkeyKey(keyName string) uint32 {
	// Key codes from wayland-virtual-input-go
	keyMap := map[string]uint32{
		"a": 30, "b": 48, "c": 46, "d": 32, "e": 18, "f": 33, "g": 34, "h": 35,
		"i": 23, "j": 36, "k": 37, "l": 38, "m": 50, "n": 49, "o": 24, "p": 25,
		"q": 16, "r": 19, "s": 31, "t": 20, "u": 22, "v": 47, "w": 17, "x": 45,
		"y": 21, "z": 44,
		"1": 2, "2": 3, "3": 4, "4": 5, "5": 6, "6": 7, "7": 8, "8": 9, "9": 10, "0": 11,
		"space": 57, "enter": 28, "tab": 15, "backspace": 14, "esc": 1,
	}

	return keyMap[strings.ToLower(keyName)]
}

// parseHotkeyModifiers converts modifier names to a bitmask
func (ir *InputReceiver) parseHotkeyModifiers(modifierString string) uint32 {

	var modifiers uint32
	parts := strings.Split(strings.ToLower(modifierString), "+")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch part {
		case "ctrl", "control":
			modifiers |= ModifierCtrl
		case "alt":
			modifiers |= ModifierAlt
		case "shift":
			modifiers |= ModifierShift
		case "meta", "super", "cmd":
			modifiers |= ModifierMeta
		case "caps":
			modifiers |= ModifierCaps
		}
	}

	return modifiers
}
*/

// sendClientConfiguration sends the client's monitor and capability information to the server
func (ir *InputReceiver) sendClientConfiguration() error {

	// Get display information
	disp, err := display.New()
	if err != nil {
		return fmt.Errorf("failed to initialize display for config: %w", err)
	}
	defer func() {
		if err := disp.Close(); err != nil {
			logger.Errorf("Failed to close display: %v", err)
		}
	}()

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
		UinputVersion:      "wayland-virtual-input-v0.1.3", // Using Wayland virtual input
	}

	// Create client configuration
	clientConfig := &protocol.ClientConfig{
		ClientId:       ir.clientID,
		ClientName:     ir.clientID,
		Monitors:       protocolMonitors,
		Capabilities:   capabilities,
		KeyboardLayout: getKeyboardLayout(),
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
		logger.Infof("Sent client configuration: %d monitors, layout=%s, capabilities: keyboard=%v, mouse=%v",
			len(protocolMonitors), clientConfig.KeyboardLayout, capabilities.CanReceiveKeyboard, capabilities.CanReceiveMouse)
	}

	return nil
}

// getKeyboardLayout attempts to detect the current keyboard layout
func getKeyboardLayout() string {
	// Try to get layout from various sources
	
	// 1. Try localectl (systemd)
	if output, err := exec.Command("localectl", "status").Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "X11 Layout:") || strings.Contains(line, "VC Keymap:") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					return parts[2]
				}
			}
		}
	}
	
	// 2. Try setxkbmap (X11/XWayland)
	if output, err := exec.Command("setxkbmap", "-query").Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "layout:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					return parts[1]
				}
			}
		}
	}
	
	// 3. Try gsettings (GNOME)
	if output, err := exec.Command("gsettings", "get", "org.gnome.desktop.input-sources", "sources").Output(); err == nil {
		// Parse GNOME format: [('xkb', 'us'), ('xkb', 'fr')]
		if strings.Contains(string(output), "'xkb'") {
			re := regexp.MustCompile(`'xkb',\s*'(\w+)'`)
			if matches := re.FindStringSubmatch(string(output)); len(matches) > 1 {
				return matches[1]
			}
		}
	}
	
	// 4. Check environment variable
	if layout := os.Getenv("XKB_DEFAULT_LAYOUT"); layout != "" {
		return layout
	}
	
	// Default to US layout
	logger.Warn("Could not detect keyboard layout, defaulting to 'us'")
	return "us"
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

// SetOnReconnectStatus sets a callback for reconnection status updates
func (ir *InputReceiver) SetOnReconnectStatus(callback func(status string)) {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	ir.onReconnectStatus = callback
}

// monitorConnection monitors the connection and triggers reconnection when needed
func (ir *InputReceiver) monitorConnection() {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ir.reconnectCtx.Done():
			return
		case <-ticker.C:
			ir.mu.RLock()
			connected := ir.connected
			enabled := ir.reconnectEnabled
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
				select {
				case <-ir.reconnectCtx.Done():
					return
				case <-time.After(10 * time.Second):
				}
				continue
			}

			// Connection appears to be healthy, rely on SSH's built-in keepalive
			// No need to log this regularly
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
			// Connection successful, health check removed
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
		if err := ir.sshConnection.Disconnect(); err != nil {
			logger.Errorf("Failed to disconnect SSH connection: %v", err)
		}
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
		logger.Debugf("[CLIENT-RECEIVER] Injecting mouse move event")
		return backend.InjectMouseMove(e.MouseMove.Dx, e.MouseMove.Dy)

	case *protocol.InputEvent_MouseButton:
		logger.Debugf("[CLIENT-RECEIVER] Injecting mouse button event")
		return backend.InjectMouseButton(e.MouseButton.Button, e.MouseButton.Pressed)

	case *protocol.InputEvent_MouseScroll:
		logger.Debugf("[CLIENT-RECEIVER] Injecting mouse scroll event")
		return backend.InjectMouseScroll(e.MouseScroll.Dx, e.MouseScroll.Dy)

	case *protocol.InputEvent_Keyboard:
		logger.Debugf("[CLIENT-RECEIVER] Injecting keyboard event: key=%d, pressed=%v, modifiers=%032b", 
			e.Keyboard.Key, e.Keyboard.Pressed, e.Keyboard.Modifiers)
		// Check if backend supports modifiers
		if modBackend, ok := ir.inputBackend.(interface {
			InjectKeyEventWithModifiers(uint32, bool, uint32) error
		}); ok {
			logger.Debugf("[CLIENT-RECEIVER] Using InjectKeyEventWithModifiers")
			return modBackend.InjectKeyEventWithModifiers(e.Keyboard.Key, e.Keyboard.Pressed, e.Keyboard.Modifiers)
		}
		// Fallback to standard injection
		logger.Warnf("[CLIENT-RECEIVER] Backend doesn't support modifiers, falling back to standard injection")
		return backend.InjectKeyEvent(e.Keyboard.Key, e.Keyboard.Pressed)

	case *protocol.InputEvent_MousePosition:
		logger.Debugf("[CLIENT-RECEIVER] Received mouse position event")
		// Use absolute positioning if supported by the backend
		x := e.MousePosition.X
		y := e.MousePosition.Y
		if x < 0 || y < 0 {
			logger.Warnf("[CLIENT-RECEIVER] Negative mouse position: x=%d, y=%d", x, y)
			return nil
		}
		if err := backend.InjectMousePosition(uint32(x), uint32(y)); err != nil {
			logger.Warnf("[CLIENT-RECEIVER] Failed to inject absolute mouse position: %v", err)
			// Fall back to relative movement calculation if absolute positioning fails
			// This is just a placeholder - proper implementation would track current position
			return nil
		}
		return nil

	default:
		logger.Errorf("[CLIENT-RECEIVER] Unsupported input event type: %T", event.Event)
		return fmt.Errorf("unsupported input event type: %T", event.Event)
	}
}
