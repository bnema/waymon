package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func (c *CLI) newConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect [slot]",
		Short: "Connect to a specific computer by slot number",
		Long: `Connect to a specific computer by its slot number (0-5).

Slot 0 is always the local server. Slots 1-5 are for connected clients.

This allows direct switching to a configured computer without cycling 
through all connected computers.

This command is useful for keybindings in window managers like Hyprland.
For example, you can bind:
  Super+0: waymon connect 0  (local)
  Super+1: waymon connect 1
  Super+2: waymon connect 2
  etc.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return c.runConnect(args[0])
		},
	}
}

func (c *CLI) runConnect(slotArg string) error {
	// Check if IPC client is available
	if c.ipcClient == nil {
		return fmt.Errorf("IPC client not available - CLI not properly initialized")
	}

	// Parse slot number
	slot, err := strconv.Atoi(slotArg)
	if err != nil {
		return fmt.Errorf("invalid slot number: %s", slotArg)
	}

	if slot < 0 || slot > 5 {
		return fmt.Errorf("slot number must be between 0 and 5")
	}

	// Send connect command
	_, err = c.ipcClient.SendConnect(int32(slot)) //nolint:gosec // slot is validated to be 0-5
	if err != nil {
		return fmt.Errorf("failed to connect to slot %d: %w", slot, err)
	}

	if slot == 0 {
		fmt.Println("Switched to local (server)")
	} else {
		fmt.Printf("Switched to computer in slot %d\n", slot)
	}
	return nil
}
