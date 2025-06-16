package setup

import (
	"fmt"
	"os"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/logger"
	"github.com/charmbracelet/huh"
	"github.com/spf13/viper"
)

// DeviceSetup handles interactive device selection and configuration
type DeviceSetup struct {
	selector *input.DeviceSelector
}

// NewDeviceSetup creates a new device setup handler
func NewDeviceSetup() *DeviceSetup {
	return &DeviceSetup{
		selector: input.NewDeviceSelector(),
	}
}

// RunInteractiveSetup runs the interactive device selection if devices are not configured
func (ds *DeviceSetup) RunInteractiveSetup() error {
	cfg := config.Get()

	// Check if devices are already configured
	if cfg.Input.MouseDevice != "" && cfg.Input.KeyboardDevice != "" {
		logger.Info("Input devices already configured")
		// Validate that configured devices actually exist and have proper capabilities
		if err := ds.ValidateDevices(); err != nil {
			logger.Warnf("Device validation failed: %v", err)
			fmt.Println("⚠️  Configured devices have issues. Please reconfigure.")
		} else {
			return nil
		}
	}

	// Check if we have permission to access input devices
	if !ds.hasInputPermission() {
		return fmt.Errorf("insufficient permissions to access input devices. Please run with sudo or add user to 'input' group")
	}

	// Show welcome message
	fmt.Println("\n🖱️  Waymon Server Setup - Input Device Selection")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("This appears to be your first time running Waymon server.")
	fmt.Println("Let's select the input devices to capture from.")
	fmt.Println("\nIMPORTANT: We'll check actual device capabilities to ensure")
	fmt.Println("we select devices that can properly capture mouse/keyboard input.")

	// Select mouse device if not configured
	if cfg.Input.MouseDevice == "" {
		fmt.Println("📌 Step 1: Select Mouse Device")
		fmt.Println("Looking for devices with X/Y movement capabilities...")
		mousePath, err := ds.selector.SelectMouseDeviceEnhanced()
		if err != nil {
			return fmt.Errorf("mouse selection failed: %w", err)
		}
		cfg.Input.MouseDevice = mousePath
		viper.Set("input.mouse_device", mousePath)
		fmt.Printf("✓ Selected mouse: %s\n\n", mousePath)
	}

	// Select keyboard device if not configured
	if cfg.Input.KeyboardDevice == "" {
		fmt.Println("📌 Step 2: Select Keyboard Device")
		fmt.Println("Looking for devices with keyboard key capabilities...")
		keyboardPath, err := ds.selector.SelectKeyboardDeviceEnhanced()
		if err != nil {
			// Keyboard is optional
			logger.Warnf("Keyboard selection failed: %v", err)
			fmt.Println("⚠️  No keyboard selected. You can add one later in the config file.")
		} else {
			cfg.Input.KeyboardDevice = keyboardPath
			viper.Set("input.keyboard_device", keyboardPath)
			fmt.Printf("✓ Selected keyboard: %s\n\n", keyboardPath)
		}
	}

	// Save configuration
	if err := config.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println("✅ Device configuration saved!")
	fmt.Printf("📁 Config file: %s\n\n", config.GetConfigPath())

	return nil
}

// PromptDeviceReselection allows users to reselect devices
func (ds *DeviceSetup) PromptDeviceReselection() error {
	// Check permissions first
	if !ds.hasInputPermission() {
		return fmt.Errorf("insufficient permissions to access input devices. Please run with sudo or add user to 'input' group")
	}

	var confirm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Reconfigure Input Devices?").
				Description("This will let you select new mouse and keyboard devices").
				Value(&confirm),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if !confirm {
		return nil
	}

	// Clear existing device configuration
	viper.Set("input.mouse_device", "")
	viper.Set("input.keyboard_device", "")

	// Run setup again
	return ds.RunInteractiveSetup()
}

// hasInputPermission checks if we can access input devices
func (ds *DeviceSetup) hasInputPermission() bool {
	// Check if we're root
	if os.Geteuid() == 0 {
		return true
	}

	// Try to open a test event device
	testPath := "/dev/input/event0"
	if file, err := os.Open(testPath); err == nil {
		if err := file.Close(); err != nil {
			logger.Debugf("Failed to close test file %s: %v", testPath, err)
		}
		return true
	}

	return false
}

// ValidateDevices checks if configured devices are still valid and have proper capabilities
func (ds *DeviceSetup) ValidateDevices() error {
	cfg := config.Get()
	detector := input.NewDeviceDetector()

	// Validate mouse device
	if cfg.Input.MouseDevice != "" {
		if _, err := os.Stat(cfg.Input.MouseDevice); os.IsNotExist(err) {
			return fmt.Errorf("configured mouse device %s no longer exists", cfg.Input.MouseDevice)
		}
		if file, err := os.Open(cfg.Input.MouseDevice); err != nil {
			return fmt.Errorf("cannot access mouse device %s: %w", cfg.Input.MouseDevice, err)
		} else {
			// Check if device actually has mouse capabilities
			caps := detector.GetDeviceCapabilities(file)
			if err := file.Close(); err != nil {
				logger.Debugf("Failed to close mouse device file: %v", err)
			}

			// Check for REL_X and REL_Y
			hasMouseMovement := false
			if relCaps, hasRel := caps[0x02]; hasRel { // EV_REL
				hasX := false
				hasY := false
				for _, rel := range relCaps {
					if rel == 0x00 {
						hasX = true
					}
					if rel == 0x01 {
						hasY = true
					}
				}
				hasMouseMovement = hasX && hasY
			}

			if !hasMouseMovement {
				return fmt.Errorf("configured mouse device %s does not have X/Y movement capabilities", cfg.Input.MouseDevice)
			}
			logger.Infof("✓ Mouse device %s validated - has proper movement capabilities", cfg.Input.MouseDevice)
		}
	}

	// Validate keyboard device
	if cfg.Input.KeyboardDevice != "" {
		if _, err := os.Stat(cfg.Input.KeyboardDevice); os.IsNotExist(err) {
			logger.Warnf("Configured keyboard device %s no longer exists", cfg.Input.KeyboardDevice)
			// Don't fail for keyboard, it's optional
		} else if file, err := os.Open(cfg.Input.KeyboardDevice); err != nil {
			logger.Warnf("Cannot access keyboard device %s: %v", cfg.Input.KeyboardDevice, err)
		} else {
			// Check if device actually has keyboard capabilities
			caps := detector.GetDeviceCapabilities(file)
			if err := file.Close(); err != nil {
				logger.Debugf("Failed to close keyboard device file: %v", err)
			}

			// Check for keyboard keys
			keyboardKeys := 0
			if keyCaps, hasKeys := caps[0x01]; hasKeys { // EV_KEY
				for _, key := range keyCaps {
					if key >= 1 && key <= 83 { // KEY_ESC to KEY_KPDOT
						keyboardKeys++
					}
				}
			}

			if keyboardKeys < 20 {
				logger.Warnf("Configured keyboard device %s has limited keyboard capabilities (%d keys)", cfg.Input.KeyboardDevice, keyboardKeys)
			} else {
				logger.Infof("✓ Keyboard device %s validated - has %d keyboard keys", cfg.Input.KeyboardDevice, keyboardKeys)
			}
		}
	}

	return nil
}
