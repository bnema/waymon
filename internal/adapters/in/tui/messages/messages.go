// Package messages provides shared message types for TUI components.
package messages

import (
	"github.com/bnema/waymon/internal/domain"
)

// Server events - messages sent from the server use case.

// ClientConnectedMsg is sent when a new client connects.
type ClientConnectedMsg struct {
	Client domain.Client
}

// ClientDisconnectedMsg is sent when a client disconnects.
type ClientDisconnectedMsg struct {
	ClientID string
}

// ClientUpdatedMsg is sent when client info is updated.
type ClientUpdatedMsg struct {
	Client domain.Client
}

// ControlSwitchedMsg is sent when control switches between local and client.
type ControlSwitchedMsg struct {
	ActiveClientID string
	IsLocal        bool
}

// ClientListUpdatedMsg is sent when the full client list changes.
type ClientListUpdatedMsg struct {
	Clients []domain.Client
}

// Client events - messages sent from the client use case.

// ConnectionStateMsg is sent when connection state changes.
type ConnectionStateMsg struct {
	Connected bool
	Error     error
}

// ControlStatusChangedMsg is sent when control status changes.
type ControlStatusChangedMsg struct {
	Status domain.ControlStatus
}

// ServerInfoReceivedMsg is sent when server info is received.
type ServerInfoReceivedMsg struct {
	Info domain.ServerInfo
}

// Generic messages.

// ErrorMsg is sent when an error occurs.
type ErrorMsg struct {
	Err error
}

// TickMsg is sent for periodic updates (used with tea.Every).
type TickMsg struct{}

// WindowSizeMsg is sent when the terminal window is resized.
// Note: Bubble Tea has its own tea.WindowSizeMsg, this is a convenience wrapper.
type WindowSizeMsg struct {
	Width  int
	Height int
}

// ActivityMsg is sent for activity logging.
type ActivityMsg struct {
	Level   string
	Message string
}

// QuitMsg is sent to request the application to quit.
type QuitMsg struct{}

// Input event messages - for mouse/keyboard events being forwarded.

// InputEventMsg wraps a domain input event.
type InputEventMsg struct {
	Event *domain.InputEvent
}

// Monitor messages.

// MonitorsDetectedMsg is sent when monitors are detected.
type MonitorsDetectedMsg struct {
	Monitors []domain.Monitor
}

// CursorPositionMsg is sent when cursor position updates.
type CursorPositionMsg struct {
	Position domain.CursorPosition
}
