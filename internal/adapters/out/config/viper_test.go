package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bnema/waymon/internal/domain"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewViperRepository(t *testing.T) {
	repo := NewViperRepository()
	assert.NotNil(t, repo)
	assert.NotNil(t, repo.v)
}

func TestViperRepository_LoadDefaults(t *testing.T) {
	ctx := zerolog.New(os.Stderr).WithContext(t.Context())

	repo := NewViperRepository()

	// Use a non-existent path so it uses defaults
	tmpDir := t.TempDir()
	repo.SetConfigPath(filepath.Join(tmpDir, "nonexistent.toml"))

	cfg, err := repo.Load(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Check server defaults
	assert.Equal(t, 52525, cfg.Server.Port)
	assert.Equal(t, "0.0.0.0", cfg.Server.BindAddress)
	assert.Equal(t, 1, cfg.Server.MaxClients)
	assert.Equal(t, "/etc/waymon/host_key", cfg.Server.SSHHostKeyPath)
	assert.Equal(t, "/etc/waymon/authorized_keys", cfg.Server.SSHAuthKeysPath)
	assert.True(t, cfg.Server.SSHWhitelistOnly)

	// Check client defaults
	assert.Equal(t, "", cfg.Client.ServerAddress)
	assert.False(t, cfg.Client.AutoConnect)
	assert.Equal(t, 5, cfg.Client.ReconnectDelay)
	assert.Equal(t, 5, cfg.Client.EdgeThreshold)
	assert.Equal(t, "right", cfg.Client.ScreenPosition)
	assert.Equal(t, "ctrl+alt", cfg.Client.HotkeyModifier)
	assert.Equal(t, "s", cfg.Client.HotkeyKey)

	// Check logging defaults
	assert.True(t, cfg.Logging.FileLogging)
}

func TestViperRepository_SaveAndLoad(t *testing.T) {
	ctx := zerolog.New(os.Stderr).WithContext(t.Context())

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "waymon.toml")

	repo := NewViperRepository()
	repo.SetConfigPath(configPath)

	// Create a config to save
	cfg := &domain.Config{
		Server: domain.ServerCfg{
			Port:             12345,
			BindAddress:      "127.0.0.1",
			Name:             "test-server",
			MaxClients:       5,
			SSHHostKeyPath:   "/custom/host_key",
			SSHAuthKeysPath:  "/custom/authorized_keys",
			SSHWhitelist:     []string{"fp1", "fp2"},
			SSHWhitelistOnly: false,
		},
		Client: domain.ClientCfg{
			ServerAddress:  "192.168.1.100:52525",
			AutoConnect:    true,
			ReconnectDelay: 10,
			EdgeThreshold:  15,
			ScreenPosition: "left",
			EdgeMappings: []domain.EdgeMapping{
				{
					MonitorID:   "DP-1",
					Edge:        "right",
					Host:        "workstation",
					Description: "Right edge to workstation",
				},
			},
			HotkeyModifier: "alt",
			HotkeyKey:      "x",
			SSHPrivateKey:  "/home/user/.ssh/id_rsa",
		},
		Logging: domain.LoggingConfig{
			FileLogging: false,
			LogLevel:    "debug",
			LogDir:      "/var/log/waymon",
		},
		Hosts: []domain.HostConfig{
			{
				Name:     "workstation",
				Address:  "192.168.1.100:52525",
				Position: "right",
			},
			{
				Name:     "laptop",
				Address:  "192.168.1.101:52525",
				Position: "left",
			},
		},
	}

	// Save the config
	err := repo.Save(ctx, cfg)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Create a new repo to load
	repo2 := NewViperRepository()
	repo2.SetConfigPath(configPath)

	loadedCfg, err := repo2.Load(ctx)
	require.NoError(t, err)

	// Verify server config
	assert.Equal(t, cfg.Server.Port, loadedCfg.Server.Port)
	assert.Equal(t, cfg.Server.BindAddress, loadedCfg.Server.BindAddress)
	assert.Equal(t, cfg.Server.Name, loadedCfg.Server.Name)
	assert.Equal(t, cfg.Server.MaxClients, loadedCfg.Server.MaxClients)
	assert.Equal(t, cfg.Server.SSHHostKeyPath, loadedCfg.Server.SSHHostKeyPath)
	assert.Equal(t, cfg.Server.SSHAuthKeysPath, loadedCfg.Server.SSHAuthKeysPath)
	assert.Equal(t, cfg.Server.SSHWhitelist, loadedCfg.Server.SSHWhitelist)
	assert.Equal(t, cfg.Server.SSHWhitelistOnly, loadedCfg.Server.SSHWhitelistOnly)

	// Verify client config
	assert.Equal(t, cfg.Client.ServerAddress, loadedCfg.Client.ServerAddress)
	assert.Equal(t, cfg.Client.AutoConnect, loadedCfg.Client.AutoConnect)
	assert.Equal(t, cfg.Client.ReconnectDelay, loadedCfg.Client.ReconnectDelay)
	assert.Equal(t, cfg.Client.EdgeThreshold, loadedCfg.Client.EdgeThreshold)
	assert.Equal(t, cfg.Client.ScreenPosition, loadedCfg.Client.ScreenPosition)
	assert.Equal(t, cfg.Client.HotkeyModifier, loadedCfg.Client.HotkeyModifier)
	assert.Equal(t, cfg.Client.HotkeyKey, loadedCfg.Client.HotkeyKey)
	assert.Equal(t, cfg.Client.SSHPrivateKey, loadedCfg.Client.SSHPrivateKey)

	// Verify edge mappings
	require.Len(t, loadedCfg.Client.EdgeMappings, 1)
	assert.Equal(t, cfg.Client.EdgeMappings[0].MonitorID, loadedCfg.Client.EdgeMappings[0].MonitorID)
	assert.Equal(t, cfg.Client.EdgeMappings[0].Edge, loadedCfg.Client.EdgeMappings[0].Edge)
	assert.Equal(t, cfg.Client.EdgeMappings[0].Host, loadedCfg.Client.EdgeMappings[0].Host)
	assert.Equal(t, cfg.Client.EdgeMappings[0].Description, loadedCfg.Client.EdgeMappings[0].Description)

	// Verify logging config
	assert.Equal(t, cfg.Logging.FileLogging, loadedCfg.Logging.FileLogging)
	assert.Equal(t, cfg.Logging.LogLevel, loadedCfg.Logging.LogLevel)
	assert.Equal(t, cfg.Logging.LogDir, loadedCfg.Logging.LogDir)

	// Verify hosts
	require.Len(t, loadedCfg.Hosts, 2)
	assert.Equal(t, cfg.Hosts[0].Name, loadedCfg.Hosts[0].Name)
	assert.Equal(t, cfg.Hosts[0].Address, loadedCfg.Hosts[0].Address)
	assert.Equal(t, cfg.Hosts[0].Position, loadedCfg.Hosts[0].Position)
	assert.Equal(t, cfg.Hosts[1].Name, loadedCfg.Hosts[1].Name)
}

