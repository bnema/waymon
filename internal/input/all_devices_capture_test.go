package input

import (
	"context"
	"testing"
	"time"

	"github.com/bnema/waymon/internal/protocol"
)

func TestNewAllDevicesCapture(t *testing.T) {
	capture := NewAllDevicesCapture()
	if capture == nil {
		t.Fatal("NewAllDevicesCapture returned nil")
	}

	if capture.devices == nil {
		t.Error("devices map not initialized")
	}

	if capture.ignoredDevices == nil {
		t.Error("ignoredDevices map not initialized")
	}

	if capture.eventChan == nil {
		t.Error("event channel not initialized")
	}

	if capture.capturing {
		t.Error("capturing should be false initially")
	}
}

func TestAllDevicesCaptureSetTarget(t *testing.T) {
	capture := NewAllDevicesCapture()

	// Test setting target
	err := capture.SetTarget("test-client")
	if err != nil {
		t.Errorf("SetTarget failed: %v", err)
	}

	if capture.currentTarget != "test-client" {
		t.Errorf("Expected target 'test-client', got '%s'", capture.currentTarget)
	}

	// Test clearing target
	err = capture.SetTarget("")
	if err != nil {
		t.Errorf("SetTarget clear failed: %v", err)
	}

	if capture.currentTarget != "" {
		t.Errorf("Expected empty target, got '%s'", capture.currentTarget)
	}
}

func TestAllDevicesCaptureOnInputEvent(t *testing.T) {
	capture := NewAllDevicesCapture()

	var receivedEvent *protocol.InputEvent
	callback := func(event *protocol.InputEvent) {
		receivedEvent = event
	}

	capture.OnInputEvent(callback)

	if capture.onInputEvent == nil {
		t.Error("OnInputEvent callback not set")
	}

	// Test callback execution by sending an event to the internal channel
	testEvent := &protocol.InputEvent{
		Event: &protocol.InputEvent_MouseMove{
			MouseMove: &protocol.MouseMoveEvent{
				Dx: 10,
				Dy: 20,
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "test",
	}

	// Set a target so events get processed
	capture.SetTarget("test-client")

	// Start capture to initialize event processing
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		capture.Start(ctx)
	}()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Send event to the channel
	select {
	case capture.eventChan <- testEvent:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Failed to send test event")
	}

	// Wait for event to be processed
	time.Sleep(50 * time.Millisecond)

	capture.Stop()

	if receivedEvent == nil {
		t.Error("Callback was not called")
	}
}

func TestAllDevicesCaptureStartStop(t *testing.T) {
	capture := NewAllDevicesCapture()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test start
	err := capture.Start(ctx)
	if err != nil {
		t.Errorf("Start failed: %v", err)
	}

	if !capture.capturing {
		t.Error("capturing should be true after start")
	}

	// Test stop
	err = capture.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	if capture.capturing {
		t.Error("capturing should be false after stop")
	}
}

func TestAllDevicesCaptureDoubleStart(t *testing.T) {
	capture := NewAllDevicesCapture()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// First start should succeed
	err := capture.Start(ctx)
	if err != nil {
		t.Errorf("First start failed: %v", err)
	}

	// Second start should fail
	err = capture.Start(ctx)
	if err == nil {
		t.Error("Second start should have failed")
	}

	capture.Stop()
}