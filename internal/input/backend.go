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
		if cfg != nil {
			var mousePath, keyboardPath string
			
			// Try to resolve persistent device info first
			if cfg.Input.MouseDeviceInfo != nil {
				deviceInfo := &PersistentDeviceInfo{
					Name:       cfg.Input.MouseDeviceInfo.Name,
					ByIDPath:   cfg.Input.MouseDeviceInfo.ByIDPath,
					ByPathPath: cfg.Input.MouseDeviceInfo.ByPathPath,
					VendorID:   cfg.Input.MouseDeviceInfo.VendorID,
					ProductID:  cfg.Input.MouseDeviceInfo.ProductID,
					Phys:       cfg.Input.MouseDeviceInfo.Phys,
				}
				if path, err := deviceInfo.ResolveToEventPath(); err == nil {
					mousePath = path
					logger.Infof("Resolved mouse device '%s' to %s", deviceInfo.Name, path)
				} else {
					logger.Warnf("Could not resolve mouse device '%s': %v", deviceInfo.Name, err)
				}
			}
			
			if cfg.Input.KeyboardDeviceInfo != nil {
				deviceInfo := &PersistentDeviceInfo{
					Name:       cfg.Input.KeyboardDeviceInfo.Name,
					ByIDPath:   cfg.Input.KeyboardDeviceInfo.ByIDPath,
					ByPathPath: cfg.Input.KeyboardDeviceInfo.ByPathPath,
					VendorID:   cfg.Input.KeyboardDeviceInfo.VendorID,
					ProductID:  cfg.Input.KeyboardDeviceInfo.ProductID,
					Phys:       cfg.Input.KeyboardDeviceInfo.Phys,
				}
				if path, err := deviceInfo.ResolveToEventPath(); err == nil {
					keyboardPath = path
					logger.Infof("Resolved keyboard device '%s' to %s", deviceInfo.Name, path)
				} else {
					logger.Warnf("Could not resolve keyboard device '%s': %v", deviceInfo.Name, err)
				}
			}
			
			
			if mousePath != "" || keyboardPath != "" {
				logger.Infof("Using configured devices - Mouse: %s, Keyboard: %s", mousePath, keyboardPath)
				return NewEvdevCaptureWithDevices(mousePath, keyboardPath), nil
			}
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
