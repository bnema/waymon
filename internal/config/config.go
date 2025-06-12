// Package config handles configuration management using Viper
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	// Server configuration
	Server ServerConfig `mapstructure:"server"`

	// Client configuration
	Client ClientConfig `mapstructure:"client"`

	// Display configuration
	Display DisplayConfig `mapstructure:"display"`

	// Input configuration
	Input InputConfig `mapstructure:"input"`

	// Known hosts for quick connections
	Hosts []HostConfig `mapstructure:"hosts"`
}

// ServerConfig contains server-specific settings
type ServerConfig struct {
	Port        int    `mapstructure:"port"`
	BindAddress string `mapstructure:"bind_address"`
	Name        string `mapstructure:"name"`
	RequireAuth bool   `mapstructure:"require_auth"`
	AuthToken   string `mapstructure:"auth_token"`
	MaxClients  int    `mapstructure:"max_clients"`
	EnableTLS   bool   `mapstructure:"enable_tls"`
	TLSCert     string `mapstructure:"tls_cert"`
	TLSKey      string `mapstructure:"tls_key"`
}

// ClientConfig contains client-specific settings
type ClientConfig struct {
	ServerAddress  string `mapstructure:"server_address"`
	AutoConnect    bool   `mapstructure:"auto_connect"`
	ReconnectDelay int    `mapstructure:"reconnect_delay"`
	EdgeThreshold  int    `mapstructure:"edge_threshold"`
	HotkeyModifier string `mapstructure:"hotkey_modifier"`
	HotkeyKey      string `mapstructure:"hotkey_key"`
	EnableTLS      bool   `mapstructure:"enable_tls"`
	TLSSkipVerify  bool   `mapstructure:"tls_skip_verify"`
}

// DisplayConfig contains display detection settings
type DisplayConfig struct {
	RefreshInterval int    `mapstructure:"refresh_interval"`
	Backend         string `mapstructure:"backend"`
	CursorTracking  bool   `mapstructure:"cursor_tracking"`
}

// InputConfig contains input handling settings
type InputConfig struct {
	MouseSensitivity float64 `mapstructure:"mouse_sensitivity"`
	ScrollSpeed      float64 `mapstructure:"scroll_speed"`
	EnableKeyboard   bool    `mapstructure:"enable_keyboard"`
	KeyboardLayout   string  `mapstructure:"keyboard_layout"`
}

// HostConfig represents a known host for quick connections
type HostConfig struct {
	Name      string `mapstructure:"name"`
	Address   string `mapstructure:"address"`
	Position  string `mapstructure:"position"` // left, right, top, bottom
	AuthToken string `mapstructure:"auth_token"`
}

var (
	// DefaultConfig provides sensible defaults
	DefaultConfig = Config{
		Server: ServerConfig{
			Port:        52525,
			BindAddress: "0.0.0.0",
			Name:        getHostname(),
			RequireAuth: false,
			AuthToken:   "",
			MaxClients:  1,
			EnableTLS:   false,
			TLSCert:     "",
			TLSKey:      "",
		},
		Client: ClientConfig{
			ServerAddress:  "",
			AutoConnect:    false,
			ReconnectDelay: 5,
			EdgeThreshold:  5,
			HotkeyModifier: "ctrl+alt",
			HotkeyKey:      "s",
			EnableTLS:      false,
			TLSSkipVerify:  false,
		},
		Display: DisplayConfig{
			RefreshInterval: 5,
			Backend:         "auto",
			CursorTracking:  true,
		},
		Input: InputConfig{
			MouseSensitivity: 1.0,
			ScrollSpeed:      1.0,
			EnableKeyboard:   true,
			KeyboardLayout:   "us",
		},
		Hosts: []HostConfig{},
	}

	// Global config instance
	cfg *Config
)

