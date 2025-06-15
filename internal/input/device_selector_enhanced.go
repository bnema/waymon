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

// analyzeCapabilities analyzes what a device can do based on its name and properties
func (s *DeviceSelector) analyzeCapabilities(file *os.File, deviceName string) DeviceCapabilities {
	caps := DeviceCapabilities{}
	
	// Get device name from sysfs for more accurate detection
	eventName := filepath.Base(file.Name())
	sysPath := fmt.Sprintf("/sys/class/input/%s/device/name", eventName)
	
	var fullName string
	if data, err := os.ReadFile(sysPath); err == nil {
		fullName = strings.TrimSpace(string(data))
	} else {
		fullName = deviceName
	}
	
	fullNameLower := strings.ToLower(fullName)
	deviceNameLower := strings.ToLower(deviceName)
	
	// Detect mouse capabilities
	if strings.Contains(fullNameLower, "mouse") || 
	   strings.Contains(deviceNameLower, "mouse") ||
	   strings.Contains(fullNameLower, "pulsar") ||
	   strings.Contains(deviceNameLower, "pulsar") ||
	   strings.Contains(fullNameLower, "logitech") ||
	   strings.Contains(deviceNameLower, "logitech") {
		caps.HasMouseMovement = true
		caps.HasMouseButtons = true
	}
	
	// Detect keyboard capabilities
	if strings.Contains(fullNameLower, "keyboard") ||
	   strings.Contains(deviceNameLower, "keyboard") ||
	   strings.Contains(fullNameLower, "lofree") ||
	   strings.Contains(deviceNameLower, "lofree") ||
	   strings.Contains(fullNameLower, "compx") ||
	   strings.Contains(deviceNameLower, "compx") {
		caps.HasKeyboard = true
	}
	
	// Generate recommendations
	if caps.HasMouseMovement && caps.HasMouseButtons && !caps.HasKeyboard {
		caps.Recommendation = "🖱️ RECOMMENDED for MOUSE"
	} else if caps.HasKeyboard && !caps.HasMouseMovement {
		caps.Recommendation = "⌨️ RECOMMENDED for KEYBOARD"
	} else if caps.HasKeyboard && caps.HasMouseMovement {
		caps.Recommendation = "🔄 COMBO device (mouse or keyboard)"
	} else {
		caps.Recommendation = "⚙️ System device"
	}
	
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
			file.Close()
		}
	}

	// Filter and sort: prioritize mouse devices
	var mouseDevices, otherDevices []EnhancedDeviceInfo
	for _, dev := range enhancedDevices {
		if dev.Capabilities.HasMouseMovement {
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
			file.Close()
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