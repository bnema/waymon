package cmd

import (
	"fmt"
	"os"

	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/ui"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Check and guide Waymon setup for Wayland virtual input",
	Long: `Check Wayland virtual input support and provide setup guidance.

Waymon uses Wayland virtual input protocols for modern input capture.
This command checks if your compositor supports the required protocols
and provides setup instructions if needed.`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

// detectWaylandVirtualInputSupport checks if Wayland virtual input is available on the system
func detectWaylandVirtualInputSupport() bool {
	// Try to create a Wayland virtual input backend to test availability
	_, err := input.NewWaylandVirtualInput()
	return err == nil
}

func runSetup(cmd *cobra.Command, args []string) error {
	fmt.Println(ui.FormatSetupHeader("Waymon Setup"))

	// Check if running as root
	if os.Geteuid() == 0 {
		fmt.Println(ui.ErrorStyle.Render("✗ Please run this command as a normal user (not root)"))
		fmt.Println("   Waymon uses Wayland virtual input which works through compositor permissions")
		return fmt.Errorf("cannot run setup as root")
	}

	// Detect Wayland virtual input availability
	fmt.Println("Checking Wayland virtual input support...")
	fmt.Println()

	waylandVirtualInputAvailable := detectWaylandVirtualInputSupport()
	if waylandVirtualInputAvailable {
		fmt.Println(ui.SuccessStyle.Render("✓ Wayland virtual input protocols detected"))
		fmt.Println("   Your compositor supports virtual input for secure, rootless input capture")
		fmt.Println("   No additional setup required - waymon will work out of the box!")
		fmt.Println()
		fmt.Println("   You can now run waymon directly:")
		fmt.Println("   • waymon server")
		fmt.Println("   • waymon client --host <server-ip>")
		fmt.Println()
		return nil
	} else {
		fmt.Println(ui.ErrorStyle.Render("✗ Wayland virtual input not available"))
		fmt.Println("   Waymon requires Wayland virtual input protocols for modern input handling")
		fmt.Println()
		fmt.Println("   Please install and configure Wayland virtual input support:")
		fmt.Println()
		fmt.Println("   📦 Supported Compositors:")
		fmt.Println("      • Hyprland: Full native support")
		fmt.Println("      • Sway: Full native support")
		fmt.Println("      • Other wlroots-based: Generally supported")
		fmt.Println()
		fmt.Println("   ❌ Unsupported Compositors:")
		fmt.Println("      • GNOME: Limited support (use different protocols)")
		fmt.Println("      • KDE Plasma: Limited support (use different protocols)")
		fmt.Println()
		fmt.Println("   📦 Hyprland Setup:")
		fmt.Println("      • Virtual input protocols should work by default")
		fmt.Println("      • No additional configuration required")
		fmt.Println()
		fmt.Println("   📦 Sway Setup:")
		fmt.Println("      • Virtual input protocols should work by default")
		fmt.Println("      • Ensure you're running a recent version of Sway")
		fmt.Println()
		fmt.Println("   📦 Other wlroots compositors:")
		fmt.Println("      • Check your compositor's documentation for virtual input support")
		fmt.Println("      • May require enabling virtual device protocols")
		fmt.Println()
		fmt.Println("   After configuring virtual input support, restart your compositor and try again.")
		fmt.Println()
		return fmt.Errorf("Wayland virtual input support required but not available")
	}
}
