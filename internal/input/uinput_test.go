package input

import (
	"os"
	"testing"
	"time"

	"github.com/bnema/waymon/internal/proto"
)

// TestUInputPermissions checks if we have the necessary permissions
func TestUInputPermissions(t *testing.T) {
	// Check if /dev/uinput exists
	if _, err := os.Stat("/dev/uinput"); os.IsNotExist(err) {
		t.Skip("/dev/uinput does not exist - uinput module not loaded")
	}

	// Check if we can open it (requires permissions)
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("Cannot open /dev/uinput: %v (try: sudo chmod 666 /dev/uinput or add user to input group)", err)
	}
	f.Close()
}

// TestUInputHandler_Integration performs actual uinput tests if permissions allow
func TestUInputHandler_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Try to create handler
	handler, err := newUInputHandler()
	if err != nil {
		t.Skipf("Cannot create uinput handler: %v", err)
	}
	defer func() { _ = handler.Close() }()

	t.Run("MouseMove", func(t *testing.T) {
		// Test relative movement
		events := []*proto.MouseEvent{
			{
				Type:        proto.EventType_EVENT_TYPE_MOVE,
				X:           100,
				Y:           100,
				TimestampMs: time.Now().UnixMilli(),
			},
			{
				Type:        proto.EventType_EVENT_TYPE_MOVE,
				X:           150,
				Y:           120,
				TimestampMs: time.Now().UnixMilli(),
			},
		}

		for _, event := range events {
			if err := handler.ProcessEvent(event); err != nil {
				t.Errorf("Failed to process move event: %v", err)
			}
		}

		// Verify internal position tracking
		if handler.currentX != 150 || handler.currentY != 120 {
			t.Errorf("Position not tracked correctly: got (%f, %f), want (150, 120)", 
				handler.currentX, handler.currentY)
		}
	})

	t.Run("MouseClick", func(t *testing.T) {
		// Test click events
		clickTests := []struct {
			button proto.MouseButton
			name   string
		}{
			{proto.MouseButton_MOUSE_BUTTON_LEFT, "left"},
			{proto.MouseButton_MOUSE_BUTTON_RIGHT, "right"},
			{proto.MouseButton_MOUSE_BUTTON_MIDDLE, "middle"},
		}

		for _, tt := range clickTests {
			// Press
			pressEvent := &proto.MouseEvent{
				Type:        proto.EventType_EVENT_TYPE_CLICK,
				X:           200,
				Y:           200,
				Button:      tt.button,
				IsPressed:   true,
				TimestampMs: time.Now().UnixMilli(),
			}
			if err := handler.ProcessEvent(pressEvent); err != nil {
				t.Errorf("Failed to process %s press: %v", tt.name, err)
			}

			// Small delay
			time.Sleep(10 * time.Millisecond)

			// Release
			releaseEvent := &proto.MouseEvent{
				Type:        proto.EventType_EVENT_TYPE_CLICK,
				X:           200,
				Y:           200,
				Button:      tt.button,
				IsPressed:   false,
				TimestampMs: time.Now().UnixMilli(),
			}
			if err := handler.ProcessEvent(releaseEvent); err != nil {
				t.Errorf("Failed to process %s release: %v", tt.name, err)
			}
		}
	})

	t.Run("MouseScroll", func(t *testing.T) {
		scrollTests := []struct {
			direction proto.ScrollDirection
			name      string
		}{
			{proto.ScrollDirection_SCROLL_DIRECTION_UP, "up"},
			{proto.ScrollDirection_SCROLL_DIRECTION_DOWN, "down"},
		}

		for _, tt := range scrollTests {
			event := &proto.MouseEvent{
				Type:        proto.EventType_EVENT_TYPE_SCROLL,
				X:           300,
				Y:           300,
				Direction:   tt.direction,
				TimestampMs: time.Now().UnixMilli(),
			}
			if err := handler.ProcessEvent(event); err != nil {
				t.Errorf("Failed to process scroll %s: %v", tt.name, err)
			}
		}
	})
}

// TestUInputSetup provides instructions for setting up uinput
func TestUInputSetup(t *testing.T) {
	t.Log("To set up uinput for testing:")
	t.Log("1. Load module: sudo modprobe uinput")
	t.Log("2. Set permissions (temporary): sudo chmod 666 /dev/uinput")
	t.Log("3. Or add user to input group (permanent): sudo usermod -a -G input $USER")
	t.Log("4. Create udev rule: echo 'KERNEL==\"uinput\", GROUP=\"input\", MODE=\"0660\"' | sudo tee /etc/udev/rules.d/99-uinput.rules")
	t.Log("5. Reload udev: sudo udevadm control --reload-rules && sudo udevadm trigger")
}