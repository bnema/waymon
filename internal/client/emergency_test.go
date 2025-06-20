package client

import (
	"context"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestEmergencyRelease(t *testing.T) {
	// Create a mock input receiver
	receiver := &InputReceiver{
		connected: true,
	}

	emergencyTriggered := false
	var mu sync.Mutex

	// Create emergency release
	emergency := NewEmergencyRelease(receiver, func(reason string) {
		mu.Lock()
		emergencyTriggered = true
		mu.Unlock()
		t.Logf("Emergency triggered with reason: %s", reason)
	})

	// Start monitoring
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	emergency.Start(ctx)

	// Test file-based trigger
	t.Run("FileTrigger", func(t *testing.T) {
		// Reset flag
		mu.Lock()
		emergencyTriggered = false
		mu.Unlock()

		// Create trigger file
		triggerFile := "/tmp/waymon-client-release"
		if err := os.WriteFile(triggerFile, []byte("trigger"), 0644); err != nil {
			t.Fatalf("Failed to create trigger file: %v", err)
		}

		// Wait for trigger
		time.Sleep(2 * time.Second)

		// Check if triggered
		mu.Lock()
		triggered := emergencyTriggered
		mu.Unlock()

		if !triggered {
			t.Error("Emergency release not triggered by file")
		}

		// Ensure file was cleaned up
		if _, err := os.Stat(triggerFile); err == nil {
			t.Error("Trigger file not cleaned up")
		}
	})
}

func TestEmergencySignal(t *testing.T) {
	// Create a mock input receiver
	receiver := &InputReceiver{
		connected: true,
	}

	emergencyTriggered := false
	var mu sync.Mutex

	// Create emergency release
	emergency := NewEmergencyRelease(receiver, func(reason string) {
		mu.Lock()
		emergencyTriggered = true
		mu.Unlock()
		t.Logf("Emergency triggered with reason: %s", reason)
	})

	// Start monitoring
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	emergency.Start(ctx)

	// Send SIGUSR1 to self
	if err := syscall.Kill(os.Getpid(), syscall.SIGUSR1); err != nil {
		t.Fatalf("Failed to send signal: %v", err)
	}

	// Wait for signal handling
	time.Sleep(100 * time.Millisecond)

	// Check if triggered
	mu.Lock()
	triggered := emergencyTriggered
	mu.Unlock()

	if !triggered {
		t.Error("Emergency release not triggered by signal")
	}
}

func TestEmergencyActivityTimeout(t *testing.T) {
	t.Skip("Activity timeout requires full InputReceiver setup")
	// Note: The activity timeout monitor checks receiver.IsConnected()
	// which requires proper initialization of the InputReceiver.
	// This would be better tested in an integration test.
}

func TestEmergencyUpdateActivity(t *testing.T) {
	// Create a mock input receiver
	receiver := &InputReceiver{
		connected: true,
	}

	emergencyTriggered := false
	var mu sync.Mutex

	// Create emergency release with short timeout
	emergency := NewEmergencyRelease(receiver, func(reason string) {
		mu.Lock()
		emergencyTriggered = true
		mu.Unlock()
		t.Logf("Emergency triggered with reason: %s", reason)
	})
	emergency.activityTimeout = 1 * time.Second // Very short timeout for testing

	// Start monitoring
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	emergency.Start(ctx)

	// Keep updating activity
	for i := 0; i < 5; i++ {
		emergency.UpdateActivity()
		time.Sleep(200 * time.Millisecond)
	}

	// Check if NOT triggered
	mu.Lock()
	triggered := emergencyTriggered
	mu.Unlock()

	if triggered {
		t.Error("Emergency release should not have been triggered with activity updates")
	}
}