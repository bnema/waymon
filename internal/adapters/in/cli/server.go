package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (c *CLI) newServerCmd() *cobra.Command {
	var (
		port        int
		bindAddress string
		noTUI       bool
		debugTUI    bool
		daemon      bool
		configPath  string
		logLevel    string
	)

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run Waymon in server mode",
		Long: `Run Waymon in server mode to capture mouse/keyboard events and send them to clients.
The server captures input using evdev and transmits events over SSH to connected clients.

Requirements:
  - Must run as root (for evdev access)
  - SSH host key at ~/.ssh/waymon_host_key (or configured path)
  - Authorized keys for client authentication`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if c.serverRunner == nil {
				return fmt.Errorf("server runner not configured")
			}

			opts := ServerOptions{
				Port:        port,
				BindAddress: bindAddress,
				NoTUI:       noTUI,
				DebugTUI:    debugTUI,
				Daemon:      daemon,
				ConfigPath:  configPath,
				LogLevel:    logLevel,
			}

			return c.serverRunner(cmd.Context(), opts)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 0, "Port to listen on (default: 52525)")
	cmd.Flags().StringVarP(&bindAddress, "bind", "b", "", "Bind address (default: 0.0.0.0)")
	cmd.Flags().BoolVar(&noTUI, "no-tui", false, "Run without TUI (useful for non-interactive environments)")
	cmd.Flags().BoolVar(&debugTUI, "debug-tui", false, "Use minimal debug TUI")
	cmd.Flags().BoolVar(&daemon, "daemon", false, "Run as daemon (no UI, for systemd service)")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to config file")
	cmd.Flags().StringVarP(&logLevel, "log-level", "l", "", "Log level (debug, info, warn, error)")

	return cmd
}
