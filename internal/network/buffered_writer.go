package network

import (
	"io"
	"sync"
	"time"
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
	}

	go bw.flushLoop()
	return bw
}

// Write implements io.Writer
func (bw *BufferedWriter) Write(p []byte) (int, error) {
	bw.mu.Lock()
	defer bw.mu.Unlock()

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

