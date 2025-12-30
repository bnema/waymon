// Package display provides display/monitor detection implementations.
package display

import (
	"context"
	"fmt"

	"github.com/bnema/waymon/internal/domain"
	"github.com/rs/zerolog"
)

// Backend interface for different display detection methods.
type Backend interface {
	GetMonitors() ([]*domain.Monitor, error)
	GetCursorPosition() (x, y int32, err error)
	Close() error
}

// Adapter implements the DisplayPort interface using various backends.
type Adapter struct {
	backend Backend
}

// New creates a new display adapter by trying different backends in order of preference.
func New(ctx context.Context) (*Adapter, error) {
	log := zerolog.Ctx(ctx)
	log.Debug().Msg("creating display adapter")

	// Try different backends in order of preference
	backends := []struct {
		name   string
		create func(context.Context) (Backend, error)
	}{
		{"sudo", newSudoBackend},
		{"wlr-randr", newWlrRandrBackend},
		// Add more backends here as needed
	}

	var backend Backend
	var lastErr error

	for _, b := range backends {
		log.Debug().Str("backend", b.name).Msg("trying backend")

		var err error
		backend, err = b.create(ctx)
		if err != nil {
			log.Debug().Str("backend", b.name).Err(err).Msg("backend failed")
			lastErr = err
			continue
		}

		log.Debug().Str("backend", b.name).Msg("backend created successfully")
		break
	}

	if backend == nil {
		return nil, fmt.Errorf("no display backend available: %w", lastErr)
	}

	return &Adapter{backend: backend}, nil
}

// GetMonitors returns the list of connected monitors.
func (a *Adapter) GetMonitors(ctx context.Context) ([]domain.Monitor, error) {
	log := zerolog.Ctx(ctx)
	log.Debug().Msg("getting monitors")

	monitors, err := a.backend.GetMonitors()
	if err != nil {
		log.Error().Err(err).Msg("failed to get monitors")
		return nil, err
	}

	// Convert pointers to values
	result := make([]domain.Monitor, len(monitors))
	for i, m := range monitors {
		result[i] = *m
	}

	log.Debug().Int("count", len(result)).Msg("monitors retrieved")
	return result, nil
}

// GetCursorPosition returns the current cursor position.
func (a *Adapter) GetCursorPosition(ctx context.Context) (*domain.CursorPosition, error) {
	log := zerolog.Ctx(ctx)
	log.Debug().Msg("getting cursor position")

	x, y, err := a.backend.GetCursorPosition()
	if err != nil {
		log.Debug().Err(err).Msg("cursor position not available")
		return nil, err
	}

	return &domain.CursorPosition{
		X: float64(x),
		Y: float64(y),
	}, nil
}

// Close releases any resources held by the display backend.
func (a *Adapter) Close() error {
	if a.backend != nil {
		return a.backend.Close()
	}
	return nil
}

// determinePrimaryMonitor sets the primary monitor based on position.
// The monitor at position (0,0) is considered primary, with fallback to first monitor.
func determinePrimaryMonitor(monitors []*domain.Monitor) {
	// Reset all monitors to non-primary
	for _, monitor := range monitors {
		monitor.Primary = false
	}

	// Determine primary monitor - the one at position (0,0) should be primary
	primarySet := false
	for _, monitor := range monitors {
		if monitor.X == 0 && monitor.Y == 0 {
			monitor.Primary = true
			primarySet = true
			break
		}
	}

	// Fallback to first monitor if no monitor is at (0,0)
	if !primarySet && len(monitors) > 0 {
		monitors[0].Primary = true
	}
}
