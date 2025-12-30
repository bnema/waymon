// Package out defines output port interfaces (driven adapters).
// These interfaces define what the application needs from external systems.
package out

import (
	"context"

	"github.com/bnema/waymon/internal/domain"
)

// ConfigRepository defines the interface for configuration persistence.
// Implementations handle loading/saving config from various sources (files, env, etc.).
type ConfigRepository interface {
	// Load loads the configuration from the underlying storage.
	Load(ctx context.Context) (*domain.Config, error)

	// Save persists the configuration to the underlying storage.
	Save(ctx context.Context, config *domain.Config) error

	// GetConfigPath returns the path to the configuration file.
	GetConfigPath() string

	// SetConfigPath sets the path to the configuration file.
	SetConfigPath(path string)
}
