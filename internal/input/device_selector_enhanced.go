package input

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bnema/waymon/internal/logger"
	"github.com/charmbracelet/huh"
)

// DeviceCapabilities represents what a device can do
type DeviceCapabilities struct {
	HasMouseMovement bool
	HasMouseButtons  bool
	HasKeyboard      bool
	Recommendation   string
}

// EnhancedDeviceInfo represents information about an input device with capabilities
type EnhancedDeviceInfo struct {
	Path         string
	Name         string
	Symlink      string
	Descriptive  string
	Capabilities DeviceCapabilities
}

// analyzeCapabilities analyzes what a device can do based on actual kernel capabilities
func (s *DeviceSelector) analyzeCapabilities(file *os.File, deviceName string) DeviceCapabilities {
	caps := DeviceCapabilities{}

	// Get device name from sysfs for more accurate detection
	eventName := filepath.Base(file.Name())
	sysPath := fmt.Sprintf("/sys/class/input/%s/device/name", eventName)

	var fullName string
	if data, err := os.ReadFile(sysPath); err == nil { //nolint:gosec // sysPath is constructed from device path
		fullName = strings.TrimSpace(string(data))
	} else {
		fullName = deviceName
	}

	// Get actual device capabilities from kernel
	detector := NewDeviceDetector()
	kernelCaps := detector.GetDeviceCapabilities(file)

	// Check for mouse capabilities - must have REL_X and REL_Y
	if relCaps, hasRel := kernelCaps[0x02]; hasRel { // EV_REL
		hasX := false
		hasY := false
		for _, rel := range relCaps {
			if rel == 0x00 { // REL_X
				hasX = true
			}
			if rel == 0x01 { // REL_Y
				hasY = true
			}
		}
		caps.HasMouseMovement = hasX && hasY
	}

	// Check for mouse buttons
	if keyCaps, hasKeys := kernelCaps[0x01]; hasKeys { // EV_KEY
		for _, key := range keyCaps {
			// BTN_LEFT = 0x110, BTN_RIGHT = 0x111, BTN_MIDDLE = 0x112
			if key >= 0x110 && key <= 0x117 {
				caps.HasMouseButtons = true
				break
			}
		}
	}

	// Check for keyboard capabilities
	if keyCaps, hasKeys := kernelCaps[0x01]; hasKeys { // EV_KEY
		// Check for typical keyboard keys (KEY_Q = 16, KEY_SPACE = 57, etc)
		keyboardKeyCount := 0
		for _, key := range keyCaps {
			if key >= 1 && key <= 83 { // KEY_ESC to KEY_KPDOT
				keyboardKeyCount++
			}
		}
		// If device has many keyboard keys, it's likely a keyboard
		caps.HasKeyboard = keyboardKeyCount > 20
	}

	// Generate recommendations based on actual capabilities
	switch {
	case caps.HasMouseMovement && caps.HasMouseButtons && !caps.HasKeyboard:
		// This is a pure mouse/trackpad device
		caps.Recommendation = "🖱️ RECOMMENDED for MOUSE"
	case caps.HasKeyboard && !caps.HasMouseMovement:
		// This is a pure keyboard device
		caps.Recommendation = "⌨️ RECOMMENDED for KEYBOARD"
	case caps.HasKeyboard && caps.HasMouseMovement:
		// This is a combo device (like laptop with built-in trackpad)
		caps.Recommendation = "🔄 COMBO device (mouse + keyboard)"
	case caps.HasMouseButtons && !caps.HasMouseMovement:
		// Gaming devices, presenter remotes, etc
		caps.Recommendation = "🎮 Gaming/Special device (buttons only)"
	default:
		caps.Recommendation = "⚙️ Other input device"
	}

	logger.Debugf("Device %s (%s): Movement=%v, Buttons=%v, Keyboard=%v",
		fullName, eventName, caps.HasMouseMovement, caps.HasMouseButtons, caps.HasKeyboard)

	return caps
}

// formatDeviceDescription creates a user-friendly description with capability hints
func (s *DeviceSelector) formatDeviceDescription(name, eventName string, caps DeviceCapabilities) string {
	// Determine if this is likely the main interface
	isMainInterface := !strings.Contains(strings.ToLower(name), "-if") &&
		!strings.Contains(strings.ToLower(name), "-event-")

	var mainIndicator string
	if isMainInterface && (caps.HasMouseMovement || caps.HasKeyboard) {
		mainIndicator = " [MAIN]"
	}

	return fmt.Sprintf("%s (%s)%s - %s", name, eventName, mainIndicator, caps.Recommendation)
}

