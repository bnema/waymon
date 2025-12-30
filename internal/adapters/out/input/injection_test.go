package input

import (
	"context"
	"testing"

	"github.com/bnema/waymon/internal/boundaries/out"
	"github.com/stretchr/testify/assert"
)

// TestWaylandInjectorImplementsInterface verifies that WaylandInjector implements InputInjectionPort.
func TestWaylandInjectorImplementsInterface(_ *testing.T) {
	var _ out.InputInjectionPort = (*WaylandInjector)(nil)
}

func TestGetKeyName(t *testing.T) {
	tests := []struct {
		key      uint32
		expected string
	}{
		{30, "A"},
		{48, "B"},
		{46, "C"},
		{1, "ESC"},
		{28, "ENTER"},
		{57, "SPACE"},
		{29, "LEFTCTRL"},
		{42, "LEFTSHIFT"},
		{56, "LEFTALT"},
		{125, "LEFTMETA"},
		{59, "F1"},
		{68, "F10"},
		{2, "1"},
		{11, "0"},
		{999, "KEY_999"}, // Unknown key
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := getKeyName(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetScreenDimensions(t *testing.T) {
	// Create a minimal injector without wayland connection for testing
	w := &WaylandInjector{
		screenWidth:  1920,
		screenHeight: 1080,
	}

	w.SetScreenDimensions(2560, 1440)

	assert.Equal(t, uint32(2560), w.screenWidth)
	assert.Equal(t, uint32(1440), w.screenHeight)
}

func TestStopWhenNotRunning(t *testing.T) {
	// Create a minimal injector without wayland connection
	w := &WaylandInjector{
		running: false,
		ctx:     context.Background(),
	}

	// Stop should be a no-op when not running
	err := w.Stop()
	assert.NoError(t, err)
}

func TestInjectMethodsErrorWhenNotRunning(t *testing.T) {
	ctx := t.Context()

	// Create a minimal injector without starting
	w := &WaylandInjector{
		running: false,
	}

	t.Run("InjectMouseMove", func(t *testing.T) {
		err := w.InjectMouseMove(ctx, 10, 20)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "virtual pointer not available")
	})

	t.Run("InjectMousePosition", func(t *testing.T) {
		err := w.InjectMousePosition(ctx, 100, 200)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "virtual pointer not available")
	})

	t.Run("InjectMouseButton", func(t *testing.T) {
		err := w.InjectMouseButton(ctx, 1, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "virtual pointer not available")
	})

	t.Run("InjectMouseScroll", func(t *testing.T) {
		err := w.InjectMouseScroll(ctx, 0, 1, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "virtual pointer not available")
	})

	t.Run("InjectKeyEvent", func(t *testing.T) {
		err := w.InjectKeyEvent(ctx, 30, true, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "virtual keyboard not available")
	})
}

func TestSetExclusiveCaptureNoChange(t *testing.T) {
	ctx := t.Context()

	w := &WaylandInjector{
		exclusive: false,
		ctx:       ctx,
	}

	// Setting to same value should be no-op
	err := w.SetExclusiveCapture(ctx, false)
	assert.NoError(t, err)
	assert.False(t, w.exclusive)
}

// Note: Full integration tests for New/Start/Stop require a Wayland session.
// These would be placed in tests/integration/.
