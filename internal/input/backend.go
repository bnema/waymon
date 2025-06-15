package input

import (
	"context"
	"fmt"

	"github.com/bnema/waymon/internal/protocol"
)

// InputBackend represents an input capture backend
type InputBackend interface {
	// Start begins capturing input events
	Start(ctx context.Context) error

	// Stop stops capturing input events
	Stop() error

	// SetTarget sets the target client ID for forwarding events
	// Empty string means control local system (no forwarding)
	SetTarget(clientID string) error

	// OnInputEvent sets the callback for captured input events
	OnInputEvent(callback func(*protocol.InputEvent))
}

// CreateBackend creates an appropriate input backend based on availability
// Uses Wayland virtual input protocols for wlroots-based compositors (Hyprland, Sway)
func CreateBackend() (InputBackend, error) {
	// Use Wayland virtual input for Hyprland/wlroots compositors
	if backend, err := NewWaylandVirtualInput(); err == nil {
		return backend, nil
	}

	return nil, fmt.Errorf("no suitable input backend available - Wayland virtual input protocols not supported by compositor")
}
