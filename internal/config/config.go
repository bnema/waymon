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


	// Logging configuration
	Logging LoggingConfig `mapstructure:"logging"`

	// Known hosts for quick connections
	Hosts []HostConfig `mapstructure:"hosts"`
}

// ServerConfig contains server-specific settings
type ServerConfig struct {
	Port        int    `mapstructure:"port"`
	BindAddress string `mapstructure:"bind_address"`
	Name        string `mapstructure:"name"`
	MaxClients  int    `mapstructure:"max_clients"`

	// SSH configuration
	SSHHostKeyPath   string   `mapstructure:"ssh_host_key_path"`
	SSHAuthKeysPath  string   `mapstructure:"ssh_authorized_keys_path"`
	SSHWhitelist     []string `mapstructure:"ssh_whitelist"`      // List of allowed SSH key fingerprints
	SSHWhitelistOnly bool     `mapstructure:"ssh_whitelist_only"` // Only allow whitelisted keys
}

// ClientConfig contains client-specific settings
type ClientConfig struct {
	ServerAddress  string        `mapstructure:"server_address"`
	AutoConnect    bool          `mapstructure:"auto_connect"`
	ReconnectDelay int           `mapstructure:"reconnect_delay"`
	EdgeThreshold  int           `mapstructure:"edge_threshold"`
	ScreenPosition string        `mapstructure:"screen_position"` // Deprecated: use EdgeMappings instead
	EdgeMappings   []EdgeMapping `mapstructure:"edge_mappings"`   // Monitor-specific edge mappings
	HotkeyModifier string        `mapstructure:"hotkey_modifier"`
	HotkeyKey      string        `mapstructure:"hotkey_key"`

	// SSH configuration
	SSHPrivateKey string `mapstructure:"ssh_private_key"`
}


// LoggingConfig contains logging settings
type LoggingConfig struct {
	FileLogging bool   `mapstructure:"file_logging"` // Enable/disable file logging
	LogLevel    string `mapstructure:"log_level"`    // Override LOG_LEVEL env var
}

// DeviceInfo stores persistent device identification
type DeviceInfo struct {
	Name       string `mapstructure:"name"`         // Human-readable device name
	ByIDPath   string `mapstructure:"by_id_path"`   // Persistent /dev/input/by-id/ path
	ByPathPath string `mapstructure:"by_path_path"` // Persistent /dev/input/by-path/ path
	VendorID   string `mapstructure:"vendor_id"`    // USB Vendor ID
	ProductID  string `mapstructure:"product_id"`   // USB Product ID
	Phys       string `mapstructure:"phys"`         // Physical location
}

// HostConfig represents a known host for quick connections
type HostConfig struct {
	Name     string `mapstructure:"name"`
	Address  string `mapstructure:"address"`
	Position string `mapstructure:"position"` // left, right, top, bottom
}

// EdgeMapping defines which monitor edge connects to which host
type EdgeMapping struct {
	MonitorID   string `mapstructure:"monitor_id"`  // Monitor ID/name or "primary" for primary monitor, "*" for any
	Edge        string `mapstructure:"edge"`        // "left", "right", "top", "bottom"
	Host        string `mapstructure:"host"`        // Host name or address to connect to
	Description string `mapstructure:"description"` // Optional description
}

var (
	// DefaultConfig provides sensible defaults
	DefaultConfig = Config{
		Server: ServerConfig{
			Port:             52525,
			BindAddress:      "0.0.0.0",
			Name:             getHostname(),
			MaxClients:       1,
			SSHHostKeyPath:   "/etc/waymon/host_key",
			SSHAuthKeysPath:  "/etc/waymon/authorized_keys",
			SSHWhitelist:     []string{},
			SSHWhitelistOnly: true,
		},
		Client: ClientConfig{
			ServerAddress:  "",
			AutoConnect:    false,
			ReconnectDelay: 5,
			EdgeThreshold:  5,
			ScreenPosition: "right", // Default: client is on the right of server
			HotkeyModifier: "ctrl+alt",
			HotkeyKey:      "s",
			SSHPrivateKey:  "",
		},
		Logging: LoggingConfig{
			FileLogging: true,  // Enable file logging by default
			LogLevel:    "",    // Empty means use LOG_LEVEL env var
		},
		Hosts: []HostConfig{},
	}

	// Global config instance
	cfg *Config

	// Override config path if set
	configPathOverride string
)

