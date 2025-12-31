package server

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/rs/zerolog"

	"github.com/bnema/waymon/internal/domain"
)

// RegisterClient registers a new client connection.
func (s *UseCaseImpl) RegisterClient(ctx context.Context, id, name, address string) {
	log := zerolog.Ctx(ctx)
	log.Debug().
		Str("id", id).
		Str("name", name).
		Str("address", address).
		Msg("Registering client")

	s.mu.Lock()
	defer s.mu.Unlock()

	client := &domain.Client{
		ID:          id,
		Name:        name,
		Address:     address,
		Status:      domain.ClientIdle,
		ConnectedAt: time.Now(),
	}

	s.clients[id] = client
	log.Info().
		Str("name", name).
		Str("id", id).
		Str("address", address).
		Int("totalClients", len(s.clients)).
		Msg("Registered client")

	// Notify UI
	s.notifyActivity("INFO", fmt.Sprintf("Client connected: %s (%s)", name, address))
}

// UnregisterClient removes a client connection.
func (s *UseCaseImpl) UnregisterClient(ctx context.Context, id string) {
	log := zerolog.Ctx(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	client, exists := s.clients[id]
	if !exists {
		return
	}

	// If this was the active client, switch to local and release input
	if s.activeClientID == id {
		log.Info().Str("client", client.Name).Msg("Active client disconnected, switching to local")

		// Release input capture
		if s.inputCapture != nil {
			if err := s.inputCapture.SetTarget(""); err != nil {
				log.Error().Err(err).Msg("Failed to release input on client disconnect")
			}
		}

		s.activeClientID = ""
		s.controllingLocal = true

		s.notifyActivity("WARN", fmt.Sprintf("Client %s disconnected - control returned to local", client.Name))
	}

	// Remove client
	delete(s.clients, id)

	// Clean up cursor state
	delete(s.clientCursors, id)

	log.Info().Str("name", client.Name).Str("id", id).Msg("Unregistered client")
	s.notifyActivity("INFO", fmt.Sprintf("Client disconnected: %s (%s)", client.Name, client.Address))
}

// GetConnectedClients returns a list of connected clients.
func (s *UseCaseImpl) GetConnectedClients(_ context.Context) []domain.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients := make([]domain.Client, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, *client)
	}
	return clients
}

// GetActiveClient returns the currently controlled client.
func (s *UseCaseImpl) GetActiveClient(_ context.Context) *domain.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.activeClientID == "" {
		return nil
	}
	if client, exists := s.clients[s.activeClientID]; exists {
		// Return a copy
		clientCopy := *client
		return &clientCopy
	}
	return nil
}

// IsControllingLocal returns whether the server is controlling the local system.
func (s *UseCaseImpl) IsControllingLocal(_ context.Context) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.controllingLocal
}

