package client

import (
	"sync"
	"testing"
	"time"

	"github.com/bnema/waymon/internal/protocol"
)

func TestHandleControlEventCallback(t *testing.T) {
	// Create input receiver
	ir, err := NewInputReceiver("test-server:52525")
	if err != nil {
		t.Fatalf("Failed to create input receiver: %v", err)
	}

	// Set up callback tracking
	var callbackCalled bool
	var callbackStatus ControlStatus
	var mu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(1)
	ir.OnStatusChange(func(status ControlStatus) {
		mu.Lock()
		defer mu.Unlock()
		callbackCalled = true
		callbackStatus = status
		wg.Done()
	})

	// Test REQUEST_CONTROL event
	controlEvent := &protocol.ControlEvent{
		Type:     protocol.ControlEvent_REQUEST_CONTROL,
		TargetId: "test-server",
	}
	ir.handleControlEvent(controlEvent)

	// Wait for callback with timeout
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Callback completed
	case <-time.After(1 * time.Second):
		t.Fatal("Callback was not called within timeout")
	}

	// Verify callback was called with correct status
	mu.Lock()
	if !callbackCalled {
		t.Error("Status change callback was not called")
	}
	if !callbackStatus.BeingControlled {
		t.Error("Expected BeingControlled to be true")
	}
	if callbackStatus.ControllerName != "test-server" {
		t.Errorf("Expected ControllerName to be 'test-server', got '%s'", callbackStatus.ControllerName)
	}
	mu.Unlock()

	// Reset for RELEASE_CONTROL test
	callbackCalled = false
	wg.Add(1)

	// Test RELEASE_CONTROL event
	releaseEvent := &protocol.ControlEvent{
		Type:     protocol.ControlEvent_RELEASE_CONTROL,
		TargetId: ir.clientID,
	}
	ir.handleControlEvent(releaseEvent)

	// Wait for callback with timeout
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Callback completed
	case <-time.After(1 * time.Second):
		t.Fatal("Release callback was not called within timeout")
	}

	// Verify callback was called with correct status for release
	mu.Lock()
	if !callbackCalled {
		t.Error("Status change callback was not called for release")
	}
	if callbackStatus.BeingControlled {
		t.Error("Expected BeingControlled to be false after release")
	}
	if callbackStatus.ControllerName != "" {
		t.Errorf("Expected ControllerName to be empty after release, got '%s'", callbackStatus.ControllerName)
	}
	mu.Unlock()
}

func TestControlStatusCopyInCallback(t *testing.T) {
	// Create input receiver
	ir, err := NewInputReceiver("test-server:52525")
	if err != nil {
		t.Fatalf("Failed to create input receiver: %v", err)
	}

	// Track all status changes
	var statusChanges []ControlStatus
	var mu sync.Mutex
	var wg sync.WaitGroup

	ir.OnStatusChange(func(status ControlStatus) {
		mu.Lock()
		defer mu.Unlock()
		// Store a copy to ensure we're not getting a reference
		statusChanges = append(statusChanges, status)
		wg.Done()
	})

	// Send multiple control events with synchronization
	events := []struct {
		eventType protocol.ControlEvent_Type
		targetID  string
		expected  bool
	}{
		{protocol.ControlEvent_REQUEST_CONTROL, "server1", true},
		{protocol.ControlEvent_RELEASE_CONTROL, "", false},
		{protocol.ControlEvent_REQUEST_CONTROL, "server2", true},
		{protocol.ControlEvent_RELEASE_CONTROL, "", false},
	}

	// Add wait group counters for all events
	wg.Add(len(events))

	for _, evt := range events {
		controlEvent := &protocol.ControlEvent{
			Type:     evt.eventType,
			TargetId: evt.targetID,
		}
		ir.handleControlEvent(controlEvent)
		// Give a tiny delay between events to ensure ordering
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for all callbacks to complete
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// All callbacks completed
	case <-time.After(2 * time.Second):
		t.Fatal("Not all callbacks were called within timeout")
	}

	// Verify we got all status changes
	mu.Lock()
	defer mu.Unlock()

	if len(statusChanges) != len(events) {
		t.Errorf("Expected %d status changes, got %d", len(events), len(statusChanges))
	}

	// Verify each status change is correct
	for i, status := range statusChanges {
		if i < len(events) {
			if status.BeingControlled != events[i].expected {
				t.Errorf("Status %d: expected BeingControlled=%v, got %v", i, events[i].expected, status.BeingControlled)
			}
			if events[i].expected && status.ControllerName != events[i].targetID {
				t.Errorf("Status %d: expected ControllerName='%s', got '%s'", i, events[i].targetID, status.ControllerName)
			}
			if !events[i].expected && status.ControllerName != "" {
				t.Errorf("Status %d: expected empty ControllerName, got '%s'", i, status.ControllerName)
			}
		}
	}
}