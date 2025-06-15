package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	setupDevices bool
	serverSetup  bool
	clientSetup  bool
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up Waymon for server or client mode",
	Long: `Set up Waymon input capture and injection.

For servers: Configures evdev device access for input capture
For clients: Checks Wayland virtual input protocol support`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().BoolVar(&setupDevices, "devices", false, "Select input devices for server mode")
	setupCmd.Flags().BoolVar(&serverSetup, "server", false, "Set up for server mode")
	setupCmd.Flags().BoolVar(&clientSetup, "client", false, "Set up for client mode")
}

// detectWaylandVirtualInputSupport checks if Wayland virtual input is available on the system
func detectWaylandVirtualInputSupport() bool {
	// Try to create a Wayland virtual input backend to test availability
	_, err := input.NewWaylandVirtualInput()
	return err == nil
}

func runSetup(cmd *cobra.Command, args []string) error {
	fmt.Println(ui.FormatSetupHeader("Waymon Setup"))

	// Determine mode
	if !serverSetup && !clientSetup {
		// Auto-detect based on whether we have evdev
		if input.IsEvdevAvailable() {
			serverSetup = true
		} else {
			clientSetup = true
		}
	}

	if serverSetup {
		return runServerSetup()
	} else {
		return runClientSetup()
	}
}

func runServerSetup() error {
	fmt.Println("\nSetting up Waymon Server...")
	fmt.Println()

	// Check evdev availability
	if !input.IsEvdevAvailable() {
		fmt.Println(ui.ErrorStyle.Render("✗ /dev/input devices not accessible"))
		fmt.Println("   Server mode requires access to /dev/input/event* devices")
		fmt.Println()
		fmt.Println("   To fix this:")
		fmt.Println("   1. Add your user to the 'input' group:")
		fmt.Println("      sudo usermod -a -G input $USER")
		fmt.Println("   2. Log out and back in for group changes to take effect")
		fmt.Println("   3. Run 'waymon setup' again")
		fmt.Println()
		return fmt.Errorf("evdev not accessible")
	}

	fmt.Println(ui.SuccessStyle.Render("✓ Evdev input devices accessible"))

	// If --devices flag is set, run device selection
	if setupDevices {
		return selectDevices()
	}

	// Check if devices are already configured
	cfg := config.Get()
	if cfg.Input.MouseDevice != "" || cfg.Input.KeyboardDevice != "" {
		fmt.Println(ui.SuccessStyle.Render("✓ Input devices already configured"))
		if cfg.Input.MouseDevice != "" {
			fmt.Printf("   Mouse: %s\n", cfg.Input.MouseDevice)
		}
		if cfg.Input.KeyboardDevice != "" {
			fmt.Printf("   Keyboard: %s\n", cfg.Input.KeyboardDevice)
		}
		fmt.Println()
		fmt.Println("   To reconfigure devices, run: waymon setup --devices")
	} else {
		fmt.Println("\n   Input devices will be auto-detected when server starts.")
		fmt.Println("   To manually select devices, run: waymon setup --devices")
	}

	fmt.Println()
	fmt.Println("Server setup complete! You can now run:")
	fmt.Println("   sudo waymon server")
	fmt.Println()
	return nil
}

func runClientSetup() error {
	fmt.Println("\nSetting up Waymon Client...")
	fmt.Println()

	// Check if running as root
	if os.Geteuid() == 0 {
		fmt.Println(ui.ErrorStyle.Render("✗ Please run this command as a normal user (not root)"))
		fmt.Println("   Client mode uses Wayland virtual input which works through compositor permissions")
		return fmt.Errorf("cannot run client setup as root")
	}

	// Detect Wayland virtual input availability
	fmt.Println("Checking Wayland virtual input support...")

	waylandVirtualInputAvailable := detectWaylandVirtualInputSupport()
	if waylandVirtualInputAvailable {
		fmt.Println(ui.SuccessStyle.Render("✓ Wayland virtual input protocols detected"))
		fmt.Println("   Your compositor supports virtual input for input injection")
		fmt.Println()
		fmt.Println("Client setup complete! You can now run:")
		fmt.Println("   waymon client --host <server-ip>:52525")
		fmt.Println()
		return nil
	}

	fmt.Println(ui.ErrorStyle.Render("✗ Wayland virtual input not available"))
	fmt.Println("   Client mode requires Wayland virtual input protocols")
	fmt.Println()
	fmt.Println("   📦 Supported Compositors:")
	fmt.Println("      • Hyprland: Full native support")
	fmt.Println("      • Sway: Full native support")
	fmt.Println("      • Other wlroots-based: Generally supported")
	fmt.Println()
	fmt.Println("   ❌ Limited Support:")
	fmt.Println("      • GNOME: Uses different protocols")
	fmt.Println("      • KDE Plasma: Uses different protocols")
	fmt.Println()
	return fmt.Errorf("Wayland virtual input support required")
}

func selectDevices() error {
	fmt.Println("\nSelect input devices for capture...")
	fmt.Println()

	selector := input.NewDeviceSelector()
	cfg := config.Get()

	// Select mouse device
	mousePath, err := selector.SelectMouseDeviceEnhanced()
	if err != nil {
		if !strings.Contains(err.Error(), "cancelled") {
			fmt.Println(ui.ErrorStyle.Render(fmt.Sprintf("✗ Failed to select mouse device: %v", err)))
		}
		return err
	}
	cfg.Input.MouseDevice = mousePath
	fmt.Println(ui.SuccessStyle.Render(fmt.Sprintf("✓ Mouse device selected: %s", mousePath)))

	// Select keyboard device (optional)
	keyboardPath, err := selector.SelectKeyboardDeviceEnhanced()
	if err != nil {
		if strings.Contains(err.Error(), "cancelled") {
			fmt.Println("   Keyboard selection skipped")
		} else {
			fmt.Println(ui.WarningStyle.Render(fmt.Sprintf("⚠ Keyboard selection failed: %v", err)))
			fmt.Println("   Continuing without keyboard capture")
		}
	} else {
		cfg.Input.KeyboardDevice = keyboardPath
		fmt.Println(ui.SuccessStyle.Render(fmt.Sprintf("✓ Keyboard device selected: %s", keyboardPath)))
	}

	// Save configuration
	viper.Set("input.mouse_device", cfg.Input.MouseDevice)
	viper.Set("input.keyboard_device", cfg.Input.KeyboardDevice)
	if err := viper.WriteConfig(); err != nil {
		fmt.Println(ui.ErrorStyle.Render(fmt.Sprintf("✗ Failed to save configuration: %v", err)))
		return err
	}

	fmt.Println()
	fmt.Println(ui.SuccessStyle.Render("✓ Device configuration saved"))
	return nil
}
