package input

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"github.com/bnema/waymon/internal/logger"
)

// ioctl macro helpers
const (
	_IOC_READ  = 2
	_IOC_WRITE = 1
)

// EVIOCGBIT calculates the ioctl value for getting event bits
func EVIOCGBIT(ev uint, len int) uintptr {
	// _IOC(_IOC_READ, 'E', 0x20 + (ev), len)
	return uintptr(((_IOC_READ) << 30) | (('E') << 8) | (0x20 + int(ev)) | ((len) << 16))
}

// Event types
const (
	EV_SYN = 0x00
	EV_KEY = 0x01
	EV_REL = 0x02
	EV_ABS = 0x03
)

// Relative axes
const (
	REL_X     = 0x00
	REL_Y     = 0x01
	REL_WHEEL = 0x08
)

// Mouse buttons
const (
	BTN_LEFT   = 0x110
	BTN_RIGHT  = 0x111
	BTN_MIDDLE = 0x112
	BTN_SIDE   = 0x113
	BTN_EXTRA  = 0x114
)

// DeviceType represents the type of input device
type DeviceType int

const (
	DeviceTypeKeyboard DeviceType = iota
	DeviceTypeMouse
)

// DeviceDetector provides common device detection functionality
type DeviceDetector struct {
	deviceType DeviceType
}

// NewDeviceDetector creates a new device detector for the specified type
func NewDeviceDetector(deviceType DeviceType) *DeviceDetector {
	return &DeviceDetector{
		deviceType: deviceType,
	}
}

// FindDevicesBySymlinks finds devices using /dev/input/by-id and /dev/input/by-path symlinks
func (d *DeviceDetector) FindDevicesBySymlinks() ([]*os.File, error) {
	var files []*os.File

	// Try by-id first (most descriptive)
	byIdDir := "/dev/input/by-id"
	if entries, err := os.ReadDir(byIdDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && d.matchesDevicePattern(entry.Name()) {
				symlinkPath := fmt.Sprintf("%s/%s", byIdDir, entry.Name())
				if targetPath, err := os.Readlink(symlinkPath); err == nil {
					realPath := fmt.Sprintf("/dev/input/%s", targetPath[3:]) // Remove "../" prefix

					if file, err := os.OpenFile(realPath, os.O_RDONLY, 0); err == nil {
						// Double-check that it's the correct device type
						if d.isCorrectDeviceType(file) {
							files = append(files, file)
							logger.Debugf("Found %s via by-id: %s -> %s", d.getDeviceTypeName(), entry.Name(), realPath)
						} else {
							file.Close()
						}
					} else {
						logger.Debugf("Cannot open %s device %s: %v", d.getDeviceTypeName(), realPath, err)
					}
				}
			}
		}
	}

	// Also try by-path as fallback
	if len(files) == 0 {
		byPathDir := "/dev/input/by-path"
		if entries, err := os.ReadDir(byPathDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && d.matchesDevicePattern(entry.Name()) {
					symlinkPath := fmt.Sprintf("%s/%s", byPathDir, entry.Name())
					if targetPath, err := os.Readlink(symlinkPath); err == nil {
						realPath := fmt.Sprintf("/dev/input/%s", targetPath[3:]) // Remove "../" prefix

						if file, err := os.OpenFile(realPath, os.O_RDONLY, 0); err == nil {
							// Double-check that it's the correct device type
							if d.isCorrectDeviceType(file) {
								files = append(files, file)
								logger.Debugf("Found %s via by-path: %s -> %s", d.getDeviceTypeName(), entry.Name(), realPath)
							} else {
								file.Close()
							}
						} else {
							logger.Debugf("Cannot open %s device %s: %v", d.getDeviceTypeName(), realPath, err)
						}
					}
				}
			}
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no %s devices found via symlinks", d.getDeviceTypeName())
	}

	return files, nil
}

