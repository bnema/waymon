package cmd

import (
	"fmt"
	"strconv"

	"github.com/bnema/waymon/internal/ipc"
	"github.com/bnema/waymon/internal/logger"
	"github.com/spf13/cobra"
)

// connectCmd represents the connect command
var connectCmd = &cobra.Command{
	Use:   "connect [slot]",
	Short: "Connect to a specific computer by slot number",
	Long: `Connect to a specific computer by its slot number (1-5).

This allows direct switching to a configured computer without cycling 
through all connected computers.

This command is useful for keybindings in window managers like Hyprland.
For example, you can bind:
  Super+1: waymon connect 1
  Super+2: waymon connect 2
  etc.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse slot number
		slot, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid slot number: %s", args[0])
		}

		if slot < 1 || slot > 5 {
			return fmt.Errorf("slot number must be between 1 and 5")
		}

		// Create IPC client
		client, err := ipc.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create IPC client: %w", err)
		}
		defer func() {
			if err := client.Close(); err != nil {
				logger.Errorf("Failed to close IPC client: %v", err)
			}
		}()

		// Send connect command
		if err := client.SendConnect(int32(slot)); err != nil { //nolint:gosec // slot is validated to be 1-5
			return fmt.Errorf("failed to connect to slot %d: %w", slot, err)
		}

		fmt.Printf("Switched to computer in slot %d\n", slot)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(connectCmd)
}
