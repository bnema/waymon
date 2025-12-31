package domain

import "time"

// ClientStatus represents the current status of a connected client.
type ClientStatus int

const (
	// ClientIdle means the client is connected but not being controlled.
	ClientIdle ClientStatus = iota
	// ClientBeingControlled means the client is currently receiving input.
	ClientBeingControlled
	// ClientDisconnected means the client has disconnected.
	ClientDisconnected
)

// String returns the string representation of ClientStatus.
func (s ClientStatus) String() string {
	switch s {
	case ClientIdle:
		return "idle"
	case ClientBeingControlled:
		return "being_controlled"
	case ClientDisconnected:
		return "disconnected"
	default:
		return "unknown"
	}
}

// ClientCapabilities describes what a client can receive/handle.
type ClientCapabilities struct {
	CanReceiveKeyboard bool
	CanReceiveMouse    bool
	CanReceiveScroll   bool
	WaylandCompositor  string
	UInputVersion      string
	KeyboardLayout     KeyboardLayout // Client's keyboard layout (e.g., "us", "fr")
}

// Client represents a connected client in the server's perspective.
type Client struct {
	ID           string
	Name         string
	Address      string
	Status       ClientStatus
	ConnectedAt  time.Time
	Monitors     []Monitor
	Capabilities *ClientCapabilities
}

// ControlStatus represents the control state from the client's perspective.
type ControlStatus struct {
	BeingControlled bool
	ControllerName  string
}

// ClientConfig is the configuration a client sends to the server on connect.
type ClientConfig struct {
	ClientID     string
	ClientName   string
	Monitors     []Monitor
	Capabilities *ClientCapabilities
}

// ServerInfo represents information about the server.
type ServerInfo struct {
	ID                   string
	Name                 string
	ConnectedClients     []Client
	CurrentlyControlling string
	Capabilities         *ServerCapabilities
}

// ServerCapabilities describes what the server supports.
type ServerCapabilities struct {
	SupportsKeyboard bool
	SupportsMouse    bool
	SupportsScroll   bool
	SupportsHotkeys  bool
}