// FindDevicesByCapabilities scans all event devices and uses capability detection
func (d *DeviceDetector) FindDevicesByCapabilities() ([]*os.File, error) {
	eventDir := "/dev/input"
	entries, err := os.ReadDir(eventDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read /dev/input: %w", err)
	}

	var files []*os.File
	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) > 5 && entry.Name()[:5] == "event" {
			path := fmt.Sprintf("%s/%s", eventDir, entry.Name())

			file, err := os.OpenFile(path, os.O_RDONLY, 0)
			if err != nil {
				logger.Debugf("Cannot open %s: %v", path, err)
				continue
			}

			// Check if this is the correct device type
			if d.isCorrectDeviceType(file) {
				files = append(files, file)
				logger.Debugf("Found %s device: %s", d.getDeviceTypeName(), path)
			} else {
				file.Close()
				logger.Debugf("Skipping non-%s device: %s", d.getDeviceTypeName(), path)
			}
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no %s devices found via capability scanning", d.getDeviceTypeName())
	}

	return files, nil
}

// SupportsEventType checks if a device supports a specific event type (minimal check for fallback)
func SupportsEventType(file *os.File, eventType uint8) bool {
	// Get supported event types
	eventTypes := make([]byte, 32) // EV_MAX/8 + 1
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		file.Fd(),
		EVIOCGBIT(0, len(eventTypes)),
		uintptr(unsafe.Pointer(&eventTypes[0]))); errno != 0 {
		return false
	}

	return (eventTypes[eventType/8] & (1 << (eventType % 8))) != 0
}

// matchesDevicePattern checks if a device name matches patterns for the device type
func (d *DeviceDetector) matchesDevicePattern(name string) bool {
	nameLower := strings.ToLower(name)

	switch d.deviceType {
	case DeviceTypeKeyboard:
		return strings.HasSuffix(nameLower, "event-kbd") ||
			strings.HasSuffix(nameLower, "kbd") ||
			strings.Contains(nameLower, "keyboard")
	case DeviceTypeMouse:
		mouseKeywords := []string{"mouse", "event-mouse", "optical", "wireless", "trackball", "trackpad", "touchpad"}
		for _, keyword := range mouseKeywords {
			if strings.Contains(nameLower, keyword) {
				return true
			}
		}
		return strings.HasSuffix(nameLower, "event-mouse") ||
			strings.Contains(nameLower, "pointer") ||
			strings.Contains(nameLower, "pointing")
	default:
		return false
	}
}

// isCorrectDeviceType checks if a device matches the expected device type
func (d *DeviceDetector) isCorrectDeviceType(file *os.File) bool {
	switch d.deviceType {
	case DeviceTypeKeyboard:
		return isKeyboardDevice(file)
	case DeviceTypeMouse:
		return isMouseDevice(file)
	default:
		return false
	}
}

// getDeviceTypeName returns a human-readable name for the device type
func (d *DeviceDetector) getDeviceTypeName() string {
	switch d.deviceType {
	case DeviceTypeKeyboard:
		return "keyboard"
	case DeviceTypeMouse:
		return "mouse"
	default:
		return "unknown"
	}
}

// HasMouseButtons checks if a device has mouse button capabilities
func (d *DeviceDetector) HasMouseButtons(file *os.File) bool {
	keyBits := make([]byte, 96) // KEY_MAX/8 + 1
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		file.Fd(),
		EVIOCGBIT(uint(EV_KEY), len(keyBits)),
		uintptr(unsafe.Pointer(&keyBits[0]))); errno != 0 {
		return false
	}

	// Check for common mouse buttons
	mouseButtons := []uint16{BTN_LEFT, BTN_RIGHT, BTN_MIDDLE, BTN_SIDE, BTN_EXTRA}
	for _, btn := range mouseButtons {
		if (keyBits[btn/8] & (1 << (btn % 8))) != 0 {
			return true
		}
	}

	return false
}
