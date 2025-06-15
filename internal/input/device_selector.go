package input

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bnema/waymon/internal/logger"
	"github.com/charmbracelet/huh"
	"github.com/gvalkov/golang-evdev"
)

// DeviceType represents the type of input device
type DeviceType int

const (
	DeviceTypeMouse DeviceType = iota
	DeviceTypeKeyboard
)

// DeviceInfo represents information about an input device
type DeviceInfo struct {
	Path        string
	Name        string
	Symlink     string
	Descriptive string
}

// DeviceSelector provides interactive device selection using huh
type DeviceSelector struct{}

// NewDeviceSelector creates a new device selector
func NewDeviceSelector() *DeviceSelector {
	return &DeviceSelector{}
}

// SelectMouseDevice presents an interactive selection for mouse devices
func (s *DeviceSelector) SelectMouseDevice() (string, error) {
	devices, err := s.listDevices(DeviceTypeMouse)
	if err != nil {
		return "", err
	}

	if len(devices) == 0 {
		return "", fmt.Errorf("no mouse devices found")
	}

	// If only one device, use it automatically
	if len(devices) == 1 {
		logger.Infof("Auto-selected mouse device: %s", devices[0].Descriptive)
		return devices[0].Path, nil
	}

	// Create options for the select
	options := make([]huh.Option[string], len(devices))
	for i, dev := range devices {
		options[i] = huh.NewOption(dev.Descriptive, dev.Path)
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select Mouse Device").
				Description("Choose the mouse device to capture input from").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return "", fmt.Errorf("device selection cancelled: %w", err)
	}

	return selected, nil
}

// SelectKeyboardDevice presents an interactive selection for keyboard devices
func (s *DeviceSelector) SelectKeyboardDevice() (string, error) {
	devices, err := s.listDevices(DeviceTypeKeyboard)
	if err != nil {
		return "", err
	}

	if len(devices) == 0 {
		return "", fmt.Errorf("no keyboard devices found")
	}

	// If only one device, use it automatically
	if len(devices) == 1 {
		logger.Infof("Auto-selected keyboard device: %s", devices[0].Descriptive)
		return devices[0].Path, nil
	}

	// Create options for the select
	options := make([]huh.Option[string], len(devices))
	for i, dev := range devices {
		options[i] = huh.NewOption(dev.Descriptive, dev.Path)
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select Keyboard Device").
				Description("Choose the keyboard device to capture input from").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return "", fmt.Errorf("device selection cancelled: %w", err)
	}

	return selected, nil
}

// listDevices lists available input devices of the specified type
func (s *DeviceSelector) listDevices(deviceType DeviceType) ([]DeviceInfo, error) {
	evdevices, err := evdev.ListInputDevices("/dev/input/event*")
	if err != nil {
		return nil, fmt.Errorf("failed to list input devices: %w", err)
	}

	var devices []DeviceInfo
	for _, dev := range evdevices {
		if s.isDeviceType(dev, deviceType) {
			info := DeviceInfo{
				Path: dev.Fn,
				Name: dev.Name,
			}

			// Try to find symlink
			info.Symlink = s.findSymlink(dev.Fn)

			// Create descriptive name
			if info.Symlink != "" {
				info.Descriptive = fmt.Sprintf("%s (%s → %s)", dev.Name, info.Symlink, dev.Fn)
			} else {
				info.Descriptive = fmt.Sprintf("%s (%s)", dev.Name, dev.Fn)
			}

			devices = append(devices, info)
		}
	}

	return devices, nil
}

// isDeviceType checks if a device matches the requested type
func (s *DeviceSelector) isDeviceType(dev *evdev.InputDevice, deviceType DeviceType) bool {
	if dev.Capabilities == nil {
		return false
	}

	switch deviceType {
	case DeviceTypeMouse:
		// Check for relative axes (mouse movement)
		if relAxes, ok := dev.CapabilitiesFlat[evdev.EV_REL]; ok && len(relAxes) > 0 {
			// Check for X and Y axes
			hasX := false
			hasY := false
			for _, axis := range relAxes {
				if axis == evdev.REL_X {
					hasX = true
				}
				if axis == evdev.REL_Y {
					hasY = true
				}
			}
			if hasX && hasY {
				// Also check for mouse buttons
				if btns, ok := dev.CapabilitiesFlat[evdev.EV_KEY]; ok && len(btns) > 0 {
					// Check for standard mouse buttons
					for _, btn := range btns {
						if btn == evdev.BTN_LEFT || btn == evdev.BTN_RIGHT || btn == evdev.BTN_MIDDLE {
							return true
						}
					}
				}
			}
		}
		return false

	case DeviceTypeKeyboard:
		// Skip devices that are likely power buttons or other special devices
		nameLower := strings.ToLower(dev.Name)
		if strings.Contains(nameLower, "power") ||
			strings.Contains(nameLower, "video") ||
			strings.Contains(nameLower, "sleep") ||
			strings.Contains(nameLower, "button") {
			return false
		}

		// Check for key events
		if keys, ok := dev.CapabilitiesFlat[evdev.EV_KEY]; ok && len(keys) > 0 {
			// Check for standard keyboard keys (KEY_A to KEY_Z)
			hasAlphaKeys := false
			for _, key := range keys {
				if key >= evdev.KEY_A && key <= evdev.KEY_Z {
					hasAlphaKeys = true
					break
				}
			}
			return hasAlphaKeys
		}
		return false

	default:
		return false
	}
}

// findSymlink finds the symlink for a device path in /dev/input/by-id or /dev/input/by-path
func (s *DeviceSelector) findSymlink(devicePath string) string {
	// Check /dev/input/by-id first
	byIDPath := "/dev/input/by-id"
	if symlink := s.findSymlinkInDir(devicePath, byIDPath); symlink != "" {
		return symlink
	}

	// Then check /dev/input/by-path
	byPathPath := "/dev/input/by-path"
	if symlink := s.findSymlinkInDir(devicePath, byPathPath); symlink != "" {
		return symlink
	}

	return ""
}

// findSymlinkInDir finds a symlink pointing to devicePath in the given directory
func (s *DeviceSelector) findSymlinkInDir(devicePath, dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			fullPath := filepath.Join(dir, entry.Name())
			target, err := os.Readlink(fullPath)
			if err != nil {
				continue
			}

			// Resolve relative paths
			if !filepath.IsAbs(target) {
				target = filepath.Join(dir, target)
			}
			target = filepath.Clean(target)

			// Check if this symlink points to our device
			if target == devicePath {
				return fullPath
			}
		}
	}

	return ""
}