package input

import (
	"context"
	"fmt"
	"os"

	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/protocol"
	"github.com/gvalkov/golang-evdev"
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
// For servers: tries evdev first (actual input capture), then falls back to Wayland virtual input
// For clients: Wayland virtual input is used for injection
func CreateBackend() (InputBackend, error) {
	// First, try all-devices capture backend (for server-side input capture)
	if IsEvdevAvailable() {
		logger.Info("Using all-devices capture backend for input capture")
		return NewAllDevicesCapture(), nil
	}

	// Fall back to Wayland virtual input (primarily for client-side injection)
	if backend, err := NewWaylandVirtualInput(); err == nil {
		logger.Info("Using Wayland virtual input backend")
		return backend, nil
	}

	return nil, fmt.Errorf("no suitable input backend available")
}

// CreateServerBackend creates an input backend specifically for server mode
// Always tries evdev first since servers need actual input capture
func CreateServerBackend() (InputBackend, error) {
	// For servers, we MUST have evdev for actual input capture
	if IsEvdevAvailable() {
		logger.Info("Using all-devices capture backend for server input capture")
		return NewAllDevicesCapture(), nil
	}

	return nil, fmt.Errorf("evdev not available - server requires evdev for input capture. " +
		"Make sure you have access to /dev/input/event* devices")
}

// CreateClientBackend creates an input backend specifically for client mode
// Uses Wayland virtual input for injection
func CreateClientBackend() (InputBackend, error) {
	// For clients, use Wayland virtual input for injection
	if backend, err := NewWaylandVirtualInput(); err == nil {
		logger.Info("Using Wayland virtual input backend for client injection")
		return backend, nil
	}

	return nil, fmt.Errorf("wayland virtual input protocols not supported by compositor")
}

// IsEvdevAvailable checks if evdev is available on this system
func IsEvdevAvailable() bool {
	// Check if /dev/input directory exists
	if _, err := os.Stat("/dev/input"); os.IsNotExist(err) {
		return false
	}

	// Try to list input devices
	devices, err := evdev.ListInputDevices("/dev/input/event*")
	if err != nil {
		return false
	}

	return len(devices) > 0
}
