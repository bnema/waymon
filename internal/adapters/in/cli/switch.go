package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (c *CLI) newSwitchCmd() *cobra.Command {
	var (
		switchPrevious bool
		switchEnable   bool
		switchDisable  bool
	)

	cmd := &cobra.Command{
		Use:   "switch",
		Short: "Switch control between connected computers",
		Long: `Switch control between connected computers in the waymon network.

By default, switches to the next computer in the rotation. Use flags to specify
different switch behavior:

  waymon switch           # Switch to next computer
  waymon switch --prev    # Switch to previous computer  
  waymon switch --enable  # Enable mouse sharing (legacy)
  waymon switch --disable # Disable mouse sharing (legacy)

The switch command communicates with a running waymon server instance via IPC.
If no waymon instance is running, the command will fail.

Example usage in window manager configs:
  Hyprland: bind = $mainMod SHIFT, S, exec, waymon switch
  i3/Sway:  bindsym $mod+Shift+s exec waymon switch
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runSwitch(switchPrevious, switchEnable, switchDisable)
		},
	}

	cmd.Flags().BoolVar(&switchPrevious, "prev", false, "Switch to previous computer instead of next")
	cmd.Flags().BoolVar(&switchEnable, "enable", false, "Enable mouse sharing (legacy)")
	cmd.Flags().BoolVar(&switchDisable, "disable", false, "Disable mouse sharing (legacy)")

	// Make enable and disable mutually exclusive
	cmd.MarkFlagsMutuallyExclusive("enable", "disable")
	cmd.MarkFlagsMutuallyExclusive("prev", "enable")
	cmd.MarkFlagsMutuallyExclusive("prev", "disable")

	return cmd
}

func (c *CLI) runSwitch(previous, enable, disable bool) error {
	// Check if IPC client is available
	if c.ipcClient == nil {
		return fmt.Errorf("IPC client not available - CLI not properly initialized")
	}

	// Determine which action to perform
	var action SwitchAction
	switch {
	case previous:
		action = SwitchActionPrevious
	case enable:
		action = SwitchActionEnable
	case disable:
		action = SwitchActionDisable
	default:
		action = SwitchActionNext
	}

	// Send switch command
	resp, err := c.ipcClient.SendSwitch(action)
	if err != nil {
		return fmt.Errorf("failed to send switch command: %w", err)
	}

	// Display result
	c.displaySwitchResult(resp, action)
	return nil
}

func (c *CLI) displaySwitchResult(resp *StatusResponse, action SwitchAction) {
	if resp.TotalCount > 1 {
		// Multi-computer setup - show rotation info
		currentName := "unknown"
		if int(resp.CurrentIndex) < len(resp.ComputerNames) {
			currentName = resp.ComputerNames[resp.CurrentIndex]
		}

		switch action {
		case SwitchActionNext:
			fmt.Printf("✓ Switched to next computer: %s (%d/%d)\n",
				currentName, resp.CurrentIndex+1, resp.TotalCount)
		case SwitchActionPrevious:
			fmt.Printf("✓ Switched to previous computer: %s (%d/%d)\n",
				currentName, resp.CurrentIndex+1, resp.TotalCount)
		default:
			fmt.Printf("✓ Active computer: %s (%d/%d)\n",
				currentName, resp.CurrentIndex+1, resp.TotalCount)
		}

		// Show all computers in rotation
		if len(resp.ComputerNames) > 0 {
			fmt.Printf("Computers in rotation: ")
			for i, name := range resp.ComputerNames {
				if i == int(resp.CurrentIndex) {
					fmt.Printf("[%s]", name)
				} else {
					fmt.Printf("%s", name)
				}
				if i < len(resp.ComputerNames)-1 {
					fmt.Printf(" → ")
				}
			}
			fmt.Println()
		}
	} else {
		// Single computer or legacy setup
		switch action {
		case SwitchActionEnable:
			if resp.Active {
				fmt.Println("✓ Mouse sharing enabled")
			} else {
				fmt.Println("✗ Failed to enable mouse sharing")
			}
		case SwitchActionDisable:
			if !resp.Active {
				fmt.Println("✓ Mouse sharing disabled")
			} else {
				fmt.Println("✗ Failed to disable mouse sharing")
			}
		default:
			if resp.Active {
				fmt.Println("✓ Mouse sharing is active")
			} else {
				fmt.Println("✓ Mouse sharing is inactive")
			}
		}
	}

	// Show connection status
	if resp.Connected && resp.ServerHost != "" {
		fmt.Printf("Connected to: %s\n", resp.ServerHost)
	} else if !resp.Connected {
		fmt.Println("Not connected to server")
	}
}
