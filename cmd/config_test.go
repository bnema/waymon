package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bnema/waymon/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestConfigInit(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "waymon-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config path for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Reset viper for clean test
	viper.Reset()

	t.Run("creates config file when it doesn't exist", func(t *testing.T) {
		// Initialize config
		err := executeCommand(rootCmd, "config", "init")
		if err != nil {
			t.Errorf("config init failed: %v", err)
		}

		// Check if file was created
		configPath := filepath.Join(tmpDir, ".config", "waymon", "waymon.toml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Error("Config file was not created")
		}
	})

	t.Run("doesn't overwrite existing config without force", func(t *testing.T) {
		// Reset viper
		viper.Reset()
		
		// Try to init again without force
		err := executeCommand(rootCmd, "config", "init")
		if err != nil {
			t.Errorf("config init failed: %v", err)
		}
		// Should not error, just skip
	})

	t.Run("overwrites with force flag", func(t *testing.T) {
		// Reset viper
		viper.Reset()
		
		// Write some content to verify overwrite
		configPath := filepath.Join(tmpDir, ".config", "waymon", "waymon.toml")
		os.WriteFile(configPath, []byte("test = true"), 0644)
		
		// Init with force
		err := executeCommand(rootCmd, "config", "init", "--force")
		if err != nil {
			t.Errorf("config init --force failed: %v", err)
		}
		
		// Read file and check it was overwritten
		content, _ := os.ReadFile(configPath)
		if string(content) == "test = true" {
			t.Error("Config file was not overwritten")
		}
	})
}

func TestConfigShow(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "waymon-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config path for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Reset viper for clean test
	viper.Reset()

	t.Run("shows default config when no file exists", func(t *testing.T) {
		err := executeCommand(rootCmd, "config", "show")
		if err != nil {
			t.Errorf("config show failed: %v", err)
		}
	})
}

func TestConfigValidation(t *testing.T) {
	t.Run("validates TOML syntax", func(t *testing.T) {
		// Create a temporary directory
		tmpDir, err := os.MkdirTemp("", "waymon-test-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		// Create config directory
		configDir := filepath.Join(tmpDir, ".config", "waymon")
		os.MkdirAll(configDir, 0755)

		// Write invalid TOML
		configPath := filepath.Join(configDir, "waymon.toml")
		invalidTOML := `
[server
port = 52525
`
		os.WriteFile(configPath, []byte(invalidTOML), 0644)

		// Override config path
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", originalHome)

		// Reset viper
		viper.Reset()

		// Try to load config
		err = config.Init()
		if err == nil {
			t.Error("Expected error for invalid TOML, got nil")
		}
		if err != nil && !contains(err.Error(), "parsing") {
			t.Errorf("Expected TOML parsing error, got: %v", err)
		}
	})
}

// Helper function to execute cobra commands in tests
func executeCommand(root *cobra.Command, args ...string) error {
	root.SetArgs(args)
	return root.Execute()
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}