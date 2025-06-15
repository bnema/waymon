package cmd

import (
	"fmt"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/setup"
	"github.com/spf13/cobra"
)

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "Configure input devices",
	Long:  `Configure input devices for Waymon server mode.`,
}

var devicesSelectCmd = &cobra.Command{
	Use:   "select",
	Short: "Configure input devices",
	Long:  `Interactively select mouse and keyboard devices for input capture.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize config
		if err := config.Init(); err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		// Run device setup
		deviceSetup := setup.NewDeviceSetup()
		if err := deviceSetup.PromptDeviceReselection(); err != nil {
			return fmt.Errorf("device setup failed: %w", err)
		}

		fmt.Println("\n✅ Device configuration updated successfully!")
		return nil
	},
}

var devicesShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current device configuration",
	Long:  `Display the currently configured input devices.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize config
		if err := config.Init(); err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		cfg := config.Get()
		
		fmt.Println("\n📋 Current Device Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		
		if cfg.Input.MouseDevice != "" {
			fmt.Printf("🖱️  Mouse Device:    %s\n", cfg.Input.MouseDevice)
		} else {
			fmt.Println("🖱️  Mouse Device:    (not configured)")
		}
		
		if cfg.Input.KeyboardDevice != "" {
			fmt.Printf("⌨️  Keyboard Device: %s\n", cfg.Input.KeyboardDevice)
		} else {
			fmt.Println("⌨️  Keyboard Device: (not configured)")
		}
		
		fmt.Printf("\n📁 Config File: %s\n", config.GetConfigPath())
		
		return nil
	},
}

func init() {
	rootCmd.AddCommand(devicesCmd)
	devicesCmd.AddCommand(devicesSelectCmd)
	devicesCmd.AddCommand(devicesShowCmd)
}