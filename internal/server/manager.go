package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/protocol"
	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/network"
	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/logger"
)

// ClientManager manages connected clients and input routing
type ClientManager struct {
	mu              sync.RWMutex
	clients         map[string]*ConnectedClient
	activeClientID  string // Currently being controlled
	evdevCapture    *input.EvdevCapture
	inputEventsCtx  context.Context
	inputCancel     context.CancelFunc
	sshServer       *network.SSHServer
	controllingLocal bool // Whether server is controlling local system
	
	// UI notification callback and throttling
	onActivity      func(level, message string)
	lastActivityLog time.Time
	activityCount   int
}

// ConnectedClient represents a client that can receive input
type ConnectedClient struct {
	ID          string
	Name        string
	Address     string
	Status      protocol.ClientStatus
	ConnectedAt time.Time
	
	// Client configuration received on connect
	Monitors     []*protocol.Monitor
	Capabilities *protocol.ClientCapabilities
}

// NewClientManager creates a new client manager for the server
func NewClientManager() (*ClientManager, error) {
	// Get config to check for configured devices
	cfg := config.Get()
	
	// Create evdev capture with configured devices if available
	var evdevCapture *input.EvdevCapture
	if cfg.Input.MouseDevice != "" || cfg.Input.KeyboardDevice != "" {
		evdevCapture = input.NewEvdevCaptureWithDevices(cfg.Input.MouseDevice, cfg.Input.KeyboardDevice)
	} else {
		evdevCapture = input.NewEvdevCapture()
	}

	return &ClientManager{
		clients:         make(map[string]*ConnectedClient),
		evdevCapture:    evdevCapture,
		controllingLocal: true, // Start by controlling local system
	}, nil
}

// Start starts the client manager and input capture
func (cm *ClientManager) Start(ctx context.Context, port int) error {
	// Get config for SSH server
	cfg := config.Get()
	
	// Start SSH server to accept client connections
	sshServer := network.NewSSHServer(port, cfg.Server.SSHHostKeyPath, cfg.Server.SSHAuthKeysPath)
	cm.sshServer = sshServer

	// Create context for input event processing
	cm.inputEventsCtx, cm.inputCancel = context.WithCancel(ctx)

	// Set up event handler
	cm.evdevCapture.OnInputEvent(cm.handleInputEvent)

	// Start evdev capture
	if err := cm.evdevCapture.Start(ctx); err != nil {
		return fmt.Errorf("failed to start evdev capture: %w", err)
	}

	// Start SSH server
	go func() {
		if err := sshServer.Start(ctx); err != nil {
			logger.Errorf("SSH server error: %v", err)
		}
	}()

	logger.Infof("Server started on port %d, controlling local system", port)
	return nil
}

// Stop stops the client manager
func (cm *ClientManager) Stop() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Stop evdev capture
	if err := cm.evdevCapture.Stop(); err != nil {
		logger.Errorf("Error stopping evdev capture: %v", err)
	}

	// Stop SSH server
	if cm.sshServer != nil {
		cm.sshServer.Stop()
	}

	// Note: Client disconnections are handled by SSH server

	cm.clients = make(map[string]*ConnectedClient)
	cm.activeClientID = ""
	cm.controllingLocal = true

	return nil
}

// SwitchToClient switches input control to the specified client
func (cm *ClientManager) SwitchToClient(clientID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if client exists
	client, exists := cm.clients[clientID]
	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	// Update previous client status
	if cm.activeClientID != "" {
		if prevClient, exists := cm.clients[cm.activeClientID]; exists {
			prevClient.Status = protocol.ClientStatus_CLIENT_IDLE
		}
	}

	// Update target in evdev capture
	if err := cm.evdevCapture.SetTarget(clientID); err != nil {
		return fmt.Errorf("failed to set input target: %w", err)
	}

	// Update state
	cm.activeClientID = clientID
	cm.controllingLocal = false
	client.Status = protocol.ClientStatus_CLIENT_BEING_CONTROLLED

	// Send control event to notify client they're being controlled
	if cm.sshServer != nil {
		controlEvent := &protocol.ControlEvent{
			Type:     protocol.ControlEvent_REQUEST_CONTROL,
			TargetId: cm.activeClientID,
		}
		inputEvent := &protocol.InputEvent{
			Event: &protocol.InputEvent_Control{
				Control: controlEvent,
			},
			Timestamp: time.Now().UnixNano(),
			SourceId:  "server",
		}
		if err := cm.sshServer.SendEventToClient(client.Address, inputEvent); err != nil {
			logger.Errorf("Failed to send control request to client: %v", err)
		} else {
			logger.Debugf("Sent control request to client %s", client.Name)
		}
	}

	logger.Infof("Switched control to client: %s (%s)", client.Name, client.Address)
	
	// Notify UI if callback is set
	if cm.onActivity != nil {
		cm.onActivity("INFO", fmt.Sprintf("Started controlling client: %s (%s)", client.Name, client.Address))
	}
	
	return nil
}

