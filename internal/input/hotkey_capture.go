package input

import (
	"context"
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"github.com/bnema/waymon/internal/logger"
)

// HotkeyCapture captures keyboard events to detect hotkeys for switching
type HotkeyCapture struct {
	mu         sync.Mutex
	capturing  bool
	eventFiles []*os.File
	wg         sync.WaitGroup
	cancel     context.CancelFunc

	// Hotkey configuration
	modifiers uint32 // Bitmask of required modifiers
	keyCode   uint16 // Key code to trigger

	// Callback when hotkey is pressed
	onHotkey func()
}

// Modifier masks
const (
	ModCtrl  = 1 << 0
	ModAlt   = 1 << 1
	ModShift = 1 << 2
	ModSuper = 1 << 3
)

// NewHotkeyCapture creates a new hotkey capture instance
func NewHotkeyCapture(modifiers uint32, keyCode uint16, onHotkey func()) *HotkeyCapture {
	return &HotkeyCapture{
		modifiers:  modifiers,
		keyCode:    keyCode,
		onHotkey:   onHotkey,
		eventFiles: make([]*os.File, 0),
	}
}

// Start begins capturing keyboard events
func (h *HotkeyCapture) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.capturing {
		return nil
	}

	// Find keyboard devices
	if err := h.findKeyboardDevices(); err != nil {
		return fmt.Errorf("failed to find keyboard devices: %w", err)
	}

	if len(h.eventFiles) == 0 {
		return fmt.Errorf("no keyboard devices found")
	}

	h.capturing = true
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel

	// Start capture goroutines
	for _, file := range h.eventFiles {
		h.wg.Add(1)
		go h.captureEvents(ctx, file)
	}

	logger.Infof("Started hotkey capture on %d devices", len(h.eventFiles))
	return nil
}

// Stop stops capturing keyboard events
func (h *HotkeyCapture) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.capturing {
		return
	}

	h.capturing = false
	if h.cancel != nil {
		h.cancel()
	}

	h.wg.Wait()

	for _, file := range h.eventFiles {
		file.Close()
	}
	h.eventFiles = h.eventFiles[:0]

	logger.Info("Stopped hotkey capture")
}

// findKeyboardDevices finds available keyboard input devices
func (h *HotkeyCapture) findKeyboardDevices() error {
	var permissionErrors int
	var totalDevices int

	// Method 1: Use /dev/input/by-id symlinks to find keyboards (most reliable)
	detector := NewDeviceDetector(DeviceTypeKeyboard)
	if files, err := detector.FindDevicesBySymlinks(); err == nil && len(files) > 0 {
		h.eventFiles = files
		logger.Infof("Found %d keyboard devices using symlinks", len(h.eventFiles))
		return nil
	}

	// Method 2: Fallback to scanning all /dev/input/event* devices with capability detection
	logger.Debug("Symlink detection failed, falling back to capability scanning")
	if files, err := detector.FindDevicesByCapabilities(); err == nil && len(files) > 0 {
		h.eventFiles = files
		logger.Infof("Found %d keyboard devices using capability detection", len(h.eventFiles))
		return nil
	}

	// Method 3: Last resort - use ALL input devices (for situations where detection fails)
	logger.Warn("Keyboard-specific detection failed, using all input devices as fallback")
	eventDir := "/dev/input"
	entries, err := os.ReadDir(eventDir)
	if err != nil {
		return fmt.Errorf("failed to read /dev/input: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) > 5 && entry.Name()[:5] == "event" {
			totalDevices++
			path := fmt.Sprintf("%s/%s", eventDir, entry.Name())

			file, err := os.OpenFile(path, os.O_RDONLY, 0)
			if err != nil {
				if os.IsPermission(err) {
					permissionErrors++
					logger.Debugf("Cannot access %s: %v", path, err)
				} else {
					logger.Debugf("Cannot open %s: %v", path, err)
				}
				continue
			}

			// In fallback mode, accept any device that supports EV_KEY events
			if SupportsEventType(file, EV_KEY) {
				h.eventFiles = append(h.eventFiles, file)
				logger.Debugf("Using input device in fallback mode: %s", path)
			} else {
				file.Close()
			}
		}
	}

	// If we found devices but couldn't access any due to permissions, provide helpful error
	if len(h.eventFiles) == 0 && permissionErrors > 0 {
		return fmt.Errorf("found %d input devices but cannot access them due to permission denied. Run 'waymon setup' or ensure user is in 'input' group", totalDevices)
	}

	if len(h.eventFiles) > 0 {
		logger.Warnf("Using %d input devices in fallback mode - this may include non-keyboard devices", len(h.eventFiles))
	}

	return nil
}

