package server

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/bnema/waymon/internal/domain"
)

// HandleInputEvent processes input events and routes them to the appropriate target.
func (s *UseCaseImpl) HandleInputEvent(ctx context.Context, event *domain.InputEvent) {
	log := zerolog.Ctx(ctx)

	// Handle control events specially
	if event.Control != nil {
		log.Debug().
			Int64("timestamp", event.Timestamp).
			Str("sourceId", event.SourceID).
			Str("controlType", event.Control.Type.String()).
			Msg("Routing control event")
		s.handleControlEvent(ctx, event.Control, event.SourceID)
		return
	}

	// NOTE: Removed per-event debug logging here - it caused severe performance
	// issues when debug logging is enabled (hundreds of logs per second for mouse events)

	// IMPORTANT: Prevent feedback loop - don't forward events that came from SSH clients
	if strings.HasPrefix(event.SourceID, "ssh-client-") {
		log.Debug().Str("sourceId", event.SourceID).Msg("Ignoring event from SSH client to prevent feedback loop")
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// If controlling local, do nothing (let input go to local system)
	if s.controllingLocal {
		log.Debug().Msg("Controlling local system, ignoring event")
		return
	}

	// If no active client, ignore
	if s.activeClientID == "" {
		log.Debug().Msg("No active client, ignoring event")
		return
	}

	// Get the active client
	client, exists := s.clients[s.activeClientID]
	if !exists {
		log.Warn().Str("clientID", s.activeClientID).Msg("Active client not found, switching to local")
		go func() {
			if err := s.SwitchToLocal(ctx); err != nil {
				log.Error().Err(err).Msg("Failed to switch to local")
			}
		}()
		return
	}

	// NOTE: Removed per-event "Routing event to client" debug log - it caused performance issues

	// Handle mouse move events with cursor constraints
	if event.MouseMove != nil {
		event = s.handleMouseMoveWithConstraints(ctx, event, client)
		if event == nil {
			return // Movement was fully constrained
		}
	}

	// Handle absolute mouse position events (update our tracking)
	if event.MousePosition != nil {
		s.updateCursorFromAbsolutePosition(event.MousePosition)
	}

	// Translate keyboard events to characters for cross-layout support
	if event.Keyboard != nil {
		event = s.translateKeyboardEvent(ctx, event, client)
	}

	// Send input event to the client via network
	if s.network != nil {
		if err := s.network.SendEventToClient(ctx, client.Address, event); err != nil {
			log.Error().Err(err).Str("clientID", s.activeClientID).Msg("Failed to send input event to client")
		} else {
			s.logInputActivity(ctx, event, client)
		}
	} else {
		log.Error().Msg("No network server available to send events")
	}
}

// handleControlEvent processes control events from clients.
func (s *UseCaseImpl) handleControlEvent(ctx context.Context, controlEvent *domain.ControlEvent, sourceID string) {
	log := zerolog.Ctx(ctx)

	switch controlEvent.Type {
	case domain.ControlClientConfig:
		if controlEvent.ClientConfig != nil {
			s.UpdateClientConfiguration(ctx, controlEvent.ClientConfig, sourceID)
		}

	case domain.ControlRequestControl:
		// Check if we're in emergency cooldown period
		s.mu.RLock()
		inCooldown := time.Since(s.emergencyReleaseTime) < s.emergencyCooldown
		s.mu.RUnlock()

		if inCooldown {
			log.Debug().Str("sourceID", sourceID).Msg("Client requested control during emergency cooldown - ignoring")
			return
		}

		log.Info().Str("sourceID", sourceID).Msg("Client requested control")
		if err := s.SwitchToClient(ctx, sourceID); err != nil {
			log.Error().Err(err).Str("sourceID", sourceID).Msg("Failed to grant control to client")
		}

	case domain.ControlReleaseControl:
		log.Info().Str("sourceID", sourceID).Msg("Client released control")
		if err := s.SwitchToLocal(ctx); err != nil {
			log.Error().Err(err).Str("sourceID", sourceID).Msg("Failed to release control from client")
		}

	default:
		log.Warn().Str("sourceID", sourceID).Str("type", controlEvent.Type.String()).Msg("Unknown control event type")
	}
}

// handleMouseMoveWithConstraints applies cursor constraints to mouse move events.
// Returns the modified event, or nil if the movement was fully constrained.
func (s *UseCaseImpl) handleMouseMoveWithConstraints(ctx context.Context, event *domain.InputEvent, client *domain.Client) *domain.InputEvent {
	log := zerolog.Ctx(ctx)

	cursor, exists := s.clientCursors[s.activeClientID]
	if !exists || len(client.Monitors) == 0 {
		// NOTE: Removed per-event "No cursor state or monitors" debug log - it caused performance issues
		return event
	}

	// Apply relative movement to cursor position
	newX := cursor.X + event.MouseMove.DX
	newY := cursor.Y + event.MouseMove.DY

	// Constrain to client's display bounds
	constrainedX, constrainedY := ConstrainCursorPosition(newX, newY, cursor.Bounds)

	// Check if we hit a boundary
	hitBoundary := newX != constrainedX || newY != constrainedY
	if hitBoundary {
		log.Debug().Msg("Cursor hit boundary")
	}

	// Calculate the actual movement after constraints
	actualDx := constrainedX - cursor.X
	actualDy := constrainedY - cursor.Y

	// Update cursor position
	cursor.X = constrainedX
	cursor.Y = constrainedY

	// If movement was fully constrained, don't send event
	if hitBoundary && actualDx == 0 && actualDy == 0 {
		log.Debug().Msg("Mouse movement fully constrained, not sending event")
		return nil
	}

	// If we hit a boundary, create a new event with constrained movement
	if hitBoundary {
		return &domain.InputEvent{
			Timestamp: event.Timestamp,
			SourceID:  event.SourceID,
			MouseMove: &domain.MouseMoveEvent{
				DX: actualDx,
				DY: actualDy,
			},
		}
	}

	return event
}

// updateCursorFromAbsolutePosition updates cursor tracking from an absolute position event.
func (s *UseCaseImpl) updateCursorFromAbsolutePosition(pos *domain.MousePositionEvent) {
	if cursor, exists := s.clientCursors[s.activeClientID]; exists {
		cursor.X = float64(pos.X)
		cursor.Y = float64(pos.Y)
	}
}

// logInputActivity logs input activity with throttling to avoid spam.
// NOTE: Per-event debug logging was removed from this function as it caused severe
// performance issues when debug logging is enabled (hundreds of logs per second).
func (s *UseCaseImpl) logInputActivity(_ context.Context, _ *domain.InputEvent, client *domain.Client) {
	// Send to UI with throttling
	if s.onActivity != nil {
		now := time.Now()
		s.activityCount++

		// Log activity every 2 seconds or every 50 events
		if now.Sub(s.lastActivityLog) > 2*time.Second || s.activityCount >= 50 {
			summary := fmt.Sprintf("Actively controlling %s (%s) - %d input events sent",
				client.Name, client.Address, s.activityCount)
			s.onActivity("INFO", summary)
			s.lastActivityLog = now
			s.activityCount = 0
		}
	}
}

// translateKeyboardEvent translates a raw keycode to a character using the server's
// keyboard layout. This enables semantic keyboard translation between systems with
// different keyboard layouts (e.g., AZERTY server to QWERTY client).
func (s *UseCaseImpl) translateKeyboardEvent(ctx context.Context, event *domain.InputEvent, client *domain.Client) *domain.InputEvent {
	log := zerolog.Ctx(ctx)

	// Skip if no keyboard layout port available
	if s.keyboardLayout == nil {
		return event
	}

	// Skip if client doesn't have capabilities configured
	if client.Capabilities == nil {
		return event
	}

	// Skip if client doesn't have a different layout (no translation needed)
	// If layouts are the same, just send raw keycodes
	serverLayout := s.getServerKeyboardLayout()
	clientLayout := client.Capabilities.KeyboardLayout
	if clientLayout == "" || clientLayout == serverLayout {
		return event
	}

	kbd := event.Keyboard
	if kbd == nil {
		return event
	}

	// Bounds check for modifiers (uint32 -> uint8)
	if kbd.Modifiers > math.MaxUint8 {
		log.Warn().
			Uint32("modifiers", kbd.Modifiers).
			Msg("Keyboard modifiers value exceeds uint8 range, skipping translation")
		return event
	}

	// Bounds check for key (uint32 -> uint16)
	if kbd.Key > math.MaxUint16 {
		log.Warn().
			Uint32("key", kbd.Key).
			Msg("Keyboard key value exceeds uint16 range, skipping translation")
		return event
	}

	// Calculate current modifiers from the key being processed
	modifiers := domain.KeyModifier(kbd.Modifiers) //nolint:gosec // bounds checked above

	// Translate keycode to character using server's layout
	charPtr := s.keyboardLayout.KeycodeToChar(ctx, uint16(kbd.Key), modifiers, serverLayout) //nolint:gosec // bounds checked above
	if charPtr == nil {
		// Keycode doesn't produce a printable character (modifier/function key)
		// Just send the raw keycode
		return event
	}

	char := *charPtr

	// If translation produced a character, add it to the event
	log.Debug().
		Uint32("keycode", kbd.Key).
		Str("char", string(char)).
		Str("serverLayout", string(serverLayout)).
		Str("clientLayout", string(clientLayout)).
		Msg("Translated keycode to character for cross-layout support")

	// Create a new event with the character field populated
	return &domain.InputEvent{
		Timestamp: event.Timestamp,
		SourceID:  event.SourceID,
		Keyboard: &domain.KeyboardEvent{
			Key:       kbd.Key,
			Pressed:   kbd.Pressed,
			Modifiers: kbd.Modifiers,
			Character: &char,
		},
	}
}

// getServerKeyboardLayout returns the server's configured keyboard layout.
// Falls back to auto-detection if not configured.
func (s *UseCaseImpl) getServerKeyboardLayout() domain.KeyboardLayout {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.config != nil && s.config.Server.KeyboardLayout != "" {
		return s.config.Server.KeyboardLayout
	}

	// Fall back to auto-detection
	if s.keyboardLayout != nil {
		return s.keyboardLayout.DetectLayout(context.Background())
	}

	return domain.LayoutUS
}
