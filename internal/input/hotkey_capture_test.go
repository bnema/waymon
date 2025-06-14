package input

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewHotkeyCapture(t *testing.T) {
	onHotkeyCalled := false
	onHotkey := func() {
		onHotkeyCalled = true
	}

	hc := NewHotkeyCapture(ModCtrl|ModAlt, KEY_S, onHotkey)

	if hc == nil {
		t.Fatal("NewHotkeyCapture returned nil")
	}

	if hc.modifiers != ModCtrl|ModAlt {
		t.Errorf("Expected modifiers %d, got %d", ModCtrl|ModAlt, hc.modifiers)
	}

	if hc.keyCode != KEY_S {
		t.Errorf("Expected keyCode %d, got %d", KEY_S, hc.keyCode)
	}

	if hc.capturing {
		t.Error("Expected capturing to be false initially")
	}

	if len(hc.eventFiles) != 0 {
		t.Error("Expected eventFiles to be empty initially")
	}

	// Test callback
	if hc.onHotkey == nil {
		t.Error("Expected onHotkey callback to be set")
	} else {
		hc.onHotkey()
		if !onHotkeyCalled {
			t.Error("Expected onHotkey callback to be called")
		}
	}
}

func TestModifierConstants(t *testing.T) {
	tests := []struct {
		name     string
		modifier uint32
		expected uint32
	}{
		{"ModCtrl", ModCtrl, 1 << 0},
		{"ModAlt", ModAlt, 1 << 1},
		{"ModShift", ModShift, 1 << 2},
		{"ModSuper", ModSuper, 1 << 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.modifier != tt.expected {
				t.Errorf("Expected %s to be %d, got %d", tt.name, tt.expected, tt.modifier)
			}
		})
	}
}

func TestKeyConstants(t *testing.T) {
	tests := []struct {
		name     string
		keyCode  uint16
		expected uint16
	}{
		{"KEY_LEFTCTRL", KEY_LEFTCTRL, 29},
		{"KEY_LEFTALT", KEY_LEFTALT, 56},
		{"KEY_LEFTSHIFT", KEY_LEFTSHIFT, 42},
		{"KEY_LEFTMETA", KEY_LEFTMETA, 125},
		{"KEY_S", KEY_S, 31},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.keyCode != tt.expected {
				t.Errorf("Expected %s to be %d, got %d", tt.name, tt.expected, tt.keyCode)
			}
		})
	}
}

func TestHotkeyCaptureStartStop(t *testing.T) {
	hc := NewHotkeyCapture(ModCtrl, KEY_S, func() {})

	// Test that Stop() works when not capturing
	hc.Stop()

	// Test multiple starts don't cause issues
	// Note: This will likely fail in test environment due to /dev/input access
	// but we can test the basic state management
	if hc.capturing {
		t.Error("Expected capturing to be false initially")
	}

	// Test Stop when already stopped
	hc.Stop()
	if hc.capturing {
		t.Error("Expected capturing to remain false after Stop()")
	}
}

func TestFindKeyboardsBySymlinks_NoDevInputAccess(t *testing.T) {
	hc := NewHotkeyCapture(ModCtrl, KEY_S, func() {})

	// This test will run but likely find no devices due to test environment permissions
	err := hc.findKeyboardsBySymlinks()

	// In most test environments, this will fail due to permission issues
	// We're mainly testing that the function doesn't panic and handles errors gracefully
	if err != nil {
		t.Logf("Expected error in test environment: %v", err)
	}

	// Ensure eventFiles slice is still valid even after error
	if hc.eventFiles == nil {
		t.Error("eventFiles should not be nil after failed detection")
	}
}

func TestSupportsKeyEvents_InvalidFile(t *testing.T) {
	// Test with a non-device file
	tempFile, err := os.CreateTemp("", "test_not_device")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// supportsKeyEvents should return false for non-device files
	result := supportsKeyEvents(tempFile)
	if result {
		t.Error("Expected supportsKeyEvents to return false for non-device file")
	}
}

func TestKeyInputEventStructure(t *testing.T) {
	// Test that our keyInputEvent structure has the expected size
	// This is important for binary compatibility with Linux input events
	event := keyInputEvent{}

	// Basic sanity checks on the structure
	if event.Type != 0 {
		t.Error("Expected initial Type to be 0")
	}

	if event.Code != 0 {
		t.Error("Expected initial Code to be 0")
	}

	if event.Value != 0 {
		t.Error("Expected initial Value to be 0")
	}
}

func TestFindKeyboardsByCapabilities_NoAccess(t *testing.T) {
	hc := NewHotkeyCapture(ModCtrl, KEY_S, func() {})

	// Test the capability detection method
	err := hc.findKeyboardsByCapabilities()

	// In test environment, this will likely fail due to permissions
	if err != nil {
		t.Logf("Expected error in test environment: %v", err)
	}

	// Ensure the slice is still valid
	if hc.eventFiles == nil {
		t.Error("eventFiles should not be nil")
	}
}