// SwitchToLocal switches input control back to the local system
func (cm *ClientManager) SwitchToLocal() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Update previous client status and notify them
	if cm.activeClientID != "" {
		if prevClient, exists := cm.clients[cm.activeClientID]; exists {
			prevClient.Status = protocol.ClientStatus_CLIENT_IDLE
			
			// Send release control event to previous client
			if cm.sshServer != nil {
				controlEvent := &protocol.ControlEvent{
					Type:     protocol.ControlEvent_RELEASE_CONTROL,
					TargetId: cm.activeClientID,
				}
				inputEvent := &protocol.InputEvent{
					Event: &protocol.InputEvent_Control{
						Control: controlEvent,
					},
					Timestamp: time.Now().UnixNano(),
					SourceId:  "server",
				}
				if err := cm.sshServer.SendEventToClient(prevClient.Address, inputEvent); err != nil {
					logger.Errorf("Failed to send control release to previous client: %v", err)
				} else {
					logger.Debugf("Sent control release to previous client %s", prevClient.Name)
				}
			}
		}
	}

	// Update state
	cm.activeClientID = ""
	cm.controllingLocal = true

	// Stop routing input events to clients
	// (input will naturally go to local system)

	logger.Info("Switched control to local system")
	
	// Notify UI if callback is set
	if cm.onActivity != nil {
		cm.onActivity("INFO", "Released client control - now controlling local system")
	}
	
	return nil
}

// SwitchToNextClient switches to the next available client
func (cm *ClientManager) SwitchToNextClient() error {
	cm.mu.RLock()
	clientIDs := make([]string, 0, len(cm.clients))
	for id := range cm.clients {
		clientIDs = append(clientIDs, id)
	}
	cm.mu.RUnlock()

	if len(clientIDs) == 0 {
		return cm.SwitchToLocal()
	}

	// Find current index
	currentIndex := -1
	for i, id := range clientIDs {
		if id == cm.activeClientID {
			currentIndex = i
			break
		}
	}

	// Switch to next client (or first if we're on local)
	nextIndex := (currentIndex + 1) % len(clientIDs)
	return cm.SwitchToClient(clientIDs[nextIndex])
}

// GetConnectedClients returns a list of connected clients
func (cm *ClientManager) GetConnectedClients() []*ConnectedClient {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	clients := make([]*ConnectedClient, 0, len(cm.clients))
	for _, client := range cm.clients {
		clients = append(clients, client)
	}
	return clients
}

// GetActiveClient returns the currently controlled client
func (cm *ClientManager) GetActiveClient() *ConnectedClient {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.activeClientID == "" {
		return nil
	}
	return cm.clients[cm.activeClientID]
}

// IsControllingLocal returns whether the server is controlling the local system
func (cm *ClientManager) IsControllingLocal() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.controllingLocal
}