// SetConfigPath allows overriding the config path
func SetConfigPath(path string) {
	configPathOverride = path
}

// Init initializes the configuration system
func Init() error {
	// Set config name and type
	viper.SetConfigName("waymon")
	viper.SetConfigType("toml")

	// If a specific path is set, use only that
	if configPathOverride != "" {
		viper.SetConfigFile(configPathOverride)
	} else {
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
	}

	// Set defaults - need to set individual fields for proper merging
	viper.SetDefault("server.port", DefaultConfig.Server.Port)
	viper.SetDefault("server.bind_address", DefaultConfig.Server.BindAddress)
	viper.SetDefault("server.name", DefaultConfig.Server.Name)
	viper.SetDefault("server.max_clients", DefaultConfig.Server.MaxClients)
	viper.SetDefault("server.ssh_host_key_path", DefaultConfig.Server.SSHHostKeyPath)
	viper.SetDefault("server.ssh_authorized_keys_path", DefaultConfig.Server.SSHAuthKeysPath)
	viper.SetDefault("server.ssh_whitelist", DefaultConfig.Server.SSHWhitelist)
	viper.SetDefault("server.ssh_whitelist_only", DefaultConfig.Server.SSHWhitelistOnly)

	viper.SetDefault("client.server_address", DefaultConfig.Client.ServerAddress)
	viper.SetDefault("client.auto_connect", DefaultConfig.Client.AutoConnect)
	viper.SetDefault("client.reconnect_delay", DefaultConfig.Client.ReconnectDelay)
	viper.SetDefault("client.edge_threshold", DefaultConfig.Client.EdgeThreshold)
	viper.SetDefault("client.screen_position", DefaultConfig.Client.ScreenPosition)
	viper.SetDefault("client.edge_mappings", DefaultConfig.Client.EdgeMappings)
	viper.SetDefault("client.hotkey_modifier", DefaultConfig.Client.HotkeyModifier)
	viper.SetDefault("client.hotkey_key", DefaultConfig.Client.HotkeyKey)
	viper.SetDefault("client.ssh_private_key", DefaultConfig.Client.SSHPrivateKey)


	viper.SetDefault("logging.file_logging", DefaultConfig.Logging.FileLogging)
	viper.SetDefault("logging.log_level", DefaultConfig.Logging.LogLevel)

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

// Set sets the current configuration (for testing)
func Set(c *Config) {
	cfg = c
}

// Save saves the current configuration to file
func Save() error {
	configPath := GetConfigPath()

	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
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
	// If override is set, use that
	if configPathOverride != "" {
		return configPathOverride
	}

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

// AddSSHKeyToWhitelist adds an SSH key fingerprint to the whitelist
func AddSSHKeyToWhitelist(fingerprint string) error {
	cfg := Get()

	// Check if already whitelisted
	for _, fp := range cfg.Server.SSHWhitelist {
		if fp == fingerprint {
			return fmt.Errorf("key already whitelisted")
		}
	}

	// Add to whitelist
	cfg.Server.SSHWhitelist = append(cfg.Server.SSHWhitelist, fingerprint)
	viper.Set("server.ssh_whitelist", cfg.Server.SSHWhitelist)
	return Save()
}

// RemoveSSHKeyFromWhitelist removes an SSH key fingerprint from the whitelist
func RemoveSSHKeyFromWhitelist(fingerprint string) error {
	cfg := Get()

	// Find and remove
	for i, fp := range cfg.Server.SSHWhitelist {
		if fp == fingerprint {
			cfg.Server.SSHWhitelist = append(cfg.Server.SSHWhitelist[:i], cfg.Server.SSHWhitelist[i+1:]...)
			viper.Set("server.ssh_whitelist", cfg.Server.SSHWhitelist)
			return Save()
		}
	}

	return fmt.Errorf("key not found in whitelist")
}

// IsSSHKeyWhitelisted checks if an SSH key fingerprint is whitelisted
func IsSSHKeyWhitelisted(fingerprint string) bool {
	cfg := Get()

	for _, fp := range cfg.Server.SSHWhitelist {
		if fp == fingerprint {
			return true
		}
	}

	return false
}

// Helper function to get hostname
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "waymon-server"
	}
	return hostname
}