// SelectMouseDeviceEnhanced presents an enhanced interactive selection for mouse devices
func (s *DeviceSelector) SelectMouseDeviceEnhanced() (string, error) {
	// Get all available input devices
	devices, err := s.ListDevices(DeviceTypeMouse) // deviceType ignored
	if err != nil {
		return "", err
	}

	if len(devices) == 0 {
		return "", fmt.Errorf("no input devices found")
	}

	// Convert to enhanced device info with capabilities
	var enhancedDevices []EnhancedDeviceInfo
	detector := NewDeviceDetector()

	for _, dev := range devices {
		if file, err := os.OpenFile(dev.Path, os.O_RDONLY, 0); err == nil {
			if detector.isValidInputDevice(file) {
				caps := s.analyzeCapabilities(file, dev.Name)
				enhanced := EnhancedDeviceInfo{
					Path:         dev.Path,
					Name:         dev.Name,
					Symlink:      dev.Symlink,
					Descriptive:  s.formatDeviceDescription(dev.Name, filepath.Base(dev.Path), caps),
					Capabilities: caps,
				}
				enhancedDevices = append(enhancedDevices, enhanced)
			}
			if err := file.Close(); err != nil {
			logger.Debugf("Failed to close file %s: %v", dev.Path, err)
		}
		}
	}

	// Filter and sort: prioritize mouse devices
	var mouseDevices, otherDevices []EnhancedDeviceInfo
	for _, dev := range enhancedDevices {
		// Only include devices that are actually recommended for mouse
		if dev.Capabilities.HasMouseMovement && strings.Contains(dev.Capabilities.Recommendation, "RECOMMENDED for MOUSE") {
			mouseDevices = append(mouseDevices, dev)
		} else {
			otherDevices = append(otherDevices, dev)
		}
	}

	// Combine: mouse devices first, then others
	allDevices := append(mouseDevices, otherDevices...)

	if len(allDevices) == 0 {
		return "", fmt.Errorf("no suitable input devices found")
	}

	// If only one mouse device, use it automatically
	if len(mouseDevices) == 1 {
		logger.Infof("Auto-selected mouse device: %s", mouseDevices[0].Name)
		return mouseDevices[0].Path, nil
	}

	// Create options for the select
	options := make([]huh.Option[string], len(allDevices))
	for i, dev := range allDevices {
		options[i] = huh.NewOption(dev.Descriptive, dev.Path)
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select Mouse Device").
				Description("Choose which device you want to use for MOUSE input capture.\nDevices marked [MAIN] are typically the primary interfaces.").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return "", fmt.Errorf("device selection cancelled: %w", err)
	}

	return selected, nil
}

// SelectKeyboardDeviceEnhanced presents an enhanced interactive selection for keyboard devices
func (s *DeviceSelector) SelectKeyboardDeviceEnhanced() (string, error) {
	// Get all available input devices
	devices, err := s.ListDevices(DeviceTypeKeyboard) // deviceType ignored
	if err != nil {
		return "", err
	}

	if len(devices) == 0 {
		return "", fmt.Errorf("no input devices found")
	}

	// Convert to enhanced device info with capabilities
	var enhancedDevices []EnhancedDeviceInfo
	detector := NewDeviceDetector()

	for _, dev := range devices {
		if file, err := os.OpenFile(dev.Path, os.O_RDONLY, 0); err == nil {
			if detector.isValidInputDevice(file) {
				caps := s.analyzeCapabilities(file, dev.Name)
				enhanced := EnhancedDeviceInfo{
					Path:         dev.Path,
					Name:         dev.Name,
					Symlink:      dev.Symlink,
					Descriptive:  s.formatDeviceDescription(dev.Name, filepath.Base(dev.Path), caps),
					Capabilities: caps,
				}
				enhancedDevices = append(enhancedDevices, enhanced)
			}
			if err := file.Close(); err != nil {
			logger.Debugf("Failed to close file %s: %v", dev.Path, err)
		}
		}
	}

	// Filter and sort: prioritize keyboard devices
	var keyboardDevices, otherDevices []EnhancedDeviceInfo
	for _, dev := range enhancedDevices {
		if dev.Capabilities.HasKeyboard {
			keyboardDevices = append(keyboardDevices, dev)
		} else {
			otherDevices = append(otherDevices, dev)
		}
	}

	// Combine: keyboard devices first, then others
	allDevices := append(keyboardDevices, otherDevices...)

	if len(allDevices) == 0 {
		return "", fmt.Errorf("no suitable input devices found")
	}

	// If only one keyboard device, use it automatically
	if len(keyboardDevices) == 1 {
		logger.Infof("Auto-selected keyboard device: %s", keyboardDevices[0].Name)
		return keyboardDevices[0].Path, nil
	}

	// Create options for the select
	options := make([]huh.Option[string], len(allDevices))
	for i, dev := range allDevices {
		options[i] = huh.NewOption(dev.Descriptive, dev.Path)
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select Keyboard Device").
				Description("Choose which device you want to use for KEYBOARD input capture.\nDevices marked [MAIN] are typically the primary interfaces.").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return "", fmt.Errorf("device selection cancelled: %w", err)
	}

	return selected, nil
}
