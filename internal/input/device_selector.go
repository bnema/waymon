package input

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bnema/waymon/internal/logger"
	"github.com/charmbracelet/huh"
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
	// Get all available input devices
	devices, err := s.ListDevices(DeviceTypeMouse) // deviceType ignored
	if err != nil {
		return "", err
	}

	if len(devices) == 0 {
		return "", fmt.Errorf("no input devices found")
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
				Description("Choose which device you want to use for MOUSE input capture").
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
	// Get all available input devices (same list as mouse)
	devices, err := s.ListDevices(DeviceTypeKeyboard) // deviceType ignored
	if err != nil {
		return "", err
	}

	if len(devices) == 0 {
		return "", fmt.Errorf("no input devices found")
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
				Description("Choose which device you want to use for KEYBOARD input capture").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return "", fmt.Errorf("device selection cancelled: %w", err)
	}

	return selected, nil
}

// ListDevices returns a list of all available input devices (ignoring deviceType for simplicity)
func (s *DeviceSelector) ListDevices(deviceType DeviceType) ([]DeviceInfo, error) {
	var devices []DeviceInfo
	detector := NewDeviceDetector()

	// First try to find devices via symlinks (more descriptive names)
	symlinkDevices := s.findDevicesBySymlinks()
	devices = append(devices, symlinkDevices...)

	// Also find devices by capabilities (might catch some that symlinks miss)
	capDevices, err := s.findDevicesByCapabilities(detector)
	if err == nil {
		// Add only devices not already in the list
		for _, capDev := range capDevices {
			found := false
			for _, existing := range devices {
				if existing.Path == capDev.Path {
					found = true
					break
				}
			}
			if !found {
				devices = append(devices, capDev)
			}
		}
	}

	return devices, nil
}

// findDevicesBySymlinks finds all input devices using symlinks for better names
func (s *DeviceSelector) findDevicesBySymlinks() []DeviceInfo {
	var devices []DeviceInfo
	detector := NewDeviceDetector()

	// Check /dev/input/by-id first (most descriptive)
	byIdDir := "/dev/input/by-id"
	if entries, err := os.ReadDir(byIdDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.Contains(strings.ToLower(entry.Name()), "event") {
				symlinkPath := filepath.Join(byIdDir, entry.Name())
				if targetPath, err := os.Readlink(symlinkPath); err == nil {
					// Resolve to absolute path
					realPath := filepath.Clean(filepath.Join(byIdDir, targetPath))

					// Verify the device is accessible
					if file, err := os.OpenFile(realPath, os.O_RDONLY, 0); err == nil {
						if detector.isValidInputDevice(file) {
							// Extract device name from symlink
							name := entry.Name()
							// Remove common prefixes/suffixes for cleaner display
							cleanName := strings.TrimSuffix(name, "-event-kbd")
							cleanName = strings.TrimSuffix(cleanName, "-event-mouse")
							cleanName = strings.TrimPrefix(cleanName, "usb-")

							devices = append(devices, DeviceInfo{
								Path:        realPath,
								Name:        cleanName,
								Symlink:     symlinkPath,
								Descriptive: fmt.Sprintf("%s (%s)", cleanName, filepath.Base(realPath)),
							})
						}
						file.Close()
					}
				}
			}
		}
	}

	// Also check /dev/input/by-path if by-id didn't find anything
	if len(devices) == 0 {
		byPathDir := "/dev/input/by-path"
		if entries, err := os.ReadDir(byPathDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.Contains(strings.ToLower(entry.Name()), "event") {
					symlinkPath := filepath.Join(byPathDir, entry.Name())
					if targetPath, err := os.Readlink(symlinkPath); err == nil {
						realPath := filepath.Clean(filepath.Join(byPathDir, targetPath))

						if file, err := os.OpenFile(realPath, os.O_RDONLY, 0); err == nil {
							if detector.isValidInputDevice(file) {
								name := entry.Name()
								cleanName := strings.TrimSuffix(name, "-event-kbd")
								cleanName = strings.TrimSuffix(cleanName, "-event-mouse")

								devices = append(devices, DeviceInfo{
									Path:        realPath,
									Name:        cleanName,
									Symlink:     symlinkPath,
									Descriptive: fmt.Sprintf("%s (%s)", cleanName, filepath.Base(realPath)),
								})
							}
							file.Close()
						}
					}
				}
			}
		}
	}

	return devices
}

// findDevicesByCapabilities finds all valid input devices by scanning capabilities
func (s *DeviceSelector) findDevicesByCapabilities(detector *DeviceDetector) ([]DeviceInfo, error) {
	eventDir := "/dev/input"
	entries, err := os.ReadDir(eventDir)
	if err != nil {
		return nil, err
	}

	var devices []DeviceInfo
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "event") {
			path := filepath.Join(eventDir, entry.Name())

			if file, err := os.OpenFile(path, os.O_RDONLY, 0); err == nil {
				if detector.isValidInputDevice(file) {
					// Try to get a better name from the device
					name := s.getDeviceName(path)
					if name == "" {
						name = entry.Name()
					}

					devices = append(devices, DeviceInfo{
						Path:        path,
						Name:        name,
						Symlink:     "",
						Descriptive: fmt.Sprintf("%s (%s)", name, entry.Name()),
					})
				}
				file.Close()
			}
		}
	}

	logger.Infof("Found %d devices by capabilities", len(devices))

	return devices, nil
}

// getDeviceName tries to get a descriptive name for the device
func (s *DeviceSelector) getDeviceName(path string) string {
	// Try to read the device name from /sys/class/input/eventX/device/name
	eventName := filepath.Base(path)
	sysPath := fmt.Sprintf("/sys/class/input/%s/device/name", eventName)

	if data, err := os.ReadFile(sysPath); err == nil {
		return strings.TrimSpace(string(data))
	}

	return ""
}
