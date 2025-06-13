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
	mu           sync.Mutex
	capturing    bool
	eventFiles   []*os.File
	wg           sync.WaitGroup
	cancel       context.CancelFunc
	
	// Hotkey configuration
	modifiers    uint32 // Bitmask of required modifiers
	keyCode      uint16 // Key code to trigger
	
	// Callback when hotkey is pressed
	onHotkey     func()
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
		modifiers: modifiers,
		keyCode:   keyCode,
		onHotkey:  onHotkey,
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
	// Similar to mouse capture, but look for keyboard devices
	eventDir := "/dev/input"
	entries, err := os.ReadDir(eventDir)
	if err != nil {
		return fmt.Errorf("failed to read /dev/input: %w", err)
	}
	
	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) > 5 && entry.Name()[:5] == "event" {
			path := fmt.Sprintf("%s/%s", eventDir, entry.Name())
			
			file, err := os.OpenFile(path, os.O_RDONLY, 0)
			if err != nil {
				logger.Debugf("Cannot access %s: %v", path, err)
				continue
			}
			
			// For now, accept all devices (in production, check for EV_KEY capability)
			h.eventFiles = append(h.eventFiles, file)
			logger.Debugf("Found input device: %s", path)
		}
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

// Key event type
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