// SwitchToClient switches input control to the specified client.
func (s *UseCaseImpl) SwitchToClient(ctx context.Context, clientID string) error {
	log := zerolog.Ctx(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if we're already controlling this client
	if s.activeClientID == clientID && !s.controllingLocal {
		return nil
	}

	log.Debug().Str("clientID", clientID).Msg("SwitchToClient called")

	// Check if client exists
	client, exists := s.clients[clientID]
	if !exists {
		log.Error().Str("clientID", clientID).Msg("Client not found")
		return domain.ErrClientNotFound
	}

	log.Debug().Str("name", client.Name).Str("address", client.Address).Msg("Found client")

	// Update previous client status
	if s.activeClientID != "" {
		if prevClient, exists := s.clients[s.activeClientID]; exists {
			prevClient.Status = domain.ClientIdle
			log.Debug().Str("prevClient", prevClient.Name).Msg("Previous client status set to IDLE")
		}
	}

	// Update target in input backend
	log.Debug().Str("target", clientID).Msg("Setting input backend target")
	if err := s.inputCapture.SetTarget(clientID); err != nil {
		log.Error().Err(err).Msg("Failed to set input target")
		return fmt.Errorf("failed to set input target: %w", err)
	}

	// Update state
	s.activeClientID = clientID
	s.controllingLocal = false
	client.Status = domain.ClientBeingControlled

	log.Debug().
		Str("activeClientID", s.activeClientID).
		Bool("controllingLocal", s.controllingLocal).
		Msg("State updated")

	// Send control event to notify client they're being controlled
	if s.network != nil {
		serverName := s.getServerName()

		controlEvent := &domain.InputEvent{
			Timestamp: time.Now().UnixNano(),
			SourceID:  "server",
			Control: &domain.ControlEvent{
				Type:     domain.ControlRequestControl,
				TargetID: serverName,
			},
		}

		log.Info().Str("client", client.Name).Str("address", client.Address).Msg("Sending REQUEST_CONTROL event")
		if err := s.network.SendEventToClient(ctx, client.Address, controlEvent); err != nil {
			log.Error().Err(err).Msg("Failed to send control request to client")
		} else {
			log.Info().Str("client", client.Name).Msg("Successfully sent control request")
		}

		// Position cursor at center of main monitor
		if err := s.positionCursorOnMainMonitor(ctx, client); err != nil {
			log.Warn().Err(err).Msg("Failed to position cursor on main monitor")
		}

		// Initialize cursor state for this client
		if len(client.Monitors) > 0 {
			bounds := CalculateTotalDisplayBounds(client.Monitors)

			// Find center position
			var centerX, centerY float64
			if mainMonitor := FindMainMonitor(client.Monitors); mainMonitor != nil {
				centerX = float64(mainMonitor.X + (mainMonitor.Width / 2))
				centerY = float64(mainMonitor.Y + (mainMonitor.Height / 2))
			} else {
				centerX = (bounds.MinX + bounds.MaxX) / 2
				centerY = (bounds.MinY + bounds.MaxY) / 2
			}

			s.clientCursors[clientID] = &domain.CursorState{
				X:      centerX,
				Y:      centerY,
				Bounds: bounds,
			}
			log.Debug().Str("client", client.Name).Msg("Initialized cursor state")
		}
	} else {
		log.Error().Msg("No network server available to send control request")
	}

	log.Info().Str("client", client.Name).Str("address", client.Address).Msg("Switched control to client")
	s.notifyActivity("INFO", fmt.Sprintf("Started controlling client: %s (%s)", client.Name, client.Address))

	return nil
}

// SwitchToLocal switches input control back to the local system.
func (s *UseCaseImpl) SwitchToLocal(ctx context.Context) error {
	log := zerolog.Ctx(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if we're already controlling local
	if s.controllingLocal {
		return nil
	}

	// Update previous client status and notify them
	if s.activeClientID != "" {
		if prevClient, exists := s.clients[s.activeClientID]; exists {
			prevClient.Status = domain.ClientIdle

			// Send release control event to previous client
			if s.network != nil {
				releaseEvent := &domain.InputEvent{
					Timestamp: time.Now().UnixNano(),
					SourceID:  "server",
					Control: &domain.ControlEvent{
						Type:     domain.ControlReleaseControl,
						TargetID: s.activeClientID,
					},
				}
				if err := s.network.SendEventToClient(ctx, prevClient.Address, releaseEvent); err != nil {
					log.Error().Err(err).Msg("Failed to send control release to previous client")
				} else {
					log.Debug().Str("client", prevClient.Name).Msg("Sent control release to previous client")
				}
			}
		}
	}

	// Clear target in input backend
	if err := s.inputCapture.SetTarget(""); err != nil {
		log.Error().Err(err).Msg("Failed to clear input target")
	}

	// Update state
	s.activeClientID = ""
	s.controllingLocal = true

	log.Info().Msg("Switched control to local system")
	s.notifyActivity("INFO", "Released client control - now controlling local system")

	return nil
}

// SwitchToNext switches to the next client in rotation (including local).
func (s *UseCaseImpl) SwitchToNext(ctx context.Context) error {
	log := zerolog.Ctx(ctx)

	s.mu.Lock()

	// Get sorted list of client IDs
	clientIDs := s.getSortedClientIDs()

	if len(clientIDs) == 0 {
		s.mu.Unlock()
		return s.SwitchToLocal(ctx)
	}

	if s.controllingLocal {
		// Currently on local, switch to first client
		clientID := clientIDs[0]
		s.mu.Unlock()
		return s.SwitchToClient(ctx, clientID)
	}

	// Find current client index
	currentIndex := -1
	for i, id := range clientIDs {
		if id == s.activeClientID {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		s.mu.Unlock()
		return s.SwitchToLocal(ctx)
	}

	// Calculate next index (wrap to local after last client)
	nextIndex := currentIndex + 1
	if nextIndex >= len(clientIDs) {
		s.mu.Unlock()
		return s.SwitchToLocal(ctx)
	}

	clientID := clientIDs[nextIndex]
	s.mu.Unlock()

	log.Debug().Str("nextClient", clientID).Msg("Switching to next client")
	return s.SwitchToClient(ctx, clientID)
}

// SwitchToPrevious switches to the previous client in rotation (including local).
func (s *UseCaseImpl) SwitchToPrevious(ctx context.Context) error {
	log := zerolog.Ctx(ctx)

	s.mu.Lock()

	// Get sorted list of client IDs
	clientIDs := s.getSortedClientIDs()

	if len(clientIDs) == 0 {
		s.mu.Unlock()
		return s.SwitchToLocal(ctx)
	}

	if s.controllingLocal {
		// Currently on local, switch to last client
		clientID := clientIDs[len(clientIDs)-1]
		s.mu.Unlock()
		return s.SwitchToClient(ctx, clientID)
	}

	// Find current client index
	currentIndex := -1
	for i, id := range clientIDs {
		if id == s.activeClientID {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		s.mu.Unlock()
		return s.SwitchToLocal(ctx)
	}

	// Calculate previous index (wrap to local before first client)
	if currentIndex == 0 {
		s.mu.Unlock()
		return s.SwitchToLocal(ctx)
	}

	clientID := clientIDs[currentIndex-1]
	s.mu.Unlock()

	log.Debug().Str("prevClient", clientID).Msg("Switching to previous client")
	return s.SwitchToClient(ctx, clientID)
}

// ConnectToSlot connects to a client by slot number (0 = local, 1-5 = clients).
func (s *UseCaseImpl) ConnectToSlot(ctx context.Context, slot int32) error {
	log := zerolog.Ctx(ctx)
	log.Debug().Int32("slot", slot).Msg("ConnectToSlot called")

	s.mu.RLock()
	clientIDs := s.getSortedClientIDs()
	s.mu.RUnlock()

	// Slot 0 is server (local)
	if slot == 0 {
		return s.SwitchToLocal(ctx)
	}

	// Slots 1-5 are clients
	if slot < 1 || slot > 5 {
		return domain.ErrInvalidSlot
	}

	clientIndex := int(slot - 1)
	if clientIndex >= len(clientIDs) {
		return domain.ErrClientNotFound
	}

	return s.SwitchToClient(ctx, clientIDs[clientIndex])
}

// UpdateClientConfiguration updates a client's configuration (monitors, capabilities).
func (s *UseCaseImpl) UpdateClientConfiguration(ctx context.Context, config *domain.ClientConfig, sourceID string) {
	log := zerolog.Ctx(ctx)
	log.Debug().
		Str("clientId", config.ClientID).
		Str("clientName", config.ClientName).
		Str("sourceID", sourceID).
		Msg("Updating client configuration")

	s.mu.Lock()
	defer s.mu.Unlock()

	// Find the client by multiple methods
	var targetClient *domain.Client
	for id, client := range s.clients {
		if client.ID == config.ClientID || client.Name == config.ClientName ||
			id == config.ClientID || id == sourceID || client.Address == sourceID {
			targetClient = client
			log.Debug().
				Str("id", client.ID).
				Str("name", client.Name).
				Str("address", client.Address).
				Msg("Found client by match")
			break
		}
	}

	// If no exact match and we only have one client, use that client
	if targetClient == nil && len(s.clients) == 1 {
		for _, client := range s.clients {
			targetClient = client
			log.Debug().Str("address", client.Address).Msg("Using only connected client")
			break
		}
	}

	if targetClient == nil {
		log.Warn().Str("clientName", config.ClientName).Str("sourceID", sourceID).Msg("Received config from unknown client")
		return
	}

	// Update client configuration
	targetClient.Monitors = config.Monitors
	targetClient.Capabilities = config.Capabilities

	// Update name if provided
	if config.ClientName != "" && targetClient.Name != config.ClientName {
		log.Debug().Str("oldName", targetClient.Name).Str("newName", config.ClientName).Msg("Updating client name")
		targetClient.Name = config.ClientName
	}

	log.Info().
		Str("client", targetClient.Name).
		Int("monitors", len(config.Monitors)).
		Msg("Updated client configuration")

	// Log monitor details
	for i, monitor := range config.Monitors {
		log.Debug().
			Int("index", i+1).
			Str("name", monitor.Name).
			Int32("width", monitor.Width).
			Int32("height", monitor.Height).
			Int32("x", monitor.X).
			Int32("y", monitor.Y).
			Bool("primary", monitor.Primary).
			Float64("scale", monitor.Scale).
			Msg("Monitor")
	}

	s.notifyActivity("INFO", fmt.Sprintf("Client %s configured with %d monitors", targetClient.Name, len(config.Monitors)))

	// Update cursor bounds if this is the active client
	if s.activeClientID == targetClient.ID && len(config.Monitors) > 0 {
		bounds := CalculateTotalDisplayBounds(config.Monitors)
		if cursor, exists := s.clientCursors[targetClient.ID]; exists {
			cursor.Bounds = bounds
			// Constrain current position to new bounds
			cursor.X, cursor.Y = ConstrainCursorPosition(cursor.X, cursor.Y, bounds)
			log.Debug().Str("client", targetClient.Name).Msg("Updated cursor bounds for active client")
		}
	}
}

// getSortedClientIDs returns a sorted list of client IDs for consistent ordering.
// Must be called with the lock held.
func (s *UseCaseImpl) getSortedClientIDs() []string {
	clientIDs := make([]string, 0, len(s.clients))
	for id := range s.clients {
		clientIDs = append(clientIDs, id)
	}
	sort.Strings(clientIDs)
	return clientIDs
}

// positionCursorOnMainMonitor positions the cursor at the center of the main monitor.
func (s *UseCaseImpl) positionCursorOnMainMonitor(ctx context.Context, client *domain.Client) error {
	log := zerolog.Ctx(ctx)

	if len(client.Monitors) == 0 {
		return domain.ErrNoMonitors
	}

	// Find the main monitor
	mainMonitor := FindMainMonitor(client.Monitors)
	if mainMonitor == nil {
		mainMonitor = &client.Monitors[0]
	}

	// Calculate center position
	centerX := mainMonitor.X + (mainMonitor.Width / 2)
	centerY := mainMonitor.Y + (mainMonitor.Height / 2)

	// Create cursor position event
	positionEvent := &domain.InputEvent{
		Timestamp: time.Now().UnixNano(),
		SourceID:  "server",
		MousePosition: &domain.MousePositionEvent{
			X: centerX,
			Y: centerY,
		},
	}

	// Send the positioning event to the client
	if err := s.network.SendEventToClient(ctx, client.Address, positionEvent); err != nil {
		return fmt.Errorf("failed to send cursor position event: %w", err)
	}

	log.Info().
		Str("monitor", mainMonitor.Name).
		Int32("width", mainMonitor.Width).
		Int32("height", mainMonitor.Height).
		Int32("x", mainMonitor.X).
		Int32("y", mainMonitor.Y).
		Int32("cursorX", centerX).
		Int32("cursorY", centerY).
		Msg("Positioned cursor at center of main monitor")

	return nil
}
