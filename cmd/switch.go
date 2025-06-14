package cmd

import (
	"fmt"

	"github.com/bnema/waymon/internal/ipc"
	"github.com/bnema/waymon/internal/logger"
	pb "github.com/bnema/waymon/internal/proto"
	"github.com/spf13/cobra"
)

var (
	switchPrevious bool
	switchEnable   bool
	switchDisable  bool
)

var switchCmd = &cobra.Command{
	Use:   "switch",
	Short: "Switch control between connected computers",
	Long: `Switch control between connected computers in the waymon network.

By default, switches to the next computer in the rotation. Use flags to specify
different switch behavior:

  waymon switch           # Switch to next computer
  waymon switch --prev    # Switch to previous computer  
  waymon switch --enable  # Enable mouse sharing (legacy)
  waymon switch --disable # Disable mouse sharing (legacy)

The switch command communicates with a running waymon client instance via IPC.
If no waymon instance is running, the command will fail.

Example usage in window manager configs:
  Hyprland: bind = $mainMod SHIFT, S, exec, waymon switch
  i3/Sway:  bindsym $mod+Shift+s exec waymon switch
`,
	RunE: runSwitch,
}

func init() {
	switchCmd.Flags().BoolVar(&switchPrevious, "prev", false, "Switch to previous computer instead of next")
	switchCmd.Flags().BoolVar(&switchEnable, "enable", false, "Enable mouse sharing (legacy)")
	switchCmd.Flags().BoolVar(&switchDisable, "disable", false, "Disable mouse sharing (legacy)")
	
	// Make enable and disable mutually exclusive
	switchCmd.MarkFlagsMutuallyExclusive("enable", "disable")
	switchCmd.MarkFlagsMutuallyExclusive("prev", "enable")
	switchCmd.MarkFlagsMutuallyExclusive("prev", "disable")
	
	rootCmd.AddCommand(switchCmd)
}

func runSwitch(cmd *cobra.Command, args []string) error {
	// Create IPC client
	client, err := ipc.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create IPC client: %w", err)
	}

	// Determine which action to perform
	var action pb.SwitchAction
	switch {
	case switchPrevious:
		action = pb.SwitchAction_SWITCH_ACTION_PREVIOUS
	case switchEnable:
		action = pb.SwitchAction_SWITCH_ACTION_ENABLE
	case switchDisable:
		action = pb.SwitchAction_SWITCH_ACTION_DISABLE
	default:
		action = pb.SwitchAction_SWITCH_ACTION_NEXT
	}

	// Send switch command
	logger.Debugf("Sending switch command: %s", action)
	resp, err := client.SendSwitch(action)
	if err != nil {
		return fmt.Errorf("failed to send switch command: %w", err)
	}

	// Display result
	displaySwitchResult(resp, action)
	return nil
}

func displaySwitchResult(resp *pb.StatusResponse, action pb.SwitchAction) {
	if resp.TotalComputers > 1 {
		// Multi-computer setup - show rotation info
		currentName := "unknown"
		if int(resp.CurrentComputer) < len(resp.ComputerNames) {
			currentName = resp.ComputerNames[resp.CurrentComputer]
		}
		
		switch action {
		case pb.SwitchAction_SWITCH_ACTION_NEXT:
			fmt.Printf("✓ Switched to next computer: %s (%d/%d)\n", 
				currentName, resp.CurrentComputer+1, resp.TotalComputers)
		case pb.SwitchAction_SWITCH_ACTION_PREVIOUS:
			fmt.Printf("✓ Switched to previous computer: %s (%d/%d)\n", 
				currentName, resp.CurrentComputer+1, resp.TotalComputers)
		default:
			fmt.Printf("✓ Active computer: %s (%d/%d)\n", 
				currentName, resp.CurrentComputer+1, resp.TotalComputers)
		}
		
		// Show all computers in rotation
		if len(resp.ComputerNames) > 0 {
			fmt.Printf("Computers in rotation: ")
			for i, name := range resp.ComputerNames {
				if i == int(resp.CurrentComputer) {
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
		case pb.SwitchAction_SWITCH_ACTION_ENABLE:
			if resp.Active {
				fmt.Println("✓ Mouse sharing enabled")
			} else {
				fmt.Println("✗ Failed to enable mouse sharing")
			}
		case pb.SwitchAction_SWITCH_ACTION_DISABLE:
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