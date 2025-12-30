package domain

// Config represents the complete application configuration.
// This is a pure domain type with no serialization tags.
type Config struct {
	Server  ServerCfg
	Client  ClientCfg
	Logging LoggingConfig
	Hosts   []HostConfig
}

// ServerCfg contains server-specific settings.
type ServerCfg struct {
	Port             int
	BindAddress      string
	Name             string
	MaxClients       int
	SSHHostKeyPath   string
	SSHAuthKeysPath  string
	SSHWhitelist     []string
	SSHWhitelistOnly bool
}

// ClientCfg contains client-specific settings.
type ClientCfg struct {
	ServerAddress  string
	AutoConnect    bool
	ReconnectDelay int
	EdgeThreshold  int
	ScreenPosition string
	EdgeMappings   []EdgeMapping
	HotkeyModifier string
	HotkeyKey      string
	SSHPrivateKey  string
}

// LoggingConfig contains logging settings.
type LoggingConfig struct {
	FileLogging bool
	LogLevel    string
	LogDir      string
}

// HostConfig represents a known host for quick connections.
type HostConfig struct {
	Name     string
	Address  string
	Position string // left, right, top, bottom
}

// EdgeMapping defines which monitor edge connects to which host.
type EdgeMapping struct {
	MonitorID   string
	Edge        string
	Host        string
	Description string
}
