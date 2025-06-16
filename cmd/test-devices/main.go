package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bnema/waymon/internal/input"
)

func main() {
	fmt.Println("Testing device capability detection...")
	fmt.Println("=====================================")

	// List all event devices
	matches, _ := filepath.Glob("/dev/input/event*")

	detector := input.NewDeviceDetector()

	for _, eventPath := range matches {
		// Get device name from sysfs
		eventName := filepath.Base(eventPath)
		sysPath := fmt.Sprintf("/sys/class/input/%s/device/name", eventName)

		nameBytes, _ := os.ReadFile(sysPath) //nolint:gosec // reading from trusted sysfs path
		deviceName := strings.TrimSpace(string(nameBytes))

		if deviceName == "" {
			continue
		}

		fmt.Printf("\n%s (%s):\n", deviceName, eventPath)

		file, err := os.OpenFile(eventPath, os.O_RDONLY, 0) //nolint:gosec // opening input devices from trusted path
		if err != nil {
			fmt.Printf("  ERROR: Cannot open: %v\n", err)
			continue
		}

		// Get kernel capabilities
		caps := detector.GetDeviceCapabilities(file)

		// Check REL capabilities
		hasMouseMovement := false
		if relCaps, hasRel := caps[0x02]; hasRel {
			fmt.Printf("  REL capabilities: ")
			for _, rel := range relCaps {
				switch rel {
				case 0x00:
					fmt.Print("REL_X ")
				case 0x01:
					fmt.Print("REL_Y ")
				case 0x06:
					fmt.Print("REL_HWHEEL ")
				case 0x08:
					fmt.Print("REL_WHEEL ")
				default:
					fmt.Printf("REL_%d ", rel)
				}
			}
			fmt.Println()

			// Check if has X and Y movement
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

		// Check KEY capabilities
		hasMouseButtons := false
		keyboardKeys := 0
		if keyCaps, hasKeys := caps[0x01]; hasKeys {
			for _, key := range keyCaps {
				if key >= 0x110 && key <= 0x117 { // Mouse buttons
					hasMouseButtons = true
				}
				if key >= 1 && key <= 83 { // Keyboard keys
					keyboardKeys++
				}
			}

			if hasMouseButtons {
				fmt.Printf("  Has mouse buttons: YES\n")
			}
			if keyboardKeys > 20 {
				fmt.Printf("  Has keyboard keys: YES (%d keys)\n", keyboardKeys)
			}
		}

		// Determine device type
		if hasMouseMovement && hasMouseButtons {
			fmt.Printf("  ✅ GOOD FOR MOUSE (has X/Y movement + buttons)\n")
		} else if hasMouseButtons && !hasMouseMovement {
			fmt.Printf("  ⚠️  Has mouse buttons but NO movement\n")
		} else if keyboardKeys > 20 {
			fmt.Printf("  ⌨️  KEYBOARD device\n")
		}

		if err := file.Close(); err != nil {
			fmt.Printf("Warning: failed to close file %s: %v\n", eventPath, err)
		}
	}
}