// Init initializes the configuration system
func Init() error {
	// Set config name and type
	viper.SetConfigName("waymon")
	viper.SetConfigType("toml")

	// Add config paths in order of precedence
	viper.AddConfigPath("/etc/waymon") // System config directory (primary)

	// If running with sudo, try the real user's config
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		userConfigPath := fmt.Sprintf("/home/%s/.config/waymon", sudoUser)
		viper.AddConfigPath(userConfigPath)
	} else if home := os.Getenv("HOME"); home != "" && home != "/root" {
		// Normal user config
		viper.AddConfigPath(filepath.Join(home, ".config", "waymon"))
	}

	viper.AddConfigPath(".") // Current directory (lowest priority)

	// Set defaults
	viper.SetDefault("server", DefaultConfig.Server)
	viper.SetDefault("client", DefaultConfig.Client)
	viper.SetDefault("display", DefaultConfig.Display)
	viper.SetDefault("input", DefaultConfig.Input)
	viper.SetDefault("hosts", DefaultConfig.Hosts)

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, use defaults
	}

	// Unmarshal config
	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("unable to unmarshal config: %w", err)
	}

	return nil
}

// Get returns the current configuration
func Get() *Config {
	if cfg == nil {
		// Return defaults if not initialized
		return &DefaultConfig
	}
	return cfg
}

// Save saves the current configuration to file
func Save() error {
	configPath := GetConfigPath()

	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		// If we can't create it (e.g., /etc/waymon needs sudo), provide helpful message
		if os.IsPermission(err) && strings.Contains(configPath, "/etc/") {
			return fmt.Errorf("failed to create config directory %s: permission denied. Try running with sudo", dir)
		}
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config
	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// GetConfigPath returns the path to the config file
func GetConfigPath() string {
	// Check if config file is already loaded
	if viper.ConfigFileUsed() != "" {
		return viper.ConfigFileUsed()
	}

	// For servers/sudo, prefer system config
	if os.Getuid() == 0 || os.Getenv("SUDO_USER") != "" {
		return "/etc/waymon/waymon.toml"
	}

	// For regular users, use user config directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "/etc/waymon/waymon.toml"
	}

	return filepath.Join(home, ".config", "waymon", "waymon.toml")
}

// AddHost adds a new host to the configuration
func AddHost(host HostConfig) error {
	cfg := Get()

	// Check if host already exists
	for i, h := range cfg.Hosts {
		if h.Name == host.Name {
			// Update existing host
			cfg.Hosts[i] = host
			viper.Set("hosts", cfg.Hosts)
			return Save()
		}
	}

	// Add new host
	cfg.Hosts = append(cfg.Hosts, host)
	viper.Set("hosts", cfg.Hosts)
	return Save()
}

// RemoveHost removes a host from the configuration
func RemoveHost(name string) error {
	cfg := Get()

	// Find and remove host
	for i, h := range cfg.Hosts {
		if h.Name == name {
			cfg.Hosts = append(cfg.Hosts[:i], cfg.Hosts[i+1:]...)
			viper.Set("hosts", cfg.Hosts)
			return Save()
		}
	}

	return fmt.Errorf("host %s not found", name)
}

// GetHost returns a host configuration by name
func GetHost(name string) (*HostConfig, error) {
	cfg := Get()

	for _, h := range cfg.Hosts {
		if h.Name == name {
			return &h, nil
		}
	}

	return nil, fmt.Errorf("host %s not found", name)
}

// ListHosts returns all configured hosts
func ListHosts() []HostConfig {
	cfg := Get()
	return cfg.Hosts
}

// UpdateServer updates server configuration
func UpdateServer(serverCfg ServerConfig) error {
	viper.Set("server", serverCfg)
	cfg.Server = serverCfg
	return Save()
}

// UpdateClient updates client configuration
func UpdateClient(clientCfg ClientConfig) error {
	viper.Set("client", clientCfg)
	cfg.Client = clientCfg
	return Save()
}

// Helper function to get hostname
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "waymon-server"
	}
	return hostname
}
