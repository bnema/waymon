package cli

import (
	"fmt"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
	"github.com/spf13/cobra"
)

func (c *CLI) newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the Waymon server",
		Long:  `Stop the running Waymon server instance via IPC.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runStop()
		},
	}
}

func (c *CLI) runStop() error {
	// Check if IPC client is available
	if c.ipcClient == nil {
		return fmt.Errorf("IPC client not available - CLI not properly initialized")
	}

	// Check if waymon is running
	if !c.ipcClient.IsRunning() {
		fmt.Println(styles.WarningStyle.Render("Waymon server is not running"))
		return nil
	}

	// Send stop command
	err := c.ipcClient.SendStop()
	if err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}

	fmt.Println(styles.SuccessStyle.Render(styles.IconCheck) + " Waymon server stopped")
	return nil
}
