package cmd

import (
	"fmt"

	"github.com/bnema/waymon/internal/ipc"
	"github.com/spf13/cobra"
)

// releaseCmd represents the release command
var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Release control back to the local machine",
	Long: `Release control from any connected computer and return mouse/keyboard 
control back to the local machine (server).

This command is useful for keybindings in window managers like Hyprland.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create IPC client
		client, err := ipc.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create IPC client: %w", err)
		}
		defer client.Close()

		// Send release command
		if err := client.SendRelease(); err != nil {
			return fmt.Errorf("failed to release control: %w", err)
		}

		fmt.Println("Control released to local machine")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(releaseCmd)
}