func TestViperRepository_GetConfigPath(t *testing.T) {
	repo := NewViperRepository()

	// Test with override
	repo.SetConfigPath("/custom/path.toml")
	assert.Equal(t, "/custom/path.toml", repo.GetConfigPath())

	// Test without override (result depends on environment)
	repo2 := NewViperRepository()
	path := repo2.GetConfigPath()
	assert.NotEmpty(t, path)
	assert.True(t, filepath.IsAbs(path))
}

func TestViperRepository_SetConfigPath(t *testing.T) {
	repo := NewViperRepository()
	assert.Empty(t, repo.configPath)

	repo.SetConfigPath("/test/path.toml")
	assert.Equal(t, "/test/path.toml", repo.configPath)
}

func TestViperRepository_LoadFromFile(t *testing.T) {
	ctx := zerolog.New(os.Stderr).WithContext(t.Context())

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "waymon.toml")

	// Write a TOML config file
	tomlContent := `
[server]
port = 9999
bind_address = "192.168.1.1"
name = "my-server"
max_clients = 3

[client]
server_address = "10.0.0.1:52525"
auto_connect = true
screen_position = "top"

[logging]
file_logging = false
log_level = "warn"

[[hosts]]
name = "desktop"
address = "10.0.0.2:52525"
position = "bottom"
`
	err := os.WriteFile(configPath, []byte(tomlContent), 0644)
	require.NoError(t, err)

	repo := NewViperRepository()
	repo.SetConfigPath(configPath)

	cfg, err := repo.Load(ctx)
	require.NoError(t, err)

	// Verify loaded values
	assert.Equal(t, 9999, cfg.Server.Port)
	assert.Equal(t, "192.168.1.1", cfg.Server.BindAddress)
	assert.Equal(t, "my-server", cfg.Server.Name)
	assert.Equal(t, 3, cfg.Server.MaxClients)

	assert.Equal(t, "10.0.0.1:52525", cfg.Client.ServerAddress)
	assert.True(t, cfg.Client.AutoConnect)
	assert.Equal(t, "top", cfg.Client.ScreenPosition)

	assert.False(t, cfg.Logging.FileLogging)
	assert.Equal(t, "warn", cfg.Logging.LogLevel)

	require.Len(t, cfg.Hosts, 1)
	assert.Equal(t, "desktop", cfg.Hosts[0].Name)
	assert.Equal(t, "10.0.0.2:52525", cfg.Hosts[0].Address)
	assert.Equal(t, "bottom", cfg.Hosts[0].Position)
}

func TestViperRepository_LoadInvalidTOML(t *testing.T) {
	ctx := zerolog.New(os.Stderr).WithContext(t.Context())

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "waymon.toml")

	// Write invalid TOML
	err := os.WriteFile(configPath, []byte("invalid toml content [[["), 0644)
	require.NoError(t, err)

	repo := NewViperRepository()
	repo.SetConfigPath(configPath)

	_, err = repo.Load(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error reading config file")
}

func TestGetHostname(t *testing.T) {
	hostname := getHostname()
	assert.NotEmpty(t, hostname)
}
