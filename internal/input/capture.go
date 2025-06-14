package input

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/proto"
)

// MouseCapture handles capturing mouse events from the system
type MouseCapture struct {
	mu           sync.Mutex
	capturing    bool
	eventFiles   []*os.File
	edgeDetector *EdgeDetector
	wg           sync.WaitGroup
	cancel       context.CancelFunc
}

// NewMouseCapture creates a new mouse capture instance
func NewMouseCapture(edgeDetector *EdgeDetector) *MouseCapture {
	return &MouseCapture{
		edgeDetector: edgeDetector,
		eventFiles:   make([]*os.File, 0),
	}
}

// Start begins capturing mouse events
func (m *MouseCapture) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.capturing {
		return nil
	}

	// Find mouse event devices
	if err := m.findMouseDevices(); err != nil {
		return fmt.Errorf("failed to find mouse devices: %w", err)
	}

	if len(m.eventFiles) == 0 {
		return fmt.Errorf("no mouse devices found")
	}

	m.capturing = true
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	// Start capture goroutines for each device
	for _, file := range m.eventFiles {
		m.wg.Add(1)
		go m.captureEvents(ctx, file)
	}

	logger.Infof("Started mouse capture on %d devices", len(m.eventFiles))
	return nil
}

// Stop stops capturing mouse events
func (m *MouseCapture) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.capturing {
		return
	}

	m.capturing = false
	if m.cancel != nil {
		m.cancel()
	}

	// Wait for goroutines to finish
	m.wg.Wait()

	// Close event files
	for _, file := range m.eventFiles {
		file.Close()
	}
	m.eventFiles = m.eventFiles[:0]

	logger.Info("Stopped mouse capture")
}

// findMouseDevices finds available mouse input devices
func (m *MouseCapture) findMouseDevices() error {
	var permissionErrors int
	var totalDevices int

	// Method 1: Use /dev/input/by-id symlinks to find mice (most reliable)
	detector := NewDeviceDetector(DeviceTypeMouse)
	if files, err := detector.FindDevicesBySymlinks(); err == nil && len(files) > 0 {
		m.eventFiles = files
		logger.Infof("Found %d mouse devices using symlinks", len(m.eventFiles))
		return nil
	}

	// Method 2: Fallback to scanning all /dev/input/event* devices with capability detection
	logger.Debug("Symlink detection failed, falling back to capability scanning")
	if files, err := detector.FindDevicesByCapabilities(); err == nil && len(files) > 0 {
		m.eventFiles = files
		logger.Infof("Found %d mouse devices using capability detection", len(m.eventFiles))
		return nil
	}

	// Method 3: Last resort - use ALL input devices (for situations where detection fails)
	logger.Warn("Mouse-specific detection failed, using all input devices as fallback")
	eventDir := "/dev/input"
	entries, err := os.ReadDir(eventDir)
	if err != nil {
		return fmt.Errorf("failed to read /dev/input: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) > 5 && entry.Name()[:5] == "event" {
			totalDevices++
			path := filepath.Join(eventDir, entry.Name())

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

			// In fallback mode, accept any device with relative movement or button capabilities
			if SupportsEventType(file, EV_REL) || SupportsEventType(file, EV_KEY) {
				m.eventFiles = append(m.eventFiles, file)
				logger.Debugf("Using input device in fallback mode: %s", path)
			} else {
				file.Close()
			}
		}
	}

	// If we found devices but couldn't access any due to permissions, provide helpful error
	if len(m.eventFiles) == 0 && permissionErrors > 0 {
		return fmt.Errorf("found %d input devices but cannot access them due to permission denied. Run 'waymon setup' or ensure user is in 'input' group", totalDevices)
	}

	if len(m.eventFiles) > 0 {
		logger.Warnf("Using %d input devices in fallback mode - this may include non-mouse devices", len(m.eventFiles))
	}

	return nil
}

