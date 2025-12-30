package out

import (
	"context"

	"github.com/bnema/waymon/internal/domain"
)

// DisplayPort defines the interface for display/monitor detection.
// Implementations handle various backends (wlr-output-management, portal, etc.).
type DisplayPort interface {
	// GetMonitors returns the list of connected monitors.
	GetMonitors(ctx context.Context) ([]domain.Monitor, error)

	// GetCursorPosition returns the current cursor position.
	GetCursorPosition(ctx context.Context) (*domain.CursorPosition, error)

	// Close releases any resources held by the display backend.
	Close() error
}
