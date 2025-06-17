package network

import (
	"io"
	"sync"
	"testing"
	"time"

	"github.com/bnema/waymon/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockWriter simulates a network writer with configurable latency
type MockWriter struct {
	mu           sync.Mutex
	writtenData  [][]byte
	writeLatency time.Duration
}

func (m *MockWriter) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.writeLatency > 0 {
		time.Sleep(m.writeLatency)
	}
	
	// Copy the data to avoid slice reuse issues
	data := make([]byte, len(p))
	copy(data, p)
	m.writtenData = append(m.writtenData, data)
	
	return len(p), nil
}

func (m *MockWriter) GetWrittenCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.writtenData)
}

// TestSSHClientEventSending tests the performance of sending events over SSH
func TestSSHClientEventSending(t *testing.T) {
	// Create a mock writer to simulate SSH connection
	mockWriter := &MockWriter{}
	
	// Test sending mouse movement events
	t.Run("MouseMovementThroughput", func(t *testing.T) {
		eventCount := 1000
		mockWriter.writtenData = nil
		
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
			
			err := writeInputMessage(mockWriter, event)
			require.NoError(t, err)
		}
		
		duration := time.Since(startTime)
		eventsPerSecond := float64(eventCount) / duration.Seconds()
		
		t.Logf("Sent %d events in %v (%.0f events/sec)", 
			eventCount, duration, eventsPerSecond)
		
		// Should handle at least 10,000 events per second
		assert.Greater(t, eventsPerSecond, 10000.0, 
			"Event sending rate too low")
		
		// Verify all events were written
		assert.Equal(t, eventCount, mockWriter.GetWrittenCount())
	})
	
	// Test with simulated network latency
	t.Run("WithNetworkLatency", func(t *testing.T) {
		mockWriter.writtenData = nil
		mockWriter.writeLatency = 1 * time.Millisecond // Simulate 1ms network latency
		
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
			
			err := writeInputMessage(mockWriter, event)
			require.NoError(t, err)
		}
		
		duration := time.Since(startTime)
		eventsPerSecond := float64(eventCount) / duration.Seconds()
		
		t.Logf("With 1ms latency: %d events in %v (%.0f events/sec)", 
			eventCount, duration, eventsPerSecond)
		
		// Even with latency, should handle reasonable rate
		assert.Greater(t, eventsPerSecond, 500.0, 
			"Event sending rate too low with latency")
	})
}

// TestConcurrentEventSending tests sending events from multiple goroutines
func TestConcurrentEventSending(t *testing.T) {
	mockWriter := &MockWriter{}
	
	// Use a mutex to make writes thread-safe
	var writeMu sync.Mutex
	safeWrite := func(event *protocol.InputEvent) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return writeInputMessage(mockWriter, event)
	}
	
	goroutines := 10
	eventsPerGoroutine := 100
	totalEvents := goroutines * eventsPerGoroutine
	
	var wg sync.WaitGroup
	startTime := time.Now()
	
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for i := 0; i < eventsPerGoroutine; i++ {
				event := &protocol.InputEvent{
					Event: &protocol.InputEvent_MouseMove{
						MouseMove: &protocol.MouseMoveEvent{
							Dx: float64(i),
							Dy: float64(goroutineID),
						},
					},
					Timestamp: time.Now().UnixNano(),
					SourceId:  "test",
				}
				
				err := safeWrite(event)
				assert.NoError(t, err)
			}
		}(g)
	}
	
	wg.Wait()
	duration := time.Since(startTime)
	
	eventsPerSecond := float64(totalEvents) / duration.Seconds()
	t.Logf("Concurrent: %d events from %d goroutines in %v (%.0f events/sec)", 
		totalEvents, goroutines, duration, eventsPerSecond)
	
	// Verify all events were written
	assert.Equal(t, totalEvents, mockWriter.GetWrittenCount())
}

// BenchmarkWriteInputMessage benchmarks the protocol buffer serialization
func BenchmarkWriteInputMessage(b *testing.B) {
	writer := io.Discard // Write to nowhere for pure serialization benchmark
	
	event := &protocol.InputEvent{
		Event: &protocol.InputEvent_MouseMove{
			MouseMove: &protocol.MouseMoveEvent{
				Dx: 123.45,
				Dy: 678.90,
			},
		},
		Timestamp: time.Now().UnixNano(),
		SourceId:  "benchmark",
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		err := writeInputMessage(writer, event)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestEventBatching tests if we should implement event batching
func TestEventBatching(t *testing.T) {
	// This test explores whether batching multiple events into one message
	// would improve performance
	
	t.Run("SingleEventSize", func(t *testing.T) {
		mockWriter := &MockWriter{}
		
		event := &protocol.InputEvent{
			Event: &protocol.InputEvent_MouseMove{
				MouseMove: &protocol.MouseMoveEvent{
					Dx: 100.0,
					Dy: 200.0,
				},
			},
			Timestamp: time.Now().UnixNano(),
			SourceId:  "test",
		}
		
		err := writeInputMessage(mockWriter, event)
		require.NoError(t, err)
		
		// Check the size of a single event message
		totalSize := 0
		for _, data := range mockWriter.writtenData {
			totalSize += len(data)
		}
		
		t.Logf("Single mouse move event size: %d bytes", totalSize)
		
		// Mouse events should be relatively small
		assert.Less(t, totalSize, 100, "Event size larger than expected")
	})
	
	t.Run("BurstEventTiming", func(t *testing.T) {
		mockWriter := &MockWriter{}
		
		// Simulate a burst of 10 rapid mouse movements
		startTime := time.Now()
		
		for i := 0; i < 10; i++ {
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
			
			err := writeInputMessage(mockWriter, event)
			require.NoError(t, err)
		}
		
		burstDuration := time.Since(startTime)
		t.Logf("10-event burst took %v", burstDuration)
		
		// Burst should complete very quickly
		assert.Less(t, burstDuration, 1*time.Millisecond)
	})
}