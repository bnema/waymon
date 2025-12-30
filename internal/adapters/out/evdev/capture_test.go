package evdev

import (
	"testing"
	"time"

	"github.com/bnema/waymon/internal/boundaries/out"
	"github.com/bnema/waymon/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCaptureImplementsInterface verifies that Capture implements InputCapturePort.
func TestCaptureImplementsInterface(t *testing.T) {
	var _ out.InputCapturePort = (*Capture)(nil)
}

func TestNew(t *testing.T) {
	c := New()
	require.NotNil(t, c)
	assert.NotNil(t, c.devices)
	assert.NotNil(t, c.ignoredDevices)
	assert.NotNil(t, c.eventChan)
	assert.False(t, c.capturing)
	assert.Equal(t, 30*time.Second, c.grabTimeout)
}

func TestSetGrabTimeout(t *testing.T) {
	c := New()
	c.SetGrabTimeout(60 * time.Second)
	assert.Equal(t, 60*time.Second, c.grabTimeout)
}

func TestSetEmergencyKey(t *testing.T) {
	c := New()
	c.SetEmergencyKey(42)
	assert.Equal(t, uint16(42), c.emergencyKey)
}

func TestSetNoGrab(t *testing.T) {
	c := New()
	c.SetNoGrab(true)
	assert.True(t, c.noGrab)
	c.SetNoGrab(false)
	assert.False(t, c.noGrab)
}

func TestSetEmergencyHandler(t *testing.T) {
	c := New()
	called := false
	handler := func() { called = true }
	c.SetEmergencyHandler(handler)
	require.NotNil(t, c.emergencyHandler)
	c.emergencyHandler()
	assert.True(t, called)
}

func TestSetEventCallback(t *testing.T) {
	c := New()
	var receivedEvent *domain.InputEvent
	callback := func(e *domain.InputEvent) { receivedEvent = e }

	// Need a context for logging - use a background context for test
	c.ctx = t.Context()

	c.SetEventCallback(callback)
	require.NotNil(t, c.onInputEvent)

	// Verify callback is set by calling it
	testEvent := &domain.InputEvent{
		Timestamp: time.Now().UnixNano(),
		SourceID:  "test",
	}
	c.onInputEvent(testEvent)
	assert.Equal(t, testEvent, receivedEvent)
}

func TestStopWhenNotCapturing(t *testing.T) {
	c := New()
	c.ctx = t.Context()

	// Stop should be a no-op when not capturing
	err := c.Stop()
	assert.NoError(t, err)
}

// TestIsValidInputDevice tests the device filtering logic.
// Note: This test doesn't require actual devices.
func TestIsValidInputDeviceExcludePatterns(t *testing.T) {
	// We can't easily test isValidInputDevice without mocking evdev.InputDevice
	// This is a limitation of the current design - the function depends on
	// evdev.InputDevice which requires real /dev/input devices.
	// In a production setting, we might want to extract the pattern matching
	// logic into a separate testable function.
	t.Skip("requires mock evdev device - integration test only")
}

// Note: Full integration tests for Start/Stop/SetTarget require root access
// and actual input devices. These would be placed in tests/integration/.
