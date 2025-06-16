package setup

import (
	"fmt"
	"os"
	"path/filepath"

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
	if cfg.Input.MouseDeviceInfo != nil && cfg.Input.KeyboardDeviceInfo != nil {
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
	if cfg.Input.MouseDeviceInfo == nil {
		fmt.Println("📌 Step 1: Select Mouse Device")
		fmt.Println("Looking for devices with X/Y movement capabilities...")
		mousePath, err := ds.selector.SelectMouseDeviceEnhanced()
		if err != nil {
			return fmt.Errorf("mouse selection failed: %w", err)
		}
		
		// Get persistent device info
		persistentInfo, err := input.ResolveToPersistentPath(mousePath)
		if err != nil {
			return fmt.Errorf("could not get persistent device info for mouse: %w", err)
		}
		
		deviceInfo := &config.DeviceInfo{
			Name:       persistentInfo.Name,
			ByIDPath:   persistentInfo.ByIDPath,
			ByPathPath: persistentInfo.ByPathPath,
			VendorID:   persistentInfo.VendorID,
			ProductID:  persistentInfo.ProductID,
			Phys:       persistentInfo.Phys,
		}
		cfg.Input.MouseDeviceInfo = deviceInfo
		viper.Set("input.mouse_device_info", deviceInfo)
		fmt.Printf("✓ Selected mouse: %s\n", persistentInfo.Name)
		if persistentInfo.ByIDPath != "" {
			fmt.Printf("  Persistent ID: %s\n\n", filepath.Base(persistentInfo.ByIDPath))
		}
	}

	// Select keyboard device if not configured
	if cfg.Input.KeyboardDeviceInfo == nil {
		fmt.Println("📌 Step 2: Select Keyboard Device")
		fmt.Println("Looking for devices with keyboard key capabilities...")
		keyboardPath, err := ds.selector.SelectKeyboardDeviceEnhanced()
		if err != nil {
			// Keyboard is optional
			logger.Warnf("Keyboard selection failed: %v", err)
			fmt.Println("⚠️  No keyboard selected. You can add one later in the config file.")
		} else {
			// Get persistent device info
			persistentInfo, err := input.ResolveToPersistentPath(keyboardPath)
			if err != nil {
				logger.Warnf("Could not get persistent device info for keyboard: %v", err)
				fmt.Println("⚠️  Could not get persistent device info. Device may not work after reconnection.")
			} else {
				deviceInfo := &config.DeviceInfo{
					Name:       persistentInfo.Name,
					ByIDPath:   persistentInfo.ByIDPath,
					ByPathPath: persistentInfo.ByPathPath,
					VendorID:   persistentInfo.VendorID,
					ProductID:  persistentInfo.ProductID,
					Phys:       persistentInfo.Phys,
				}
				cfg.Input.KeyboardDeviceInfo = deviceInfo
				viper.Set("input.keyboard_device_info", deviceInfo)
				fmt.Printf("✓ Selected keyboard: %s\n", persistentInfo.Name)
				if persistentInfo.ByIDPath != "" {
					fmt.Printf("  Persistent ID: %s\n\n", filepath.Base(persistentInfo.ByIDPath))
				}
			}
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
	viper.Set("input.mouse_device_info", nil)
	viper.Set("input.keyboard_device_info", nil)

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
	if cfg.Input.MouseDeviceInfo != nil {
		// Resolve persistent device info to current path
		deviceInfo := &input.PersistentDeviceInfo{
			Name:       cfg.Input.MouseDeviceInfo.Name,
			ByIDPath:   cfg.Input.MouseDeviceInfo.ByIDPath,
			ByPathPath: cfg.Input.MouseDeviceInfo.ByPathPath,
			VendorID:   cfg.Input.MouseDeviceInfo.VendorID,
			ProductID:  cfg.Input.MouseDeviceInfo.ProductID,
			Phys:       cfg.Input.MouseDeviceInfo.Phys,
		}
		
		mousePath, err := deviceInfo.ResolveToEventPath()
		if err != nil {
			return fmt.Errorf("configured mouse device '%s' no longer available: %w", deviceInfo.Name, err)
		}
		
		if file, err := os.Open(mousePath); err != nil {
			return fmt.Errorf("cannot access mouse device %s: %w", mousePath, err)
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
				return fmt.Errorf("configured mouse device '%s' does not have X/Y movement capabilities", deviceInfo.Name)
			}
			logger.Infof("✓ Mouse device '%s' validated - has proper movement capabilities", deviceInfo.Name)
		}
	}

	// Validate keyboard device
	if cfg.Input.KeyboardDeviceInfo != nil {
		// Resolve persistent device info to current path
		deviceInfo := &input.PersistentDeviceInfo{
			Name:       cfg.Input.KeyboardDeviceInfo.Name,
			ByIDPath:   cfg.Input.KeyboardDeviceInfo.ByIDPath,
			ByPathPath: cfg.Input.KeyboardDeviceInfo.ByPathPath,
			VendorID:   cfg.Input.KeyboardDeviceInfo.VendorID,
			ProductID:  cfg.Input.KeyboardDeviceInfo.ProductID,
			Phys:       cfg.Input.KeyboardDeviceInfo.Phys,
		}
		
		keyboardPath, err := deviceInfo.ResolveToEventPath()
		if err != nil {
			logger.Warnf("Configured keyboard device '%s' no longer available: %v", deviceInfo.Name, err)
			// Don't fail for keyboard, it's optional
		} else if file, err := os.Open(keyboardPath); err != nil {
			logger.Warnf("Cannot access keyboard device %s: %v", keyboardPath, err)
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
				logger.Warnf("Configured keyboard device '%s' has limited keyboard capabilities (%d keys)", deviceInfo.Name, keyboardKeys)
			} else {
				logger.Infof("✓ Keyboard device '%s' validated - has %d keyboard keys", deviceInfo.Name, keyboardKeys)
			}
		}
	}

	return nil
}
