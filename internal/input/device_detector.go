package input

import (
	"os"
	"syscall"
	"unsafe"
)

// DeviceDetector provides device detection capabilities
type DeviceDetector struct{}

// NewDeviceDetector creates a new device detector
func NewDeviceDetector() *DeviceDetector {
	return &DeviceDetector{}
}

// isValidInputDevice checks if an open file descriptor is a valid input device
func (d *DeviceDetector) isValidInputDevice(file *os.File) bool {
	capabilities := d.GetDeviceCapabilities(file)

	// Must have some kind of input capability (keys or relative movement)
	_, hasKeys := capabilities[0x01] // EV_KEY
	_, hasRel := capabilities[0x02]  // EV_REL

	return hasKeys || hasRel
}

// GetDeviceCapabilities gets the capabilities of an evdev device
func (d *DeviceDetector) GetDeviceCapabilities(file *os.File) map[int][]int {
	capabilities := make(map[int][]int)

	// EVIOCGBIT(ev, len) - get event type bits
	// We check for EV_KEY (0x01), EV_REL (0x02), etc.

	// Check for EV_KEY capabilities (buttons and keys)
	keyBits := make([]byte, 96) // KEY_MAX/8 + 1
	if d.getEventTypeBits(file, 0x01, keyBits) {
		var keys []int
		for i := 0; i < len(keyBits)*8; i++ {
			if keyBits[i/8]&(1<<(i%8)) != 0 {
				keys = append(keys, i)
			}
		}
		if len(keys) > 0 {
			capabilities[0x01] = keys
		}
	}

	// Check for EV_REL capabilities (relative movement)
	relBits := make([]byte, 2) // REL_MAX is small
	if d.getEventTypeBits(file, 0x02, relBits) {
		var rels []int
		for i := 0; i < len(relBits)*8; i++ {
			if relBits[i/8]&(1<<(i%8)) != 0 {
				rels = append(rels, i)
			}
		}
		if len(rels) > 0 {
			capabilities[0x02] = rels
		}
	}

	return capabilities
}

// getEventTypeBits uses ioctl to get event type capabilities
func (d *DeviceDetector) getEventTypeBits(file *os.File, eventType int, bits []byte) bool {
	// EVIOCGBIT(ev, len) = _IOC(_IOC_READ, 'E', 0x20 + ev, len)
	cmd := 0x80000000 | (uintptr(len(bits)) << 16) | (uintptr('E') << 8) | uintptr(0x20+eventType)

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		file.Fd(),
		cmd,
		uintptr(unsafe.Pointer(&bits[0])), //nolint:gosec // required for ioctl syscall
	)

	return errno == 0
}
