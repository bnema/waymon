// Package config provides a viper-based implementation of the ConfigRepository interface.
package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bnema/waymon/internal/domain"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

// viperConfig holds the internal configuration structure with mapstructure tags.
// This maps to the domain.Config but includes serialization tags for viper.
type viperConfig struct {
	Server  viperServerConfig  `mapstructure:"server"`
	Client  viperClientConfig  `mapstructure:"client"`
	Logging viperLoggingConfig `mapstructure:"logging"`
	Hosts   []viperHostConfig  `mapstructure:"hosts"`
}

type viperServerConfig struct {
	Port             int      `mapstructure:"port"`
	BindAddress      string   `mapstructure:"bind_address"`
	Name             string   `mapstructure:"name"`
	MaxClients       int      `mapstructure:"max_clients"`
	SSHHostKeyPath   string   `mapstructure:"ssh_host_key_path"`
	SSHAuthKeysPath  string   `mapstructure:"ssh_authorized_keys_path"`
	SSHWhitelist     []string `mapstructure:"ssh_whitelist"`
	SSHWhitelistOnly bool     `mapstructure:"ssh_whitelist_only"`
}

type viperClientConfig struct {
	ServerAddress  string             `mapstructure:"server_address"`
	AutoConnect    bool               `mapstructure:"auto_connect"`
	ReconnectDelay int                `mapstructure:"reconnect_delay"`
	EdgeThreshold  int                `mapstructure:"edge_threshold"`
	ScreenPosition string             `mapstructure:"screen_position"`
	EdgeMappings   []viperEdgeMapping `mapstructure:"edge_mappings"`
	HotkeyModifier string             `mapstructure:"hotkey_modifier"`
	HotkeyKey      string             `mapstructure:"hotkey_key"`
	SSHPrivateKey  string             `mapstructure:"ssh_private_key"`
}

type viperLoggingConfig struct {
	FileLogging bool   `mapstructure:"file_logging"`
	LogLevel    string `mapstructure:"log_level"`
	LogDir      string `mapstructure:"log_dir"`
}

type viperHostConfig struct {
	Name     string `mapstructure:"name"`
	Address  string `mapstructure:"address"`
	Position string `mapstructure:"position"`
}

type viperEdgeMapping struct {
	MonitorID   string `mapstructure:"monitor_id"`
	Edge        string `mapstructure:"edge"`
	Host        string `mapstructure:"host"`
	Description string `mapstructure:"description"`
}

// ViperRepository implements the ConfigRepository interface using viper.
type ViperRepository struct {
	v          *viper.Viper
	configPath string
}

// NewViperRepository creates a new viper-based configuration repository.
func NewViperRepository() *ViperRepository {
	return &ViperRepository{
		v: viper.New(),
	}
}

// Load loads the configuration from the underlying storage.
func (r *ViperRepository) Load(ctx context.Context) (*domain.Config, error) {
	log := zerolog.Ctx(ctx)
	log.Debug().Msg("loading configuration")

	// Set config name and type
	r.v.SetConfigName("waymon")
	r.v.SetConfigType("toml")

	// If a specific path is set, use only that
	if r.configPath != "" {
		log.Debug().Str("path", r.configPath).Msg("using override config path")
		r.v.SetConfigFile(r.configPath)
	} else {
		// Add config paths in order of precedence
		r.v.AddConfigPath("/etc/waymon") // System config directory (primary)

		// If running with sudo, try the real user's config
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
			userConfigPath := fmt.Sprintf("/home/%s/.config/waymon", sudoUser)
			r.v.AddConfigPath(userConfigPath)
			log.Debug().Str("path", userConfigPath).Msg("added sudo user config path")
		} else if home := os.Getenv("HOME"); home != "" && home != "/root" {
			// Normal user config
			userPath := filepath.Join(home, ".config", "waymon")
			r.v.AddConfigPath(userPath)
			log.Debug().Str("path", userPath).Msg("added user config path")
		}

		r.v.AddConfigPath(".") // Current directory (lowest priority)
	}

	// Set defaults
	r.setDefaults()

	// Read config file if it exists
	if err := r.v.ReadInConfig(); err != nil {
		// Check if it's a "file not found" error
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Debug().Msg("config file not found, using defaults")
		} else if os.IsNotExist(err) {
			// When SetConfigFile is used with a non-existent file, it returns os.PathError
			log.Debug().Msg("config file not found, using defaults")
		} else {
			log.Error().Err(err).Msg("failed to read config file")
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	} else {
		log.Debug().Str("file", r.v.ConfigFileUsed()).Msg("loaded config file")
	}

	// Unmarshal to internal config
	var vc viperConfig
	if err := r.v.Unmarshal(&vc); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal config")
		return nil, fmt.Errorf("unable to unmarshal config: %w", err)
	}

	// Convert to domain config
	cfg := r.toDomain(&vc)
	log.Debug().Msg("configuration loaded successfully")

	return cfg, nil
}

