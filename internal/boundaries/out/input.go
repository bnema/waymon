package out

import (
	"context"

	"github.com/bnema/waymon/internal/domain"
)

// InputCapturePort defines the interface for capturing input events.
// Used by the server to capture mouse/keyboard events from local devices.
type InputCapturePort interface {
	// Start begins capturing input events.
	Start(ctx context.Context) error

	// Stop stops capturing input events and releases devices.
	Stop() error

	// SetTarget sets the target client ID for forwarding events.
	// Empty string means control local system (no forwarding, release grab).
	SetTarget(clientID string) error

	// SetEventCallback sets the callback for captured input events.
	SetEventCallback(callback func(*domain.InputEvent))
}

// InputInjectionPort defines the interface for injecting input events.
// Used by the client to inject received mouse/keyboard events.
type InputInjectionPort interface {
	// Start initializes the input injection system.
	Start(ctx context.Context) error

	// Stop stops the input injection system.
	Stop() error

	// InjectMouseMove injects a relative mouse movement.
	InjectMouseMove(ctx context.Context, dx, dy float64) error

	// InjectMousePosition injects an absolute mouse position.
	InjectMousePosition(ctx context.Context, x, y int32) error

	// InjectMouseButton injects a mouse button press or release.
	InjectMouseButton(ctx context.Context, button uint32, pressed bool) error

	// InjectMouseScroll injects a scroll event.
	InjectMouseScroll(ctx context.Context, dx, dy float64, scrollType domain.ScrollType) error

	// InjectKeyEvent injects a keyboard key event.
	InjectKeyEvent(ctx context.Context, key uint32, pressed bool, modifiers uint32) error

	// SetExclusiveCapture enables or disables exclusive input capture.
	SetExclusiveCapture(ctx context.Context, enabled bool) error
}