// Linux keyboard input event structure
type keyInputEvent struct {
	Time  syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

// Linux input event types and constants
const (
	EV_KEY_EVENT = 0x01
)

// Key codes for modifiers
const (
	KEY_LEFTCTRL  = 29
	KEY_LEFTALT   = 56
	KEY_LEFTSHIFT = 42
	KEY_LEFTMETA  = 125 // Super/Windows key
	KEY_S         = 31  // Default hotkey
)

// isKeyboardDevice checks if a device has keyboard capabilities (evtest-style detection)
func isKeyboardDevice(file *os.File) bool {
	// Get supported event types
	eventTypes := make([]byte, 32) // EV_MAX/8 + 1
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		file.Fd(),
		EVIOCGBIT(0, len(eventTypes)),
		uintptr(unsafe.Pointer(&eventTypes[0]))); errno != 0 {
		return false
	}

	// Check if device supports EV_KEY events (bit 1)
	evKeyBit := uint8(1) // EV_KEY = 0x01
	if (eventTypes[evKeyBit/8] & (1 << (evKeyBit % 8))) == 0 {
		return false
	}

	// Get supported keys
	keyBits := make([]byte, 96) // KEY_MAX/8 + 1 (assuming KEY_MAX < 768)
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		file.Fd(),
		EVIOCGBIT(1, len(keyBits)), // EV_KEY = 1
		uintptr(unsafe.Pointer(&keyBits[0]))); errno != 0 {
		return false
	}

	// Enhanced keyboard detection: check for multiple key ranges like evtest does

	// Check for letter keys (QWERTY layout)
	hasLetterKeys := false
	for keyCode := 16; keyCode <= 50; keyCode++ { // Q to P, A to L, Z to M
		if (keyBits[keyCode/8] & (1 << (keyCode % 8))) != 0 {
			hasLetterKeys = true
			break
		}
	}

	// Check for number keys (1-9, 0)
	hasNumberKeys := false
	for keyCode := 2; keyCode <= 11; keyCode++ { // 1-9, 0
		if (keyBits[keyCode/8] & (1 << (keyCode % 8))) != 0 {
			hasNumberKeys = true
			break
		}
	}

	// Check for common modifier keys
	hasModifierKeys := false
	modifierKeys := []int{29, 42, 54, 56, 125} // CTRL, SHIFT, ALT, SUPER
	for _, keyCode := range modifierKeys {
		if (keyBits[keyCode/8] & (1 << (uint(keyCode) % 8))) != 0 {
			hasModifierKeys = true
			break
		}
	}

	// Check for function keys (F1-F12)
	hasFunctionKeys := false
	for keyCode := 59; keyCode <= 70; keyCode++ { // F1-F12
		if (keyBits[keyCode/8] & (1 << (uint(keyCode) % 8))) != 0 {
			hasFunctionKeys = true
			break
		}
	}

	// Check for space, enter, backspace (essential keys)
	hasEssentialKeys := false
	essentialKeys := []int{57, 28, 14} // SPACE, ENTER, BACKSPACE
	for _, keyCode := range essentialKeys {
		if (keyBits[keyCode/8] & (1 << (uint(keyCode) % 8))) != 0 {
			hasEssentialKeys = true
			break
		}
	}

	// A device is considered a keyboard if it has:
	// 1. Letter keys (most important), OR
	// 2. At least 2 of: number keys, modifier keys, function keys, essential keys
	if hasLetterKeys {
		return true
	}

	keyTypeCount := 0
	if hasNumberKeys {
		keyTypeCount++
	}
	if hasModifierKeys {
		keyTypeCount++
	}
	if hasFunctionKeys {
		keyTypeCount++
	}
	if hasEssentialKeys {
		keyTypeCount++
	}

	return keyTypeCount >= 2
}

// captureEvents captures events from a single device
func (h *HotkeyCapture) captureEvents(ctx context.Context, file *os.File) {
	defer h.wg.Done()

	eventSize := int(unsafe.Sizeof(keyInputEvent{}))
	buffer := make([]byte, eventSize)

	var currentModifiers uint32

	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := file.Read(buffer)
			if err != nil {
				logger.Errorf("Error reading from device: %v", err)
				return
			}

			if n != eventSize {
				continue
			}

			event := (*keyInputEvent)(unsafe.Pointer(&buffer[0]))

			if event.Type == EV_KEY_EVENT {
				// Track modifier state
				switch event.Code {
				case KEY_LEFTCTRL:
					if event.Value != 0 {
						currentModifiers |= ModCtrl
					} else {
						currentModifiers &^= ModCtrl
					}
				case KEY_LEFTALT:
					if event.Value != 0 {
						currentModifiers |= ModAlt
					} else {
						currentModifiers &^= ModAlt
					}
				case KEY_LEFTSHIFT:
					if event.Value != 0 {
						currentModifiers |= ModShift
					} else {
						currentModifiers &^= ModShift
					}
				case KEY_LEFTMETA:
					if event.Value != 0 {
						currentModifiers |= ModSuper
					} else {
						currentModifiers &^= ModSuper
					}
				default:
					// Check if hotkey is pressed
					if event.Code == h.keyCode && event.Value != 0 {
						if currentModifiers == h.modifiers && h.onHotkey != nil {
							logger.Info("Hotkey detected!")
							h.onHotkey()
						}
					}
				}
			}
		}
	}
}
