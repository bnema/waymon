package cmd

import (
	"fmt"
	"os/exec"

	"github.com/bnema/waymon/internal/ui"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Waymon server daemon",
	Long:  `Stop the Waymon server if it's running as a systemd service.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Try to stop via systemctl
		stopCmd := exec.Command("systemctl", "stop", "waymon")
		output, err := stopCmd.CombinedOutput()
		
		if err != nil {
			// Check if it's a permission error
			if exec.Command("systemctl", "is-active", "waymon").Run() == nil {
				fmt.Println(ui.ErrorStyle.Render("Error: ") + "Permission denied. Try:")
				fmt.Println(ui.InfoStyle.Render("  sudo systemctl stop waymon"))
				return nil
			}
			// Service might not exist
			fmt.Println(ui.WarningStyle.Render("Waymon service not found or not running"))
			return nil
		}
		
		fmt.Println(ui.SuccessStyle.Render("✓") + " Waymon server stopped")
		if len(output) > 0 {
			fmt.Printf("Output: %s\n", output)
		}
		
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}