package input

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bnema/waymon/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMouseMovementLatency tests that mouse movement events are sent immediately
func TestMouseMovementLatency(t *testing.T) {
	// Create a test capture system
	capture := NewAllDevicesCapture()
	
	// Start capture
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := capture.Start(ctx)
	require.NoError(t, err)
	defer capture.Stop()
	
	// Set a target to enable event forwarding
	err = capture.SetTarget("test-client")
	require.NoError(t, err)
	
	// Test 1: Verify events are sent without buffering
	t.Run("NoBuffering", func(t *testing.T) {
		receivedChan := make(chan *protocol.InputEvent, 1)
		
		// Set up event handler for this test
		capture.OnInputEvent(func(event *protocol.InputEvent) {
			select {
			case receivedChan <- event:
			default:
			}
		})
		
		event := &protocol.InputEvent{
			Event: &protocol.InputEvent_MouseMove{
				MouseMove: &protocol.MouseMoveEvent{
					Dx: 10.0,
					Dy: 5.0,
				},
			},
			Timestamp: time.Now().UnixNano(),
			SourceId:  "test",
		}
		
		startTime := time.Now()
		capture.sendEvent(event)
		
		// Wait for event to be received
		select {
		case <-receivedChan:
			latency := time.Since(startTime)
			// Events should be processed within 20ms (one tick interval)
			assert.Less(t, latency, 20*time.Millisecond, 
				"Event processing took too long: %v", latency)
		case <-time.After(50 * time.Millisecond):
			t.Fatal("Event was not received within timeout")
		}
	})
	
	// Test 2: High-frequency mouse movement
	t.Run("HighFrequencyMovement", func(t *testing.T) {
		var receivedCount int32
		
		// Set up event handler
		capture.OnInputEvent(func(event *protocol.InputEvent) {
			atomic.AddInt32(&receivedCount, 1)
		})
		
		// Send 100 mouse movements rapidly
		eventCount := 100
		startTime := time.Now()
		
		for i := 0; i < eventCount; i++ {
			event := &protocol.InputEvent{
				Event: &protocol.InputEvent_MouseMove{
					MouseMove: &protocol.MouseMoveEvent{
						Dx: float64(i),
						Dy: float64(i),
					},
				},
				Timestamp: time.Now().UnixNano(),
				SourceId:  "test",
			}
			capture.sendEvent(event)
		}
		
		// Wait a bit for events to be processed
		time.Sleep(100 * time.Millisecond)
		
		actualCount := atomic.LoadInt32(&receivedCount)
		duration := time.Since(startTime)
		
		// All events should be received
		assert.GreaterOrEqual(t, actualCount, int32(eventCount-10), 
			"Too many events were lost: received %d/%d", actualCount, eventCount)
		
		// Calculate event rate
		eventsPerSecond := float64(actualCount) / duration.Seconds()
		t.Logf("Processed %d events in %v (%.0f events/sec)", 
			actualCount, duration, eventsPerSecond)
		
		// Should handle at least 500 events per second
		assert.Greater(t, eventsPerSecond, 500.0, 
			"Event processing rate too low")
	})
	
	// Test 3: Event channel capacity
	t.Run("ChannelCapacity", func(t *testing.T) {
		// Send more events than channel capacity (1000)
		eventCount := 1500
		droppedEvents := 0
		
		for i := 0; i < eventCount; i++ {
			event := &protocol.InputEvent{
				Event: &protocol.InputEvent_MouseMove{
					MouseMove: &protocol.MouseMoveEvent{
						Dx: float64(i),
						Dy: float64(i),
					},
				},
				Timestamp: time.Now().UnixNano(),
				SourceId:  "test",
			}
			
			// Try to send without blocking
			select {
			case capture.eventChan <- event:
				// Event sent successfully
			default:
				// Channel full, event dropped
				droppedEvents++
			}
		}
		
		// Some events may be dropped when channel is full
		t.Logf("Dropped %d events out of %d when channel was full", 
			droppedEvents, eventCount)
		
		// With 1000 buffer, should drop fewer events
		assert.Less(t, droppedEvents, eventCount/3, 
			"Too many events dropped - channel capacity may be too small")
	})
}

