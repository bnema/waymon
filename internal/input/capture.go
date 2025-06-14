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
	// Look for event devices
	eventDir := "/dev/input"
	entries, err := os.ReadDir(eventDir)
	if err != nil {
		return fmt.Errorf("failed to read /dev/input: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) > 5 && entry.Name()[:5] == "event" {
			path := filepath.Join(eventDir, entry.Name())

			// Try to open the device (read-only is sufficient for capture)
			file, err := os.OpenFile(path, os.O_RDONLY, 0)
			if err != nil {
				// Skip devices we can't access
				logger.Debugf("Cannot access %s: %v", path, err)
				continue
			}

			// Check if this is a mouse device using ioctl
			if m.isMouseDevice(file) {
				m.eventFiles = append(m.eventFiles, file)
				logger.Debugf("Found mouse device: %s", path)
			} else {
				file.Close()
			}
		}
	}

	return nil
}

// isMouseDevice checks if a device is a mouse using ioctl
func (m *MouseCapture) isMouseDevice(file *os.File) bool {
	// Get device capabilities
	// This is a simplified check - in production, you'd want to use proper ioctl calls
	// to check EV_REL (relative movement) and EV_KEY (buttons) capabilities

	// For now, we'll accept any device that might be a mouse
	// In a real implementation, you'd check the device capabilities properly
	return true
}

// Linux input event structure
type inputEvent struct {
	Time  syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
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
