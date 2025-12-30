// Package cli provides the command-line interface adapter for waymon.
// It follows clean architecture by receiving dependencies through injection.
package cli

import (
	"context"

	"github.com/bnema/waymon/internal/boundaries/out"
	"github.com/spf13/cobra"
)

var (
	// Version is set at build time via ldflags.
	Version = "dev"
)

// IPCClient defines the interface for IPC communication.
// This allows dependency injection and testing.
type IPCClient interface {
	SendSwitch(action SwitchAction) (*StatusResponse, error)
	SendStatus() (*StatusResponse, error)
	SendRelease() (*StatusResponse, error)
	SendConnect(slot int32) (*StatusResponse, error)
	SendStop() error
	IsRunning() bool
}

// SwitchAction represents an action for the switch command.
type SwitchAction int

const (
	SwitchActionNext SwitchAction = iota
	SwitchActionPrevious
	SwitchActionEnable
	SwitchActionDisable
)

// StatusResponse contains the response to a status query.
type StatusResponse struct {
	Active        bool
	Connected     bool
	ServerHost    string
	CurrentIndex  int32
	TotalCount    int32
	ComputerNames []string
}

// ServerOptions configures the server application.
type ServerOptions struct {
	Port        int
	BindAddress string
	NoTUI       bool
	DebugTUI    bool
	Daemon      bool
	ConfigPath  string
	LogLevel    string
}

// ClientOptions configures the client application.
type ClientOptions struct {
	ServerAddress string
	HostName      string
	ConfigPath    string
	LogLevel      string
	NoTUI         bool
}

// ServerRunner is a function that runs the server with the given options.
type ServerRunner func(ctx context.Context, opts ServerOptions) error

// ClientRunner is a function that runs the client with the given options.
type ClientRunner func(ctx context.Context, opts ClientOptions) error

// CLI holds the dependencies for CLI commands.
type CLI struct {
	// Dependencies injected from app layer
	ipcClient   IPCClient
	displayPort out.DisplayPort
	configRepo  out.ConfigRepository

	// Runners injected from composition root
	serverRunner ServerRunner
	clientRunner ClientRunner

	// Flags
	logLevel   string
	configPath string

	// Root command
	rootCmd *cobra.Command
}

// Option is a functional option for configuring CLI.
type Option func(*CLI)

// WithIPCClient sets the IPC client.
func WithIPCClient(client IPCClient) Option {
	return func(c *CLI) {
		c.ipcClient = client
	}
}

// WithDisplayPort sets the display port.
func WithDisplayPort(port out.DisplayPort) Option {
	return func(c *CLI) {
		c.displayPort = port
	}
}

// WithConfigRepository sets the config repository.
func WithConfigRepository(repo out.ConfigRepository) Option {
	return func(c *CLI) {
		c.configRepo = repo
	}
}

// WithServerRunner sets the server runner function.
func WithServerRunner(runner ServerRunner) Option {
	return func(c *CLI) {
		c.serverRunner = runner
	}
}

// WithClientRunner sets the client runner function.
func WithClientRunner(runner ClientRunner) Option {
	return func(c *CLI) {
		c.clientRunner = runner
	}
}

// New creates a new CLI instance with the given options.
func New(opts ...Option) *CLI {
	c := &CLI{}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	// Build command tree
	c.buildCommands()

	return c
}

// Execute runs the CLI.
func (c *CLI) Execute() error {
	return c.rootCmd.Execute()
}

// ExecuteContext runs the CLI with a context.
func (c *CLI) ExecuteContext(ctx context.Context) error {
	return c.rootCmd.ExecuteContext(ctx)
}

// SetVersion sets the version string.
func SetVersion(v string) {
	Version = v
}

// GetRootCmd returns the root command (useful for testing).
func (c *CLI) GetRootCmd() *cobra.Command {
	return c.rootCmd
}

// buildCommands builds the command tree.
func (c *CLI) buildCommands() {
	c.rootCmd = &cobra.Command{
		Use:   "waymon",
		Short: "Waymon - Wayland mouse sharing",
		Long: `Waymon is a client/server mouse sharing application for Wayland systems.
It allows seamless mouse movement between two computers on a local network,
working around Wayland's security restrictions by using the uinput kernel module.`,
		SilenceUsage: true,
		Version:      Version,
	}

	c.rootCmd.SetVersionTemplate(`{{with .Name}}{{printf "%s " .}}{{end}}{{printf "version %s\n" .Version}}`)

	// Add global flags
	c.rootCmd.PersistentFlags().StringVar(&c.configPath, "config", "", "config file (default is $HOME/.config/waymon/waymon.toml)")
	c.rootCmd.PersistentFlags().StringVar(&c.logLevel, "log-level", "", "set log level (debug, info, warn, error, fatal)")

	// Add subcommands
	c.rootCmd.AddCommand(c.newServerCmd())
	c.rootCmd.AddCommand(c.newClientCmd())
	c.rootCmd.AddCommand(c.newStatusCmd())
	c.rootCmd.AddCommand(c.newSwitchCmd())
	c.rootCmd.AddCommand(c.newConnectCmd())
	c.rootCmd.AddCommand(c.newReleaseCmd())
	c.rootCmd.AddCommand(c.newStopCmd())
	c.rootCmd.AddCommand(c.newListCmd())
	c.rootCmd.AddCommand(c.newMonitorsCmd())
	c.rootCmd.AddCommand(c.newConfigCmd())
	c.rootCmd.AddCommand(c.newVersionCmd())
}
