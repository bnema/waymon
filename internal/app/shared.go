// Package app provides the application wiring layer that connects all components.
// This file contains shared utilities used by both client and server runners.
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bnema/waymon/internal/adapters/out/config"
	"github.com/bnema/waymon/internal/domain"
)

// initializeConfig creates a config file with smart defaults if it doesn't exist.
// Returns nil if config already exists or was successfully created.
func initializeConfig(ctx context.Context, repo *config.ViperRepository) error {
	if repo.Exists() {
		return nil
	}

	// Load will apply defaults
	cfg, err := repo.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load defaults: %w", err)
	}

	// Create config directory if needed
	configPath := repo.GetConfigPath()
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	if err := repo.Save(ctx, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// normalizeServerAddress ensures the server address has a port.
// If no port is specified, appends the default port.
func normalizeServerAddress(addr string) string {
	if addr == "" {
		return ""
	}

	// Check if port is already specified
	if strings.Contains(addr, ":") {
		return addr
	}

	return fmt.Sprintf("%s:%d", addr, domain.DefaultServerPort)
}