// isMouseDevice checks if a device has mouse capabilities (evtest-style detection)
func isMouseDevice(file *os.File) bool {
	// Get supported event types
	eventTypes := make([]byte, 32) // EV_MAX/8 + 1
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		file.Fd(),
		EVIOCGBIT(0, len(eventTypes)),
		uintptr(unsafe.Pointer(&eventTypes[0]))); errno != 0 {
		return false
	}

	// Check if device supports EV_REL (relative movement) - primary mouse indicator
	supportsRel := (eventTypes[EV_REL/8] & (1 << (EV_REL % 8))) != 0
	supportsKey := (eventTypes[EV_KEY/8] & (1 << (EV_KEY % 8))) != 0

	// If device doesn't support relative movement or keys, it's not a mouse
	if !supportsRel && !supportsKey {
		return false
	}

	// If it supports relative movement, check for X/Y axes
	if supportsRel {
		relBits := make([]byte, 12) // REL_MAX/8 + 1
		if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
			file.Fd(),
			EVIOCGBIT(uint(EV_REL), len(relBits)),
			uintptr(unsafe.Pointer(&relBits[0]))); errno != 0 {
			return false
		}

		// Check for X and Y relative movement
		hasRelX := (relBits[REL_X/8] & (1 << (REL_X % 8))) != 0
		hasRelY := (relBits[REL_Y/8] & (1 << (REL_Y % 8))) != 0

		if hasRelX && hasRelY {
			// This is likely a mouse - check for mouse buttons to be sure
			if supportsKey {
				detector := NewDeviceDetector(DeviceTypeMouse)
				return detector.HasMouseButtons(file)
			}
			return true // Relative X/Y movement is a strong indicator
		}
	}

	// If it only supports keys, check if they're mouse buttons
	if supportsKey && !supportsRel {
		detector := NewDeviceDetector(DeviceTypeMouse)
		return detector.HasMouseButtons(file)
	}

	return false
}

// Linux input event structure
type inputEvent struct {
	Time  syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

// captureEvents captures events from a single device
func (m *MouseCapture) captureEvents(ctx context.Context, file *os.File) {
	defer m.wg.Done()

	eventSize := int(unsafe.Sizeof(inputEvent{}))
	buffer := make([]byte, eventSize)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Read event
			n, err := file.Read(buffer)
			if err != nil {
				logger.Errorf("Error reading from device: %v", err)
				return
			}

			if n != eventSize {
				continue
			}

			// Parse event
			event := (*inputEvent)(unsafe.Pointer(&buffer[0]))

			// Only process events when edge detector is capturing
			if m.edgeDetector.IsCapturing() {
				m.processEvent(event)
			}
		}
	}
}

// processEvent processes a single input event
func (m *MouseCapture) processEvent(event *inputEvent) {
	switch event.Type {
	case EV_REL:
		// Relative movement
		switch event.Code {
		case REL_X:
			m.edgeDetector.HandleMouseMove(event.Value, 0)
		case REL_Y:
			m.edgeDetector.HandleMouseMove(0, event.Value)
		case REL_WHEEL:
			direction := proto.ScrollDirection_SCROLL_DIRECTION_DOWN
			if event.Value > 0 {
				direction = proto.ScrollDirection_SCROLL_DIRECTION_UP
			}
			m.edgeDetector.HandleMouseScroll(direction, event.Value)
		}

	case EV_KEY:
		// Button events
		button := proto.MouseButton_MOUSE_BUTTON_UNSPECIFIED
		switch event.Code {
		case BTN_LEFT:
			button = proto.MouseButton_MOUSE_BUTTON_LEFT
		case BTN_RIGHT:
			button = proto.MouseButton_MOUSE_BUTTON_RIGHT
		case BTN_MIDDLE:
			button = proto.MouseButton_MOUSE_BUTTON_MIDDLE
		case BTN_SIDE:
			button = proto.MouseButton_MOUSE_BUTTON_BACK
		case BTN_EXTRA:
			button = proto.MouseButton_MOUSE_BUTTON_FORWARD
		}

		if button != proto.MouseButton_MOUSE_BUTTON_UNSPECIFIED {
			pressed := event.Value != 0
			m.edgeDetector.HandleMouseButton(button, pressed)
		}
	}
}
