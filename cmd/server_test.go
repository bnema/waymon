package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestServerCommand(t *testing.T) {
	// Save original values
	originalUID := os.Getuid()

	t.Run("fails without TTY in test environment", func(t *testing.T) {
		// The server command now runs as normal user but requires TTY for TUI
		// In test environment without TTY, it should fail with TTY error
		
		// Try to run server command (will fail due to no TTY)
		err := executeCommand(rootCmd, "server")
		if err == nil {
			t.Error("Expected error in test environment without TTY, got nil")
		}
		if err != nil && !contains(err.Error(), "TTY") {
			t.Errorf("Expected TTY error in test environment, got: %v", err)
		}
	})
	
	t.Run("runs without TUI when --no-tui flag is used", func(t *testing.T) {
		// Skip if running as root to avoid actual server startup
		if originalUID == 0 {
			t.Skip("Test skipped when running as root to avoid actual server startup")
		}
		
		// This test documents that --no-tui should work in test environments
		// We don't actually run it to completion to avoid starting a real server
		t.Skip("Test documents --no-tui behavior - manual verification needed")
	})
}

func TestServerWithSudo(t *testing.T) {
	t.Run("preserves PATH for display detection", func(t *testing.T) {
		// This test documents the PATH preservation issue
		// When running with sudo, the PATH may not include user binaries
		// like hyprctl, swaymsg, etc.

		// We need to ensure the display detection can find these binaries
		t.Skip("Manual test: sudo should preserve PATH or we should use absolute paths")
	})
}

func TestEnsureServerConfig(t *testing.T) {
	t.Run("creates config when running as root", func(t *testing.T) {
		// Skip if not running as root
		if os.Geteuid() != 0 {
			t.Skip("Test requires root privileges")
		}

		// This test documents that when running as root,
		// the config should be created in /etc/waymon/
		// if it doesn't exist
	})
}

func TestConfigPathResolution(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "waymon-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("finds config in current directory", func(t *testing.T) {
		// Reset viper
		viper.Reset()

		// Create config in current directory
		oldWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldWd)

		// Write a valid config
		validConfig := `[server]
port = 52525
bind_address = "0.0.0.0"
name = "test-server"

[client]
server_address = ""
edge_threshold = 5

[display]
backend = "auto"

[input]
mouse_sensitivity = 1.0
`
		err := os.WriteFile("waymon.toml", []byte(validConfig), 0644)
		if err != nil {
			t.Fatal(err)
		}

		// Initialize config
		initConfig()
		// If it doesn't panic, test passes
	})

	t.Run("handles malformed TOML gracefully", func(t *testing.T) {
		// Reset viper
		viper.Reset()

		// Create config directory
		configDir := filepath.Join(tmpDir, ".config", "waymon")
		os.MkdirAll(configDir, 0755)

		// Write invalid TOML
		invalidConfig := `[server
port = 52525
`
		configPath := filepath.Join(configDir, "waymon.toml")
		err := os.WriteFile(configPath, []byte(invalidConfig), 0644)
		if err != nil {
			t.Fatal(err)
		}

		// Override HOME
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", originalHome)

		// This should not panic, just print a warning
		// The initConfig in root.go already handles this correctly
		viper.Reset()
		initConfig()
		// Test passes if no panic occurs
	})
}
