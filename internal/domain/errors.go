// Package domain contains pure domain types with no external dependencies.
// This is the core of the application following clean architecture principles.
package domain

import "errors"

// Domain errors - these represent business logic errors that can occur
// throughout the application. They contain no external dependencies.
var (
	// ErrClientNotFound is returned when attempting to operate on a client
	// that doesn't exist in the system.
	ErrClientNotFound = errors.New("client not found")

	// ErrMaxClientsReached is returned when the server has reached its
	// maximum allowed number of connected clients.
	ErrMaxClientsReached = errors.New("maximum clients reached")

	// ErrNotConnected is returned when attempting an operation that requires
	// an active connection, but no connection exists.
	ErrNotConnected = errors.New("not connected")

	// ErrAlreadyConnected is returned when attempting to connect
	// while already connected.
	ErrAlreadyConnected = errors.New("already connected")

	// ErrAuthenticationFailed is returned when SSH authentication fails.
	ErrAuthenticationFailed = errors.New("authentication failed")

	// ErrInputDeviceUnavailable is returned when the required input device
	// (evdev, wayland virtual input, etc.) is not available.
	ErrInputDeviceUnavailable = errors.New("input device unavailable")

	// ErrInvalidEvent is returned when an input event is malformed or
	// cannot be processed.
	ErrInvalidEvent = errors.New("invalid event")

	// ErrConnectionClosed is returned when the connection was closed
	// unexpectedly.
	ErrConnectionClosed = errors.New("connection closed")

	// ErrInvalidSlot is returned when an invalid slot number is specified
	// for client connection.
	ErrInvalidSlot = errors.New("invalid slot number")

	// ErrEmergencyCooldown is returned when a control request is made
	// during the emergency release cooldown period.
	ErrEmergencyCooldown = errors.New("emergency release cooldown active")

	// ErrNoMonitors is returned when the client has no monitors configured.
	ErrNoMonitors = errors.New("no monitors configured")

	// ErrConfigNotFound is returned when the configuration file cannot be found.
	ErrConfigNotFound = errors.New("configuration not found")

	// ErrInvalidConfig is returned when the configuration is invalid.
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrInvalidAction is returned when an invalid action is specified.
	ErrInvalidAction = errors.New("invalid action")
)