// Save persists the configuration to the underlying storage.
func (r *ViperRepository) Save(ctx context.Context, config *domain.Config) error {
	log := zerolog.Ctx(ctx)
	log.Debug().Msg("saving configuration")

	configPath := r.GetConfigPath()

	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		if os.IsPermission(err) && strings.Contains(configPath, "/etc/") {
			log.Error().Err(err).Str("dir", dir).Msg("permission denied creating config directory")
			return fmt.Errorf("failed to create config directory %s: permission denied. Try running with sudo", dir)
		}
		log.Error().Err(err).Str("dir", dir).Msg("failed to create config directory")
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Update viper with current config
	r.fromDomain(config)

	// Write config
	if err := r.v.WriteConfigAs(configPath); err != nil {
		log.Error().Err(err).Str("path", configPath).Msg("failed to write config")
		return fmt.Errorf("failed to write config: %w", err)
	}

	log.Debug().Str("path", configPath).Msg("configuration saved successfully")
	return nil
}

// GetConfigPath returns the path to the configuration file.
func (r *ViperRepository) GetConfigPath() string {
	// If override is set, use that
	if r.configPath != "" {
		return r.configPath
	}

	// Check if config file is already loaded
	if r.v.ConfigFileUsed() != "" {
		return r.v.ConfigFileUsed()
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

// SetConfigPath sets the path to the configuration file.
func (r *ViperRepository) SetConfigPath(path string) {
	r.configPath = path
}

// Exists returns true if the configuration file exists on disk.
func (r *ViperRepository) Exists() bool {
	path := r.GetConfigPath()
	_, err := os.Stat(path)
	return err == nil
}

// setDefaults sets the default configuration values.
func (r *ViperRepository) setDefaults() {
	hostname := getHostname()

	// Server defaults
	r.v.SetDefault("server.port", 52525)
	r.v.SetDefault("server.bind_address", "0.0.0.0")
	r.v.SetDefault("server.name", hostname)
	r.v.SetDefault("server.max_clients", 1)
	r.v.SetDefault("server.ssh_host_key_path", "/etc/waymon/host_key")
	r.v.SetDefault("server.ssh_authorized_keys_path", "/etc/waymon/authorized_keys")
	r.v.SetDefault("server.ssh_whitelist", []string{})
	r.v.SetDefault("server.ssh_whitelist_only", true)

	// Client defaults
	r.v.SetDefault("client.server_address", "")
	r.v.SetDefault("client.auto_connect", false)
	r.v.SetDefault("client.reconnect_delay", 5)
	r.v.SetDefault("client.edge_threshold", 5)
	r.v.SetDefault("client.screen_position", "right")
	r.v.SetDefault("client.edge_mappings", []viperEdgeMapping{})
	r.v.SetDefault("client.hotkey_modifier", "ctrl+alt")
	r.v.SetDefault("client.hotkey_key", "s")
	r.v.SetDefault("client.ssh_private_key", getDefaultSSHKeyPath())

	// Logging defaults
	r.v.SetDefault("logging.file_logging", true)
	r.v.SetDefault("logging.log_level", "")
	r.v.SetDefault("logging.log_dir", "")

	// Hosts defaults
	r.v.SetDefault("hosts", []viperHostConfig{})
}

// toDomain converts internal viper config to domain config.
func (r *ViperRepository) toDomain(vc *viperConfig) *domain.Config {
	// Convert edge mappings
	edgeMappings := make([]domain.EdgeMapping, len(vc.Client.EdgeMappings))
	for i, em := range vc.Client.EdgeMappings {
		edgeMappings[i] = domain.EdgeMapping{
			MonitorID:   em.MonitorID,
			Edge:        em.Edge,
			Host:        em.Host,
			Description: em.Description,
		}
	}

	// Convert hosts
	hosts := make([]domain.HostConfig, len(vc.Hosts))
	for i, h := range vc.Hosts {
		hosts[i] = domain.HostConfig{
			Name:     h.Name,
			Address:  h.Address,
			Position: h.Position,
		}
	}

	return &domain.Config{
		Server: domain.ServerCfg{
			Port:             vc.Server.Port,
			BindAddress:      vc.Server.BindAddress,
			Name:             vc.Server.Name,
			MaxClients:       vc.Server.MaxClients,
			SSHHostKeyPath:   vc.Server.SSHHostKeyPath,
			SSHAuthKeysPath:  vc.Server.SSHAuthKeysPath,
			SSHWhitelist:     vc.Server.SSHWhitelist,
			SSHWhitelistOnly: vc.Server.SSHWhitelistOnly,
		},
		Client: domain.ClientCfg{
			ServerAddress:  vc.Client.ServerAddress,
			AutoConnect:    vc.Client.AutoConnect,
			ReconnectDelay: vc.Client.ReconnectDelay,
			EdgeThreshold:  vc.Client.EdgeThreshold,
			ScreenPosition: vc.Client.ScreenPosition,
			EdgeMappings:   edgeMappings,
			HotkeyModifier: vc.Client.HotkeyModifier,
			HotkeyKey:      vc.Client.HotkeyKey,
			SSHPrivateKey:  vc.Client.SSHPrivateKey,
		},
		Logging: domain.LoggingConfig{
			FileLogging: vc.Logging.FileLogging,
			LogLevel:    vc.Logging.LogLevel,
			LogDir:      vc.Logging.LogDir,
		},
		Hosts: hosts,
	}
}

// fromDomain updates viper values from domain config.
func (r *ViperRepository) fromDomain(cfg *domain.Config) {
	// Server
	r.v.Set("server.port", cfg.Server.Port)
	r.v.Set("server.bind_address", cfg.Server.BindAddress)
	r.v.Set("server.name", cfg.Server.Name)
	r.v.Set("server.max_clients", cfg.Server.MaxClients)
	r.v.Set("server.ssh_host_key_path", cfg.Server.SSHHostKeyPath)
	r.v.Set("server.ssh_authorized_keys_path", cfg.Server.SSHAuthKeysPath)
	r.v.Set("server.ssh_whitelist", cfg.Server.SSHWhitelist)
	r.v.Set("server.ssh_whitelist_only", cfg.Server.SSHWhitelistOnly)

	// Client
	r.v.Set("client.server_address", cfg.Client.ServerAddress)
	r.v.Set("client.auto_connect", cfg.Client.AutoConnect)
	r.v.Set("client.reconnect_delay", cfg.Client.ReconnectDelay)
	r.v.Set("client.edge_threshold", cfg.Client.EdgeThreshold)
	r.v.Set("client.screen_position", cfg.Client.ScreenPosition)
	r.v.Set("client.hotkey_modifier", cfg.Client.HotkeyModifier)
	r.v.Set("client.hotkey_key", cfg.Client.HotkeyKey)
	r.v.Set("client.ssh_private_key", cfg.Client.SSHPrivateKey)

	// Convert edge mappings - use maps to ensure correct TOML keys
	edgeMappings := make([]map[string]string, len(cfg.Client.EdgeMappings))
	for i, em := range cfg.Client.EdgeMappings {
		edgeMappings[i] = map[string]string{
			"monitor_id":  em.MonitorID,
			"edge":        em.Edge,
			"host":        em.Host,
			"description": em.Description,
		}
	}
	r.v.Set("client.edge_mappings", edgeMappings)

	// Logging
	r.v.Set("logging.file_logging", cfg.Logging.FileLogging)
	r.v.Set("logging.log_level", cfg.Logging.LogLevel)
	r.v.Set("logging.log_dir", cfg.Logging.LogDir)

	// Convert hosts - use maps to ensure correct TOML keys
	hosts := make([]map[string]string, len(cfg.Hosts))
	for i, h := range cfg.Hosts {
		hosts[i] = map[string]string{
			"name":     h.Name,
			"address":  h.Address,
			"position": h.Position,
		}
	}
	r.v.Set("hosts", hosts)
}

// getHostname returns the system hostname or a default.
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "waymon-server"
	}
	return hostname
}

// getDefaultSSHKeyPath returns the path to an existing SSH private key,
// or the preferred default path if none exist.
func getDefaultSSHKeyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Try common key types in order of preference
	keyPaths := []string{
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
		filepath.Join(home, ".ssh", "id_rsa"),
	}

	for _, p := range keyPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Return preferred default even if it doesn't exist
	return filepath.Join(home, ".ssh", "id_ed25519")
}
