package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (c *CLI) newReleaseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "release",
		Short: "Release control back to the local machine",
		Long: `Release control from any connected computer and return mouse/keyboard 
control back to the local machine (server).

This command is useful for keybindings in window managers like Hyprland.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.runRelease()
		},
	}
}

func (c *CLI) runRelease() error {
	// Check if IPC client is available
	if c.ipcClient == nil {
		return fmt.Errorf("IPC client not available - CLI not properly initialized")
	}

	// Send release command
	_, err := c.ipcClient.SendRelease()
	if err != nil {
		return fmt.Errorf("failed to release control: %w", err)
	}

	fmt.Println("Control released to local machine")
	return nil
}