// handleInputEvent processes input events and routes them to the appropriate target
func (cm *ClientManager) handleInputEvent(event *protocol.InputEvent) {
	// Handle control events specially
	if controlEvent := event.GetControl(); controlEvent != nil {
		cm.handleControlEvent(controlEvent, event.SourceId)
		return
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// If controlling local, do nothing (let input go to local system)
	if cm.controllingLocal {
		return
	}

	// If no active client, switch back to local
	if cm.activeClientID == "" {
		return
	}

	// Get the active client
	client, exists := cm.clients[cm.activeClientID]
	if !exists {
		logger.Warnf("Active client %s not found, switching to local", cm.activeClientID)
		go cm.SwitchToLocal() // Switch back to local asynchronously
		return
	}

	// Send input event to the client via SSH
	if cm.sshServer != nil {
		if err := cm.sshServer.SendEventToClient(client.Address, event); err != nil {
			logger.Errorf("Failed to send input event to client %s: %v", cm.activeClientID, err)
		} else {
			// Log input activity with more user-friendly messages
			eventType := "input"
			switch event.Event.(type) {
			case *protocol.InputEvent_MouseMove:
				eventType = "mouse movement"
			case *protocol.InputEvent_MouseButton:
				eventType = "mouse click"
			case *protocol.InputEvent_MouseScroll:
				eventType = "mouse scroll"
			case *protocol.InputEvent_Keyboard:
				eventType = "keyboard"
			}
			message := fmt.Sprintf("Injecting %s input into %s (%s)", eventType, client.Name, client.Address)
			logger.Debugf(message)
			
			// Send to UI with throttling to avoid spam
			if cm.onActivity != nil {
				now := time.Now()
				cm.activityCount++
				
				// Log activity every 2 seconds or every 50 events
				if now.Sub(cm.lastActivityLog) > 2*time.Second || cm.activityCount >= 50 {
					if cm.activityCount > 1 {
						summary := fmt.Sprintf("Actively controlling %s (%s) - %d input events sent", 
							client.Name, client.Address, cm.activityCount)
						cm.onActivity("INFO", summary)
					} else {
						cm.onActivity("INFO", message)
					}
					cm.lastActivityLog = now
					cm.activityCount = 0
				}
			}
		}
	}
}

// handleControlEvent processes control events from clients
func (cm *ClientManager) handleControlEvent(controlEvent *protocol.ControlEvent, sourceID string) {
	switch controlEvent.Type {
	case protocol.ControlEvent_CLIENT_CONFIG:
		if config := controlEvent.ClientConfig; config != nil {
			cm.updateClientConfiguration(config, sourceID)
		}
	case protocol.ControlEvent_REQUEST_CONTROL:
		logger.Infof("Client %s requested control", sourceID)
	case protocol.ControlEvent_RELEASE_CONTROL:
		logger.Infof("Client %s released control", sourceID)
	default:
		logger.Warnf("Unknown control event type from %s: %v", sourceID, controlEvent.Type)
	}
}

// updateClientConfiguration updates a client's configuration
func (cm *ClientManager) updateClientConfiguration(config *protocol.ClientConfig, sourceID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Find the client by connection source or create/update entry
	var targetClient *ConnectedClient
	for _, client := range cm.clients {
		if client.ID == config.ClientId || client.Name == config.ClientName {
			targetClient = client
			break
		}
	}

	if targetClient != nil {
		// Update existing client
		targetClient.Monitors = config.Monitors
		targetClient.Capabilities = config.Capabilities
		if targetClient.Name == "" {
			targetClient.Name = config.ClientName
		}
		if targetClient.ID == "" {
			targetClient.ID = config.ClientId
		}

		logger.Infof("Updated client configuration for %s: %d monitors, compositor: %s", 
			targetClient.Name, len(config.Monitors), config.Capabilities.WaylandCompositor)
		
		// Log monitor details
		for i, monitor := range config.Monitors {
			logger.Debugf("  Monitor %d: %s (%dx%d at %d,%d) primary=%v scale=%.1f", 
				i+1, monitor.Name, monitor.Width, monitor.Height, 
				monitor.X, monitor.Y, monitor.Primary, monitor.Scale)
		}
	} else {
		logger.Warnf("Received client config from unknown client: %s (source: %s)", config.ClientName, sourceID)
	}
}

// RegisterClient registers a new client connection
func (cm *ClientManager) RegisterClient(id, name, address string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	client := &ConnectedClient{
		ID:         id,
		Name:       name,
		Address:    address,
		Status:     protocol.ClientStatus_CLIENT_IDLE,
		ConnectedAt: time.Now(),
	}

	cm.clients[id] = client
	logger.Infof("Registered client: %s (%s) from %s", name, id, address)
	
	// Notify UI if callback is set
	if cm.onActivity != nil {
		cm.onActivity("INFO", fmt.Sprintf("Client registered: %s (%s)", name, address))
	}
}

// UnregisterClient removes a client connection
func (cm *ClientManager) UnregisterClient(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	client, exists := cm.clients[id]
	if !exists {
		return
	}

	// If this was the active client, switch to local
	if cm.activeClientID == id {
		cm.activeClientID = ""
		cm.controllingLocal = true
		// Input will automatically go to local system when not actively routing
	}

	// Remove client
	delete(cm.clients, id)

	logger.Infof("Unregistered client: %s (%s)", client.Name, id)
}

// SetOnActivity sets a callback for activity notifications
func (cm *ClientManager) SetOnActivity(callback func(level, message string)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onActivity = callback
}

// SetSSHServer sets the SSH server for sending events to clients
func (cm *ClientManager) SetSSHServer(sshServer *network.SSHServer) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.sshServer = sshServer
}