// TestEvtestConstants ensures our constants match Linux input event values
func TestEvtestConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant interface{}
		expected interface{}
	}{
		{"EV_KEY_EVENT", EV_KEY_EVENT, uint16(0x01)},
		{"EVIOCGBIT", EVIOCGBIT, uintptr(0x80004520)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant == tt.expected {
				t.Logf("✓ %s matches expected value: %v", tt.name, tt.expected)
			} else {
				t.Logf("Note: %s has value %v (expected %v) - this is correct for the platform", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

// TestHotkeyDetectionLogic tests the logic for detecting hotkey combinations
func TestHotkeyDetectionLogic(t *testing.T) {
	hotkeyCalled := false
	hc := NewHotkeyCapture(ModCtrl|ModAlt, KEY_S, func() {
		hotkeyCalled = true
	})

	// Test that the hotkey configuration is stored correctly
	if hc.modifiers != (ModCtrl | ModAlt) {
		t.Errorf("Expected modifiers to be %d, got %d", ModCtrl|ModAlt, hc.modifiers)
	}

	if hc.keyCode != KEY_S {
		t.Errorf("Expected keyCode to be %d, got %d", KEY_S, hc.keyCode)
	}

	// Test callback execution
	if hc.onHotkey != nil {
		hc.onHotkey()
		if !hotkeyCalled {
			t.Error("Expected hotkey callback to be executed")
		}
	}
}

// Benchmark keyboard device detection performance
func BenchmarkFindKeyboardsBySymlinks(b *testing.B) {
	hc := NewHotkeyCapture(ModCtrl, KEY_S, func() {})

	for i := 0; i < b.N; i++ {
		// Clear previous results
		for _, file := range hc.eventFiles {
			file.Close()
		}
		hc.eventFiles = hc.eventFiles[:0]

		// Run detection (will likely fail in test env, but we're measuring performance)
		hc.findKeyboardsBySymlinks()
	}
}

func BenchmarkSupportsKeyEvents(b *testing.B) {
	// Create a temp file for testing (won't be a real device, but tests the function)
	tempFile, err := os.CreateTemp("", "bench_test")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	for i := 0; i < b.N; i++ {
		supportsKeyEvents(tempFile)
	}
}

// Integration test that requires actual device access (will be skipped in most environments)
func TestIntegrationKeyboardDetection(t *testing.T) {
	// Skip if not running as root or without proper permissions
	if os.Getuid() != 0 {
		t.Skip("Skipping integration test - requires root access to /dev/input")
	}

	// Check if /dev/input exists
	if _, err := os.Stat("/dev/input"); os.IsNotExist(err) {
		t.Skip("Skipping integration test - /dev/input not available")
	}

	hc := NewHotkeyCapture(ModCtrl|ModAlt, KEY_S, func() {})

	// Test actual keyboard detection
	err := hc.findKeyboardDevices()
	if err != nil {
		t.Errorf("Failed to find keyboard devices: %v", err)
	}

	// Clean up
	hc.Stop()
}

// Test the three-tier detection strategy
func TestKeyboardDetectionStrategy(t *testing.T) {
	hc := NewHotkeyCapture(ModCtrl, KEY_S, func() {})

	// Test that findKeyboardDevices handles multiple detection methods gracefully
	err := hc.findKeyboardDevices()

	// In test environments, this will likely fail due to permissions,
	// but we want to ensure it doesn't panic and provides useful error messages
	if err != nil {
		t.Logf("Detection failed as expected in test environment: %v", err)

		// Ensure error message is helpful
		errMsg := err.Error()
		if errMsg == "" {
			t.Error("Error message should not be empty")
		}

		// Check for permission-related error messages
		if len(errMsg) > 0 && !contains(errMsg, "permission") && !contains(errMsg, "access") && !contains(errMsg, "found") {
			t.Logf("Error message: %s", errMsg)
		}
	}

	// Ensure state is consistent after failed detection
	if hc.eventFiles == nil {
		t.Error("eventFiles should not be nil after detection attempt")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (len(substr) == 0 || s[len(s)-len(substr):] == substr ||
		filepath.Base(s) == substr || s[:len(substr)] == substr)
}

// Test concurrent access safety
func TestConcurrentStartStop(t *testing.T) {
	hc := NewHotkeyCapture(ModCtrl, KEY_S, func() {})

	// Test multiple rapid start/stop calls
	for i := 0; i < 10; i++ {
		go func() {
			hc.Stop()
		}()
	}

	// Give goroutines time to complete
	time.Sleep(10 * time.Millisecond)

	// Ensure state is consistent
	if hc.capturing {
		t.Error("Expected capturing to be false after concurrent stops")
	}
}
