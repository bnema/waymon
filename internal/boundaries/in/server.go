// Package in defines input port interfaces (what the application exposes).
// These interfaces are implemented by usecases and called by adapters/in.
package in

import (
	"context"

	"github.com/bnema/waymon/internal/boundaries/out"
	"github.com/bnema/waymon/internal/domain"
)

// ServerUseCase defines the interface for server-side business logic.
// Implemented by usecase/server and called by adapters/in (CLI, TUI, IPC).
type ServerUseCase interface {
	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	StartNetworking(ctx context.Context) error

	// Client management
	RegisterClient(ctx context.Context, id, name, address string)
	UnregisterClient(ctx context.Context, id string)
	GetConnectedClients(ctx context.Context) []domain.Client
	GetActiveClient(ctx context.Context) *domain.Client

	// Control switching
	SwitchToClient(ctx context.Context, clientID string) error
	SwitchToLocal(ctx context.Context) error
	SwitchToNext(ctx context.Context) error
	SwitchToPrevious(ctx context.Context) error
	ConnectToSlot(ctx context.Context, slot int32) error
	IsControllingLocal(ctx context.Context) bool

	// Event handling
	HandleInputEvent(ctx context.Context, event *domain.InputEvent)

	// Callbacks
	SetOnActivity(callback func(level, message string))

	// Shutdown
	NotifyShutdown(ctx context.Context)

	// Accessors for adapters
	GetNetworkServer() out.NetworkServerPort
	GetSSHHostKeyPath() string
	GetSSHAuthKeysPath() string
}
