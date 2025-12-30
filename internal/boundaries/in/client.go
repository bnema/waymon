package in

import (
	"context"

	"github.com/bnema/waymon/internal/domain"
)

// ClientUseCase defines the interface for client-side business logic.
// Implemented by usecase/client and called by adapters/in (CLI, TUI).
type ClientUseCase interface {
	// Lifecycle
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	IsConnected() bool

	// Status
	GetControlStatus(ctx context.Context) domain.ControlStatus

	// Callbacks
	SetOnControlChanged(callback func(status domain.ControlStatus))
	SetOnConnectionStateChanged(callback func(connected bool, serverName string))
}