// TestMouseButtonLatency tests that mouse button events are sent immediately
func TestMouseButtonLatency(t *testing.T) {
	capture := NewAllDevicesCapture()
	
	eventReceived := make(chan *protocol.InputEvent, 1)
	
	capture.OnInputEvent(func(event *protocol.InputEvent) {
		select {
		case eventReceived <- event:
		default:
		}
	})
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := capture.Start(ctx)
	require.NoError(t, err)
	defer capture.Stop()
	
	err = capture.SetTarget("test-client")
	require.NoError(t, err)
	
	// Send mouse button event
	buttonEvent := &protocol.InputEvent{
		Event: &protocol.InputEvent_MouseButton{
			MouseButton: &protocol.MouseButtonEvent{
				Button:  1,
				Pressed: true,
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "test",
	}
	
	startTime := time.Now()
	capture.sendEvent(buttonEvent)
	
	select {
	case receivedEvent := <-eventReceived:
		latency := time.Since(startTime)
		assert.Less(t, latency, 5*time.Millisecond, 
			"Button event latency too high: %v", latency)
		
		assert.NotNil(t, receivedEvent)
		assert.NotNil(t, receivedEvent.GetMouseButton())
	case <-time.After(10 * time.Millisecond):
		t.Fatal("Button event not received within timeout")
	}
}

// TestEventAggregatorPerformance tests the event aggregator performance
func TestEventAggregatorPerformance(t *testing.T) {
	aggregator := NewEventAggregator()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := aggregator.Start(ctx)
	require.NoError(t, err)
	defer aggregator.Stop()
	
	// Track output events
	var outputEvents []*protocol.InputEvent
	var mu sync.Mutex
	
	go func() {
		for event := range aggregator.OutputChannel() {
			mu.Lock()
			outputEvents = append(outputEvents, event)
			mu.Unlock()
		}
	}()
	
	// Test mouse movement aggregation
	t.Run("MouseMovementAggregation", func(t *testing.T) {
		// Send multiple small movements within one tick period
		inputChan := aggregator.InputChannel()
		
		for i := 0; i < 5; i++ {
			event := &protocol.InputEvent{
				Event: &protocol.InputEvent_MouseMove{
					MouseMove: &protocol.MouseMoveEvent{
						Dx: 2.0,
						Dy: 1.0,
					},
				},
				Timestamp: time.Now().UnixNano(),
				SourceId:  "test",
			}
			inputChan <- event
		}
		
		// Wait for aggregation tick
		time.Sleep(20 * time.Millisecond)
		
		mu.Lock()
		defer mu.Unlock()
		
		// Should have aggregated movements into fewer events
		// (Ideally just one aggregated event)
		assert.Greater(t, len(outputEvents), 0, "No output events received")
		
		// Check if movements were aggregated
		totalDx := 0.0
		totalDy := 0.0
		for _, event := range outputEvents {
			if move := event.GetMouseMove(); move != nil {
				totalDx += move.Dx
				totalDy += move.Dy
			}
		}
		
		// Total movement should be preserved
		assert.InDelta(t, 10.0, totalDx, 0.1, "X movement not preserved")
		assert.InDelta(t, 5.0, totalDy, 0.1, "Y movement not preserved")
	})
	
	// Test non-movement events pass through immediately
	t.Run("NonMovementEventsImmediate", func(t *testing.T) {
		mu.Lock()
		outputEvents = outputEvents[:0] // Clear
		mu.Unlock()
		
		// Send button event
		buttonEvent := &protocol.InputEvent{
			Event: &protocol.InputEvent_MouseButton{
				MouseButton: &protocol.MouseButtonEvent{
					Button:  1,
					Pressed: true,
				},
			},
			Timestamp: time.Now().UnixNano(),
			SourceId:  "test",
		}
		
		startTime := time.Now()
		aggregator.InputChannel() <- buttonEvent
		
		// Non-movement events should pass through immediately
		time.Sleep(5 * time.Millisecond)
		
		mu.Lock()
		found := false
		for _, event := range outputEvents {
			if event.GetMouseButton() != nil {
				found = true
				latency := time.Since(startTime)
				assert.Less(t, latency, 10*time.Millisecond, 
					"Button event not passed through immediately")
				break
			}
		}
		mu.Unlock()
		
		assert.True(t, found, "Button event not found in output")
	})
}

// BenchmarkEventProcessing benchmarks the event processing pipeline
func BenchmarkEventProcessing(b *testing.B) {
	capture := NewAllDevicesCapture()
	
	var eventCount int32
	capture.OnInputEvent(func(event *protocol.InputEvent) {
		atomic.AddInt32(&eventCount, 1)
	})
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := capture.Start(ctx)
	require.NoError(b, err)
	defer capture.Stop()
	
	err = capture.SetTarget("test-client")
	require.NoError(b, err)
	
	b.ResetTimer()
	
	// Benchmark sending events
	for i := 0; i < b.N; i++ {
		event := &protocol.InputEvent{
			Event: &protocol.InputEvent_MouseMove{
				MouseMove: &protocol.MouseMoveEvent{
					Dx: float64(i % 100),
					Dy: float64(i % 100),
				},
			},
			Timestamp: time.Now().UnixNano(),
			SourceId:  "benchmark",
		}
		capture.sendEvent(event)
	}
	
	// Give some time for events to be processed
	time.Sleep(100 * time.Millisecond)
	
	b.Logf("Processed %d events", atomic.LoadInt32(&eventCount))
}