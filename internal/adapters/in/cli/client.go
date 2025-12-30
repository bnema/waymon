package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (c *CLI) newClientCmd() *cobra.Command {
	var (
		serverAddr string
		hostName   string
		configPath string
		logLevel   string
		noTUI      bool
	)

	cmd := &cobra.Command{
		Use:   "client",
		Short: "Run Waymon in client mode",
		Long: `Run Waymon in client mode to receive mouse/keyboard events from a server.
The client injects received input events locally using Wayland virtual input protocols.

Requirements:
  - Wayland compositor with virtual input support
  - SSH private key for server authentication`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if c.clientRunner == nil {
				return fmt.Errorf("client runner not configured")
			}

			opts := ClientOptions{
				ServerAddress: serverAddr,
				HostName:      hostName,
				ConfigPath:    configPath,
				LogLevel:      logLevel,
				NoTUI:         noTUI,
			}

			return c.clientRunner(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVarP(&serverAddr, "host", "H", "", "Server address (host:port)")
	cmd.Flags().StringVarP(&hostName, "name", "n", "", "Named host from config to connect to")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to config file")
	cmd.Flags().StringVarP(&logLevel, "log-level", "l", "", "Log level (debug, info, warn, error)")
	cmd.Flags().BoolVar(&noTUI, "no-tui", false, "Run without TUI")

	return cmd
}
