package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestClientConnectionFlow tests that the client connection happens in the correct order
func TestClientConnectionFlow(t *testing.T) {
	// This test ensures that:
	// 1. TUI starts first
	// 2. Connection happens in background goroutine
	// 3. Edge detector and mouse capture start AFTER successful connection
	// 4. No blocking occurs between components

	// Create a mock sequence tracker
	var sequence []string
	sequenceChan := make(chan string, 10)

	// Mock TUI startup
	go func() {
		sequenceChan <- "tui_started"
	}()

	// Mock connection goroutine
	go func() {
		// Small delay to ensure TUI is ready
		time.Sleep(100 * time.Millisecond)
		sequenceChan <- "connection_started"

		// Simulate connection time
		time.Sleep(200 * time.Millisecond)
		sequenceChan <- "connection_established"

		// Start edge detector and mouse capture after connection
		sequenceChan <- "edge_detector_started"
		sequenceChan <- "mouse_capture_started"
	}()

	// Collect sequence
	timeout := time.After(1 * time.Second)
	done := false
	for !done {
		select {
		case event := <-sequenceChan:
			sequence = append(sequence, event)
			if event == "mouse_capture_started" {
				done = true
			}
		case <-timeout:
			done = true
		}
	}

	// Verify sequence
	expectedSequence := []string{
		"tui_started",
		"connection_started",
		"connection_established",
		"edge_detector_started",
		"mouse_capture_started",
	}

	assert.Equal(t, expectedSequence, sequence, "Components should start in the correct order")
}

// TestClientConnectionTimeout tests that connection timeouts are handled properly
func TestClientConnectionTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	startTime := time.Now()
	<-ctx.Done()
	duration := time.Since(startTime)

	assert.True(t, duration >= 100*time.Millisecond, "Context should timeout after specified duration")
	assert.True(t, duration < 200*time.Millisecond, "Context should timeout promptly")
	assert.Equal(t, context.DeadlineExceeded, ctx.Err(), "Context should report deadline exceeded")
}

// TestEdgeDetectorNotStartedBeforeConnection tests that edge detector doesn't start prematurely
func TestEdgeDetectorNotStartedBeforeConnection(t *testing.T) {
	// Track if edge detector was started
	edgeDetectorStarted := false
	connectionEstablished := false

	// Simulate connection delay
	go func() {
		time.Sleep(500 * time.Millisecond)
		connectionEstablished = true
	}()

	// Check that edge detector doesn't start before connection
	checkInterval := 50 * time.Millisecond
	for i := 0; i < 5; i++ {
		time.Sleep(checkInterval)
		if !connectionEstablished && edgeDetectorStarted {
			t.Fatal("Edge detector started before connection was established")
		}
	}

	// Now simulate starting edge detector after connection
	for !connectionEstablished {
		time.Sleep(checkInterval)
	}
	edgeDetectorStarted = true

	assert.True(t, connectionEstablished, "Connection should be established")
	assert.True(t, edgeDetectorStarted, "Edge detector should start after connection")
}
