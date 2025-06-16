package input

import (
	"context"
	"fmt"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/logger"
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
// For servers: tries evdev first (actual input capture), then falls back to Wayland virtual input
// For clients: Wayland virtual input is used for injection
func CreateBackend() (InputBackend, error) {
	// First, try evdev backend (for server-side input capture)
	if IsEvdevAvailable() {
		logger.Info("Using evdev backend for input capture")
		return NewEvdevCapture(), nil
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
		logger.Info("Using evdev backend for server input capture")

		// Check if devices are configured
		cfg := config.Get()
		if cfg != nil && (cfg.Input.MouseDevice != "" || cfg.Input.KeyboardDevice != "") {
			logger.Infof("Using configured devices - Mouse: %s, Keyboard: %s",
				cfg.Input.MouseDevice, cfg.Input.KeyboardDevice)
			return NewEvdevCaptureWithDevices(cfg.Input.MouseDevice, cfg.Input.KeyboardDevice), nil
		}

		return NewEvdevCapture(), nil
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
