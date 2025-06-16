package server

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/ipc"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/network"
	pb "github.com/bnema/waymon/internal/proto"
	"github.com/bnema/waymon/internal/protocol"
)

// ClientManager manages connected clients and input routing
type ClientManager struct {
	mu               sync.RWMutex
	clients          map[string]*ConnectedClient
	activeClientID   string // Currently being controlled
	inputBackend     input.InputBackend
	inputEventsCtx   context.Context
	inputCancel      context.CancelFunc
	sshServer        *network.SSHServer
	controllingLocal bool // Whether server is controlling local system

	// UI notification callback and throttling
	onActivity      func(level, message string)
	lastActivityLog time.Time
	activityCount   int

	// Cursor position tracking for each client
	clientCursors map[string]*cursorState
}

// cursorState tracks cursor position for a client
type cursorState struct {
	x, y   float64
	bounds rect
}

// rect represents a bounding rectangle
type rect struct {
	minX, minY float64
	maxX, maxY float64
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
	// Create server input backend (evdev for capture)
	inputBackend, err := input.CreateServerBackend()
	if err != nil {
		return nil, fmt.Errorf("failed to create server input backend: %w", err)
	}

	return &ClientManager{
		clients:          make(map[string]*ConnectedClient),
		inputBackend:     inputBackend,
		controllingLocal: true, // Start by controlling local system
		clientCursors:    make(map[string]*cursorState),
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
	logger.Debug("[SERVER-MANAGER] Setting up input event handler")
	cm.inputBackend.OnInputEvent(cm.HandleInputEvent)

	// Start input backend
	logger.Debug("[SERVER-MANAGER] Starting input backend")
	if err := cm.inputBackend.Start(ctx); err != nil {
		return fmt.Errorf("failed to start input backend: %w", err)
	}
	logger.Debug("[SERVER-MANAGER] Input backend started successfully")

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

	// Stop input backend
	if err := cm.inputBackend.Stop(); err != nil {
		logger.Errorf("Error stopping input backend: %v", err)
	}

	// Stop SSH server
	if cm.sshServer != nil {
		cm.sshServer.Stop()
	}

	// Note: Client disconnections are handled by SSH server

	cm.clients = make(map[string]*ConnectedClient)
	cm.activeClientID = ""
	cm.controllingLocal = true
	cm.clientCursors = make(map[string]*cursorState)

	return nil
}

// SwitchToClient switches input control to the specified client
func (cm *ClientManager) SwitchToClient(clientID string) error {
	logger.Debugf("[SERVER-MANAGER] SwitchToClient called: clientID=%s", clientID)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if client exists
	client, exists := cm.clients[clientID]
	if !exists {
		logger.Errorf("[SERVER-MANAGER] Client %s not found", clientID)
		return fmt.Errorf("client %s not found", clientID)
	}

	logger.Debugf("[SERVER-MANAGER] Found client: name=%s, address=%s", client.Name, client.Address)

	// Update previous client status
	if cm.activeClientID != "" {
		if prevClient, exists := cm.clients[cm.activeClientID]; exists {
			prevClient.Status = protocol.ClientStatus_CLIENT_IDLE
			logger.Debugf("[SERVER-MANAGER] Previous client %s status set to IDLE", prevClient.Name)
		}
	}

	// Update target in input backend
	logger.Debugf("[SERVER-MANAGER] Setting input backend target to %s", clientID)
	if err := cm.inputBackend.SetTarget(clientID); err != nil {
		logger.Errorf("[SERVER-MANAGER] Failed to set input target: %v", err)
		return fmt.Errorf("failed to set input target: %w", err)
	}

	// Update state
	cm.activeClientID = clientID
	cm.controllingLocal = false
	client.Status = protocol.ClientStatus_CLIENT_BEING_CONTROLLED

	logger.Debugf("[SERVER-MANAGER] State updated: activeClientID=%s, controllingLocal=%v",
		cm.activeClientID, cm.controllingLocal)

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

		logger.Debugf("[SERVER-MANAGER] Sending REQUEST_CONTROL event to client %s", client.Name)
		if err := cm.sshServer.SendEventToClient(client.Address, inputEvent); err != nil {
			logger.Errorf("[SERVER-MANAGER] Failed to send control request to client: %v", err)
		} else {
			logger.Debugf("[SERVER-MANAGER] Successfully sent control request to client %s", client.Name)
		}

		// Position cursor at center of main monitor (monitor at 0,0)
		if err := cm.positionCursorOnMainMonitor(client); err != nil {
			logger.Warnf("[SERVER-MANAGER] Failed to position cursor on main monitor: %v", err)
		}

		// Initialize cursor state for this client
		if len(client.Monitors) > 0 {
			bounds := cm.calculateTotalDisplayBounds(client.Monitors)

			// Find center position (same logic as positionCursorOnMainMonitor)
			var centerX, centerY float64
			if mainMonitor := cm.findMainMonitor(client.Monitors); mainMonitor != nil {
				centerX = float64(mainMonitor.X + (mainMonitor.Width / 2))
				centerY = float64(mainMonitor.Y + (mainMonitor.Height / 2))
			} else {
				// Fallback to center of total bounds
				centerX = (bounds.minX + bounds.maxX) / 2
				centerY = (bounds.minY + bounds.maxY) / 2
			}

			cm.clientCursors[clientID] = &cursorState{
				x:      centerX,
				y:      centerY,
				bounds: bounds,
			}
			logger.Debugf("[SERVER-MANAGER] Initialized cursor state for client %s: pos=(%.1f,%.1f), bounds=(%.0f,%.0f,%.0f,%.0f)",
				client.Name, centerX, centerY, bounds.minX, bounds.minY, bounds.maxX, bounds.maxY)
		}
	} else {
		logger.Error("[SERVER-MANAGER] No SSH server available to send control request")
	}

	logger.Infof("[SERVER-MANAGER] Switched control to client: %s (%s)", client.Name, client.Address)

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

	// Clear target in input backend
	if err := cm.inputBackend.SetTarget(""); err != nil {
		logger.Errorf("Failed to clear input target: %v", err)
	}

	// Update state
	cm.activeClientID = ""
	cm.controllingLocal = true

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

// HandleInputEvent processes input events and routes them to the appropriate target
func (cm *ClientManager) HandleInputEvent(event *protocol.InputEvent) {
	// Handle control events specially
	if controlEvent := event.GetControl(); controlEvent != nil {
		// Skip debug logs for health check events to reduce noise
		if controlEvent.Type != protocol.ControlEvent_HEALTH_CHECK_PING && 
		   controlEvent.Type != protocol.ControlEvent_HEALTH_CHECK_PONG {
			logger.Debugf("[SERVER-MANAGER] handleInputEvent called: type=%T, timestamp=%d, sourceId=%s",
				event.Event, event.Timestamp, event.SourceId)
			logger.Debugf("[SERVER-MANAGER] Routing control event: type=%v", controlEvent.Type)
		}
		cm.handleControlEvent(controlEvent, event.SourceId)
		return
	}

	logger.Debugf("[SERVER-MANAGER] handleInputEvent called: type=%T, timestamp=%d, sourceId=%s",
		event.Event, event.Timestamp, event.SourceId)

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// If controlling local, do nothing (let input go to local system)
	if cm.controllingLocal {
		logger.Debug("[SERVER-MANAGER] Controlling local system, ignoring event")
		return
	}

	// If no active client, switch back to local
	if cm.activeClientID == "" {
		logger.Debug("[SERVER-MANAGER] No active client, ignoring event")
		return
	}

	// Get the active client
	client, exists := cm.clients[cm.activeClientID]
	if !exists {
		logger.Warnf("[SERVER-MANAGER] Active client %s not found, switching to local", cm.activeClientID)
		go func() { // Switch back to local asynchronously
			if err := cm.SwitchToLocal(); err != nil {
				logger.Errorf("Failed to switch to local: %v", err)
			}
		}()
		return
	}

	logger.Debugf("[SERVER-MANAGER] Routing event to client: %s (%s)", client.Name, client.Address)

	// Handle mouse move events with cursor constraints
	if mouseMoveEvent := event.GetMouseMove(); mouseMoveEvent != nil {
		// Get or create cursor state for this client
		cursor, exists := cm.clientCursors[cm.activeClientID]
		if !exists || len(client.Monitors) == 0 {
			// No cursor state or monitors, send event as-is
			logger.Debugf("[SERVER-MANAGER] No cursor state or monitors for client %s, sending raw mouse move", client.Name)
		} else {
			// Apply relative movement to cursor position
			newX := cursor.x + mouseMoveEvent.Dx
			newY := cursor.y + mouseMoveEvent.Dy

			// Constrain to client's display bounds
			constrainedX, constrainedY := cm.constrainCursorPosition(newX, newY, cursor.bounds)

			// Check if we hit a boundary
			hitBoundary := false
			if newX != constrainedX || newY != constrainedY {
				hitBoundary = true
				logger.Debugf("[SERVER-MANAGER] Cursor hit boundary: attempted=(%.1f,%.1f), constrained=(%.1f,%.1f), bounds=(%.0f,%.0f,%.0f,%.0f)",
					newX, newY, constrainedX, constrainedY, cursor.bounds.minX, cursor.bounds.minY, cursor.bounds.maxX, cursor.bounds.maxY)
			}

			// Calculate the actual movement after constraints
			actualDx := constrainedX - cursor.x
			actualDy := constrainedY - cursor.y

			// Update cursor position
			cursor.x = constrainedX
			cursor.y = constrainedY

			// Modify the event to reflect constrained movement
			if hitBoundary {
				// Only send event if there's actual movement
				if actualDx == 0 && actualDy == 0 {
					logger.Debugf("[SERVER-MANAGER] Mouse movement fully constrained, not sending event")
					return
				}

				// Update the event with constrained movement
				mouseMoveEvent.Dx = actualDx
				mouseMoveEvent.Dy = actualDy
			}
		}
	}

	// Handle absolute mouse position events (update our tracking)
	if mousePosEvent := event.GetMousePosition(); mousePosEvent != nil {
		if cursor, exists := cm.clientCursors[cm.activeClientID]; exists {
			// Update tracked position to match absolute position
			cursor.x = float64(mousePosEvent.X)
			cursor.y = float64(mousePosEvent.Y)
			logger.Debugf("[SERVER-MANAGER] Updated cursor position from absolute event: (%.1f,%.1f)", cursor.x, cursor.y)
		}
	}

	// Send input event to the client via SSH
	if cm.sshServer != nil {
		if err := cm.sshServer.SendEventToClient(client.Address, event); err != nil {
			logger.Errorf("[SERVER-MANAGER] Failed to send input event to client %s: %v", cm.activeClientID, err)
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
			logger.Debugf("[SERVER-MANAGER] %s", message)

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
	} else {
		logger.Error("[SERVER-MANAGER] No SSH server available to send events")
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
		// Grant control to the requesting client
		if err := cm.SwitchToClient(sourceID); err != nil {
			logger.Errorf("Failed to grant control to client %s: %v", sourceID, err)
		}
	case protocol.ControlEvent_RELEASE_CONTROL:
		logger.Infof("Client %s released control", sourceID)
		// Release control and switch back to local
		if err := cm.SwitchToLocal(); err != nil {
			logger.Errorf("Failed to release control from client %s: %v", sourceID, err)
		}
	case protocol.ControlEvent_HEALTH_CHECK_PING:
		// Respond to health check ping
		cm.handleHealthCheckPing(sourceID)
	default:
		logger.Warnf("Unknown control event type from %s: %v", sourceID, controlEvent.Type)
	}
}

// updateClientConfiguration updates a client's configuration
func (cm *ClientManager) updateClientConfiguration(config *protocol.ClientConfig, sourceID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	logger.Debugf("[SERVER-MANAGER] updateClientConfiguration: clientId=%s, clientName=%s, sourceID=%s",
		config.ClientId, config.ClientName, sourceID)
	logger.Debugf("[SERVER-MANAGER] Current registered clients: %d", len(cm.clients))
	for id, client := range cm.clients {
		logger.Debugf("[SERVER-MANAGER]   Client: id=%s, name=%s, address=%s", id, client.Name, client.Address)
	}

	// Find the client by multiple methods:
	// 1. Exact ID match
	// 2. Exact name match
	// 3. Address-based match (since registration uses address as ID)
	// 4. Source ID match (the SSH session ID)
	// 5. If only one client connected, assume it's that client
	var targetClient *ConnectedClient

	// Try exact matches first
	for id, client := range cm.clients {
		// Check if the client ID matches the config ID, name, or source ID
		if client.ID == config.ClientId || client.Name == config.ClientName ||
			id == config.ClientId || id == sourceID || client.Address == sourceID {
			targetClient = client
			logger.Debugf("[SERVER-MANAGER] Found client by match: id=%s, name=%s, address=%s",
				client.ID, client.Name, client.Address)
			break
		}
	}

	// If no exact match and we only have one client, use that client
	if targetClient == nil && len(cm.clients) == 1 {
		for _, client := range cm.clients {
			targetClient = client
			logger.Debugf("[SERVER-MANAGER] Using only connected client: %s", client.Address)
			break
		}
	}

	if targetClient != nil {
		// Update existing client
		targetClient.Monitors = config.Monitors
		targetClient.Capabilities = config.Capabilities

		// Update name to use the client-provided name instead of address
		if config.ClientName != "" && targetClient.Name != config.ClientName {
			logger.Debugf("[SERVER-MANAGER] Updating client name from '%s' to '%s'", targetClient.Name, config.ClientName)
			targetClient.Name = config.ClientName
		}

		logger.Infof("[SERVER-MANAGER] Updated client configuration for %s: %d monitors, compositor: %s",
			targetClient.Name, len(config.Monitors), config.Capabilities.WaylandCompositor)

		// Log monitor details
		for i, monitor := range config.Monitors {
			logger.Debugf("[SERVER-MANAGER]   Monitor %d: %s (%dx%d at %d,%d) primary=%v scale=%.1f",
				i+1, monitor.Name, monitor.Width, monitor.Height,
				monitor.X, monitor.Y, monitor.Primary, monitor.Scale)
		}

		// Notify UI of configuration update to refresh display
		if cm.onActivity != nil {
			cm.onActivity("INFO", fmt.Sprintf("Client %s configured with %d monitors", targetClient.Name, len(config.Monitors)))
		}

		// Update cursor bounds if this is the active client
		if cm.activeClientID == targetClient.ID && len(config.Monitors) > 0 {
			bounds := cm.calculateTotalDisplayBounds(config.Monitors)
			if cursor, exists := cm.clientCursors[targetClient.ID]; exists {
				cursor.bounds = bounds
				// Constrain current position to new bounds
				cursor.x, cursor.y = cm.constrainCursorPosition(cursor.x, cursor.y, bounds)
				logger.Debugf("[SERVER-MANAGER] Updated cursor bounds for active client %s: bounds=(%.0f,%.0f,%.0f,%.0f)",
					targetClient.Name, bounds.minX, bounds.minY, bounds.maxX, bounds.maxY)
			}
		}
	} else {
		logger.Warnf("[SERVER-MANAGER] Received client config from unknown client: %s (source: %s)", config.ClientName, sourceID)
		logger.Warnf("[SERVER-MANAGER] Available clients: %v", func() []string {
			var addrs []string
			for _, client := range cm.clients {
				addrs = append(addrs, client.Address)
			}
			return addrs
		}())
	}
}

// RegisterClient registers a new client connection
func (cm *ClientManager) RegisterClient(id, name, address string) {
	logger.Debugf("[SERVER-MANAGER] RegisterClient called: id=%s, name=%s, address=%s", id, name, address)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	client := &ConnectedClient{
		ID:          id,
		Name:        name,
		Address:     address,
		Status:      protocol.ClientStatus_CLIENT_IDLE,
		ConnectedAt: time.Now(),
	}

	cm.clients[id] = client
	logger.Infof("[SERVER-MANAGER] Registered client: %s (%s) from %s", name, id, address)
	logger.Debugf("[SERVER-MANAGER] Total clients: %d", len(cm.clients))

	// Notify UI if callback is set
	if cm.onActivity != nil {
		// Force a UI refresh by sending an activity notification
		// The UI will refresh its client list when it receives this
		cm.onActivity("INFO", fmt.Sprintf("Client connected: %s (%s)", name, address))
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

	// Clean up cursor state
	delete(cm.clientCursors, id)

	logger.Infof("Unregistered client: %s (%s)", client.Name, id)

	// Notify UI if callback is set
	if cm.onActivity != nil {
		// Force a UI refresh by sending an activity notification
		// The UI will refresh its client list when it receives this
		cm.onActivity("INFO", fmt.Sprintf("Client disconnected: %s (%s)", client.Name, client.Address))
	}
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

// NotifyShutdown sends shutdown notification to all connected clients
func (cm *ClientManager) NotifyShutdown() {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.sshServer == nil {
		logger.Warn("Cannot notify clients of shutdown - SSH server not available")
		return
	}

	// Create shutdown control event
	controlEvent := &protocol.ControlEvent{
		Type: protocol.ControlEvent_SERVER_SHUTDOWN,
	}
	inputEvent := &protocol.InputEvent{
		Event: &protocol.InputEvent_Control{
			Control: controlEvent,
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "server",
	}

	// Send to all connected clients
	for _, client := range cm.clients {
		if err := cm.sshServer.SendEventToClient(client.Address, inputEvent); err != nil {
			logger.Errorf("Failed to send shutdown notification to client %s: %v", client.Name, err)
		} else {
			logger.Infof("Sent shutdown notification to client %s (%s)", client.Name, client.Address)
		}
	}

	// Give clients a moment to process the shutdown notification
	if len(cm.clients) > 0 {
		logger.Info("Waiting for clients to process shutdown notification...")
		time.Sleep(1 * time.Second)
	}
}

// handleHealthCheckPing responds to a health check ping from a client
func (cm *ClientManager) handleHealthCheckPing(sourceID string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.sshServer == nil {
		logger.Warn("Cannot respond to health check - SSH server not available")
		return
	}

	// Find the client by source ID (checking ID, name, and address)
	var clientAddr string
	for _, client := range cm.clients {
		if client.ID == sourceID || client.Name == sourceID || client.Address == sourceID {
			clientAddr = client.Address
			break
		}
	}

	// Also check if sourceID matches just the client name without address
	if clientAddr == "" {
		for _, client := range cm.clients {
			// Extract just the name part if the client name includes address info
			clientName := client.Name
			if idx := strings.Index(clientName, " ("); idx > 0 {
				clientName = clientName[:idx]
			}
			if clientName == sourceID {
				clientAddr = client.Address
				break
			}
		}
	}

	if clientAddr == "" {
		logger.Warnf("Received health check ping from unknown client: %s", sourceID)
		logger.Debugf("Available clients: %v", func() []string {
			var info []string
			for _, c := range cm.clients {
				info = append(info, fmt.Sprintf("id=%s, name=%s, addr=%s", c.ID, c.Name, c.Address))
			}
			return info
		}())
		return
	}

	// Create health check pong response
	controlEvent := &protocol.ControlEvent{
		Type: protocol.ControlEvent_HEALTH_CHECK_PONG,
	}
	inputEvent := &protocol.InputEvent{
		Event: &protocol.InputEvent_Control{
			Control: controlEvent,
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "server",
	}

	// Send response to the client
	if err := cm.sshServer.SendEventToClient(clientAddr, inputEvent); err != nil {
		logger.Errorf("Failed to send health check response to client %s: %v", sourceID, err)
	} else {
		logger.Debug("Sent health check response to client %s", sourceID)
	}
}

// positionCursorOnMainMonitor positions the cursor at the center of the main monitor (monitor at 0,0)
func (cm *ClientManager) positionCursorOnMainMonitor(client *ConnectedClient) error {
	if len(client.Monitors) == 0 {
		return fmt.Errorf("client has no monitors configured")
	}

	// Find the main monitor (primary or the one at position 0,0)
	var mainMonitor *protocol.Monitor

	// First try to find primary monitor
	for _, monitor := range client.Monitors {
		if monitor.Primary {
			mainMonitor = monitor
			break
		}
	}

	// If no primary monitor found, find monitor at position 0,0
	if mainMonitor == nil {
		for _, monitor := range client.Monitors {
			if monitor.X == 0 && monitor.Y == 0 {
				mainMonitor = monitor
				break
			}
		}
	}

	// If still no monitor found, use the first one
	if mainMonitor == nil {
		mainMonitor = client.Monitors[0]
	}

	// Calculate center position of the main monitor
	centerX := mainMonitor.X + (mainMonitor.Width / 2)
	centerY := mainMonitor.Y + (mainMonitor.Height / 2)

	// Create cursor position event
	positionEvent := &protocol.MousePositionEvent{
		X: centerX,
		Y: centerY,
	}

	inputEvent := &protocol.InputEvent{
		Event: &protocol.InputEvent_MousePosition{
			MousePosition: positionEvent,
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "server",
	}

	// Send the positioning event to the client
	if err := cm.sshServer.SendEventToClient(client.Address, inputEvent); err != nil {
		return fmt.Errorf("failed to send cursor position event: %w", err)
	}

	logger.Infof("Positioned cursor at center of main monitor: %s (%dx%d at %d,%d) -> cursor at (%d,%d)",
		mainMonitor.Name, mainMonitor.Width, mainMonitor.Height,
		mainMonitor.X, mainMonitor.Y, centerX, centerY)

	return nil
}

// calculateTotalDisplayBounds calculates the bounding box of all monitors
func (cm *ClientManager) calculateTotalDisplayBounds(monitors []*protocol.Monitor) rect {
	if len(monitors) == 0 {
		return rect{minX: 0, minY: 0, maxX: 1920, maxY: 1080} // Default to 1080p
	}

	// Initialize bounds with first monitor
	bounds := rect{
		minX: float64(monitors[0].X),
		minY: float64(monitors[0].Y),
		maxX: float64(monitors[0].X + monitors[0].Width),
		maxY: float64(monitors[0].Y + monitors[0].Height),
	}

	// Expand bounds to include all monitors
	for _, monitor := range monitors[1:] {
		minX := float64(monitor.X)
		minY := float64(monitor.Y)
		maxX := float64(monitor.X + monitor.Width)
		maxY := float64(monitor.Y + monitor.Height)

		if minX < bounds.minX {
			bounds.minX = minX
		}
		if minY < bounds.minY {
			bounds.minY = minY
		}
		if maxX > bounds.maxX {
			bounds.maxX = maxX
		}
		if maxY > bounds.maxY {
			bounds.maxY = maxY
		}
	}

	return bounds
}

// constrainCursorPosition constrains cursor position within the given bounds
func (cm *ClientManager) constrainCursorPosition(x, y float64, bounds rect) (float64, float64) {
	// Constrain X coordinate
	if x < bounds.minX {
		x = bounds.minX
	} else if x > bounds.maxX {
		x = bounds.maxX
	}

	// Constrain Y coordinate
	if y < bounds.minY {
		y = bounds.minY
	} else if y > bounds.maxY {
		y = bounds.maxY
	}

	return x, y
}

// findMainMonitor finds the primary monitor or the monitor at position 0,0
func (cm *ClientManager) findMainMonitor(monitors []*protocol.Monitor) *protocol.Monitor {
	if len(monitors) == 0 {
		return nil
	}

	// First try to find primary monitor
	for _, monitor := range monitors {
		if monitor.Primary {
			return monitor
		}
	}

	// If no primary monitor found, find monitor at position 0,0
	for _, monitor := range monitors {
		if monitor.X == 0 && monitor.Y == 0 {
			return monitor
		}
	}

	// If still no monitor found, return the first one
	return monitors[0]
}

// HandleSwitchCommand implements ipc.MessageHandler
func (cm *ClientManager) HandleSwitchCommand(cmd *pb.SwitchCommand) (*pb.IPCMessage, error) {
	logger.Debugf("[SERVER-MANAGER] HandleSwitchCommand: action=%v", cmd.Action)

	switch cmd.Action {
	case pb.SwitchAction_SWITCH_ACTION_NEXT:
		// Switch to next client in rotation
		if err := cm.switchToNextClientOrLocal(); err != nil {
			return nil, fmt.Errorf("failed to switch to next: %w", err)
		}

	case pb.SwitchAction_SWITCH_ACTION_PREVIOUS:
		// Switch to previous client in rotation
		if err := cm.switchToPreviousClientOrLocal(); err != nil {
			return nil, fmt.Errorf("failed to switch to previous: %w", err)
		}

	case pb.SwitchAction_SWITCH_ACTION_ENABLE:
		// Legacy: Enable sharing - switch to first available client
		clients := cm.GetConnectedClients()
		if len(clients) > 0 {
			if err := cm.SwitchToClient(clients[0].ID); err != nil {
				return nil, fmt.Errorf("failed to enable sharing: %w", err)
			}
		} else {
			return nil, fmt.Errorf("no clients connected")
		}

	case pb.SwitchAction_SWITCH_ACTION_DISABLE:
		// Legacy: Disable sharing - switch to local
		if err := cm.SwitchToLocal(); err != nil {
			return nil, fmt.Errorf("failed to disable sharing: %w", err)
		}

	default:
		return nil, fmt.Errorf("unknown switch action: %v", cmd.Action)
	}

	// Return status response with current state
	return cm.HandleStatusQuery(&pb.StatusQuery{})
}

// HandleStatusQuery implements ipc.MessageHandler
func (cm *ClientManager) HandleStatusQuery(query *pb.StatusQuery) (*pb.IPCMessage, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Build computer list and find current index
	computerNames := []string{"server"} // Server is always index 0
	currentIndex := int32(0)            // Default to server

	// Get all connected clients
	clientIDs := make([]string, 0, len(cm.clients))
	for id := range cm.clients {
		clientIDs = append(clientIDs, id)
	}

	// Sort client IDs for consistent ordering
	sort.Strings(clientIDs)
	for i, id := range clientIDs {
		client := cm.clients[id]
		computerNames = append(computerNames, client.Name)

		// If this is the active client, set the current index
		if !cm.controllingLocal && id == cm.activeClientID {
			currentIndex = int32(i + 1) //nolint:gosec // client index conversion is safe
		}
	}

	// Determine if mouse sharing is active (not controlling local)
	active := !cm.controllingLocal

	// We're always "connected" when running as server
	connected := true

	// Server host is ourselves
	serverHost := "localhost"

	return ipc.NewStatusResponseMessage(
		active,
		connected,
		serverHost,
		currentIndex,
		int32(len(computerNames)), //nolint:gosec // computer count conversion is safe
		computerNames,
	)
}

// switchToNextClientOrLocal switches to the next client in rotation, or to local if only one client
func (cm *ClientManager) switchToNextClientOrLocal() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Get sorted list of client IDs
	clientIDs := make([]string, 0, len(cm.clients))
	for id := range cm.clients {
		clientIDs = append(clientIDs, id)
	}
	// Sort for consistent ordering
	sort.Strings(clientIDs)

	if len(clientIDs) == 0 {
		// No clients, stay on local
		return cm.switchToLocalInternal()
	}

	if cm.controllingLocal {
		// Currently on local, switch to first client
		cm.mu.Unlock()
		err := cm.SwitchToClient(clientIDs[0])
		cm.mu.Lock()
		return err
	}

	// Find current client index
	currentIndex := -1
	for i, id := range clientIDs {
		if id == cm.activeClientID {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		// Active client not found, switch to local
		return cm.switchToLocalInternal()
	}

	// Calculate next index
	nextIndex := (currentIndex + 1) % (len(clientIDs) + 1) // +1 to include local

	if nextIndex == len(clientIDs) {
		// Wrap around to local
		return cm.switchToLocalInternal()
	}

	// Switch to next client
	cm.mu.Unlock()
	err := cm.SwitchToClient(clientIDs[nextIndex])
	cm.mu.Lock()
	return err
}

// switchToPreviousClientOrLocal switches to the previous client in rotation
func (cm *ClientManager) switchToPreviousClientOrLocal() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Get sorted list of client IDs
	clientIDs := make([]string, 0, len(cm.clients))
	for id := range cm.clients {
		clientIDs = append(clientIDs, id)
	}
	// Sort for consistent ordering
	sort.Strings(clientIDs)

	if len(clientIDs) == 0 {
		// No clients, stay on local
		return cm.switchToLocalInternal()
	}

	if cm.controllingLocal {
		// Currently on local, switch to last client
		cm.mu.Unlock()
		err := cm.SwitchToClient(clientIDs[len(clientIDs)-1])
		cm.mu.Lock()
		return err
	}

	// Find current client index
	currentIndex := -1
	for i, id := range clientIDs {
		if id == cm.activeClientID {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		// Active client not found, switch to local
		return cm.switchToLocalInternal()
	}

	// Calculate previous index
	previousIndex := currentIndex - 1
	if previousIndex < 0 {
		// Wrap around to local
		return cm.switchToLocalInternal()
	}

	// Switch to previous client
	cm.mu.Unlock()
	err := cm.SwitchToClient(clientIDs[previousIndex])
	cm.mu.Lock()
	return err
}

// switchToLocalInternal is the internal version that assumes the lock is already held
func (cm *ClientManager) switchToLocalInternal() error {
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

	// Clear target in input backend
	if err := cm.inputBackend.SetTarget(""); err != nil {
		logger.Errorf("Failed to clear input target: %v", err)
	}

	// Update state
	cm.activeClientID = ""
	cm.controllingLocal = true

	logger.Info("Switched control to local system")

	// Notify UI if callback is set
	if cm.onActivity != nil {
		cm.onActivity("INFO", "Released client control - now controlling local system")
	}

	return nil
}
