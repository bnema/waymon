package network

import (
	"io"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/protocol"
)

// BufferedWriter implements a write buffer with automatic flushing
type BufferedWriter struct {
	w         io.Writer
	buf       []byte
	mu        sync.Mutex
	flushChan chan struct{}
	done      chan struct{}
	maxDelay  time.Duration
	maxSize   int
	immediate bool // Whether to flush immediately (for low-latency events)
}

// NewBufferedWriter creates a new buffered writer that automatically flushes
func NewBufferedWriter(w io.Writer, maxDelay time.Duration, maxSize int) *BufferedWriter {
	bw := &BufferedWriter{
		w:         w,
		buf:       make([]byte, 0, maxSize),
		flushChan: make(chan struct{}, 1),
		done:      make(chan struct{}),
		maxDelay:  maxDelay,
		maxSize:   maxSize,
		immediate: false,
	}

	go bw.flushLoop()
	return bw
}

// NewImmediateWriter creates a buffered writer that flushes immediately on every write
func NewImmediateWriter(w io.Writer) *BufferedWriter {
	return &BufferedWriter{
		w:         w,
		buf:       make([]byte, 0, 1024), // Small buffer for immediate mode
		flushChan: make(chan struct{}, 1),
		done:      make(chan struct{}),
		maxDelay:  0,
		maxSize:   1024,
		immediate: true,
	}
}

// Write implements io.Writer
func (bw *BufferedWriter) Write(p []byte) (int, error) {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	// If in immediate mode, write directly and flush
	if bw.immediate {
		bw.buf = append(bw.buf[:0], p...) // Reset buffer and add new data
		return len(p), bw.flushLocked()
	}

	// If adding this data would exceed maxSize, flush first
	if len(bw.buf)+len(p) > bw.maxSize {
		if err := bw.flushLocked(); err != nil {
			return 0, err
		}
	}

	// Add to buffer
	bw.buf = append(bw.buf, p...)

	// Schedule flush if this is the first data
	if len(bw.buf) == len(p) {
		select {
		case bw.flushChan <- struct{}{}:
		default:
		}
	}

	return len(p), nil
}

// Flush forces an immediate flush of the buffer
func (bw *BufferedWriter) Flush() error {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	return bw.flushLocked()
}

// flushLocked flushes the buffer (caller must hold mutex)
func (bw *BufferedWriter) flushLocked() error {
	if len(bw.buf) == 0 {
		return nil
	}

	_, err := bw.w.Write(bw.buf)
	bw.buf = bw.buf[:0] // Reset buffer

	// If the underlying writer supports flushing, do it
	if flusher, ok := bw.w.(interface{ Flush() error }); ok {
		flusher.Flush()
	}

	return err
}

// flushLoop runs in a goroutine to handle periodic flushing
func (bw *BufferedWriter) flushLoop() {
	timer := time.NewTimer(bw.maxDelay)
	timer.Stop()

	for {
		select {
		case <-bw.done:
			return
		case <-bw.flushChan:
			// Reset timer for delayed flush
			timer.Reset(bw.maxDelay)
		case <-timer.C:
			// Time to flush
			bw.Flush()
		}
	}
}

// Close flushes any remaining data and closes the writer
func (bw *BufferedWriter) Close() error {
	close(bw.done)
	return bw.Flush()
}

// SmartWriter wraps a writer and selectively uses immediate or buffered mode
type SmartWriter struct {
	writer *BufferedWriter
	mu     sync.Mutex
}

// NewSmartWriter creates a writer that uses immediate flushing for mouse events
func NewSmartWriter(w io.Writer) *SmartWriter {
	return &SmartWriter{
		writer: NewBufferedWriter(w, 1*time.Millisecond, 65536),
	}
}

// WriteInputEvent writes an input event using the appropriate writer
func (sw *SmartWriter) WriteInputEvent(event *protocol.InputEvent) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	// Write the event
	if err := writeInputMessage(sw.writer, event); err != nil {
		return err
	}
	
	// Check if this is a mouse movement event that needs immediate flushing
	if event.GetMouseMove() != nil || event.GetMousePosition() != nil {
		return sw.writer.Flush()
	}
	
	// Let other events use the normal buffering
	return nil
}

// Write implements io.Writer (delegates to buffered writer)
func (sw *SmartWriter) Write(p []byte) (int, error) {
	return sw.writer.Write(p)
}

// Flush flushes the writer
func (sw *SmartWriter) Flush() error {
	return sw.writer.Flush()
}

// Close closes the writer
func (sw *SmartWriter) Close() error {
	return sw.writer.Close()
}

