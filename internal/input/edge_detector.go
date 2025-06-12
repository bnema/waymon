package input

import (
	"context"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/network"
	waymonProto "github.com/bnema/waymon/internal/proto"
)

// EdgeDetector monitors cursor position and triggers edge events
type EdgeDetector struct {
	display   *display.Display
	client    *network.Client
	threshold int32

	mu        sync.Mutex
	active    bool
	capturing bool
	lastX     int32
	lastY     int32
	lastEdge  display.Edge

	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Callbacks
	onEdgeEnter func(edge display.Edge, x, y int32)
	onEdgeLeave func()
}

// NewEdgeDetector creates a new edge detector
func NewEdgeDetector(disp *display.Display, client *network.Client, threshold int32) *EdgeDetector {
	if threshold <= 0 {
		threshold = 5 // Default 5 pixels
	}

	return &EdgeDetector{
		display:   disp,
		client:    client,
		threshold: threshold,
		lastEdge:  display.EdgeNone,
	}
}

// SetCallbacks sets the edge enter/leave callbacks
func (e *EdgeDetector) SetCallbacks(onEnter func(edge display.Edge, x, y int32), onLeave func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onEdgeEnter = onEnter
	e.onEdgeLeave = onLeave
}

// Start begins monitoring cursor position
func (e *EdgeDetector) Start() error {
	e.mu.Lock()
	if e.active {
		e.mu.Unlock()
		return nil
	}

	e.active = true
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.mu.Unlock()

	// Start monitoring in a goroutine
	e.wg.Add(1)
	go e.monitorLoop(ctx)

	return nil
}

// Stop stops monitoring cursor position
func (e *EdgeDetector) Stop() {
	e.mu.Lock()
	if !e.active {
		e.mu.Unlock()
		return
	}

	e.active = false
	if e.cancel != nil {
		e.cancel()
	}
	e.mu.Unlock()

	e.wg.Wait()
}

// IsCapturing returns whether we're currently capturing mouse events
func (e *EdgeDetector) IsCapturing() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.capturing
}

// monitorLoop continuously monitors cursor position
func (e *EdgeDetector) monitorLoop(ctx context.Context) {
	defer e.wg.Done()

	// Poll interval - 16ms for ~60Hz
	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()

	// Track cursor internally since most compositors don't expose it
	var cursorX, cursorY int32

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.checkCursorPosition(cursorX, cursorY)
		}
	}
}

// checkCursorPosition checks if cursor is at an edge
func (e *EdgeDetector) checkCursorPosition(x, y int32) {
	// Since we can't get cursor position on most Wayland compositors,
	// we'll need to track it internally based on mouse movement events
	// For now, we'll use the last known position

	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.active {
		return
	}

	// Get edge at current position
	edge := e.display.GetEdge(x, y, e.threshold)

	// Check if we've entered a new edge
	if edge != display.EdgeNone && edge != e.lastEdge {
		e.lastEdge = edge
		if e.onEdgeEnter != nil && !e.capturing {
			e.onEdgeEnter(edge, x, y)
		}
	} else if edge == display.EdgeNone && e.lastEdge != display.EdgeNone {
		// We've left the edge
		e.lastEdge = edge
		if e.onEdgeLeave != nil && e.capturing {
			e.onEdgeLeave()
		}
	}
}

// StartCapture begins capturing mouse events
func (e *EdgeDetector) StartCapture(edge display.Edge) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.capturing {
		return nil
	}

	e.capturing = true

	// Send MouseEnter event to server
	if e.client != nil && e.client.IsConnected() {
		event := &waymonProto.MouseEvent{
			Type:        waymonProto.EventType_EVENT_TYPE_ENTER,
			X:           float64(e.lastX),
			Y:           float64(e.lastY),
			TimestampMs: time.Now().UnixMilli(),
		}

		return e.client.SendEvent(event)
	}

	return nil
}

// StopCapture stops capturing mouse events
func (e *EdgeDetector) StopCapture() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.capturing {
		return nil
	}

	e.capturing = false

	// Send MouseLeave event to server
	if e.client != nil && e.client.IsConnected() {
		event := &waymonProto.MouseEvent{
			Type:        waymonProto.EventType_EVENT_TYPE_LEAVE,
			X:           float64(e.lastX),
			Y:           float64(e.lastY),
			TimestampMs: time.Now().UnixMilli(),
		}

		return e.client.SendEvent(event)
	}

	return nil
}

// UpdateCursorPosition updates the internal cursor position
// This should be called whenever we receive mouse movement events
func (e *EdgeDetector) UpdateCursorPosition(x, y int32) {
	e.mu.Lock()
	e.lastX = x
	e.lastY = y
	e.mu.Unlock()

	// Check edge immediately
	e.checkCursorPosition(x, y)
}

// HandleMouseMove processes mouse movement when capturing
func (e *EdgeDetector) HandleMouseMove(dx, dy int32) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.capturing || e.client == nil || !e.client.IsConnected() {
		return nil
	}

	// Update internal position
	e.lastX += dx
	e.lastY += dy

	// Send relative movement to server
	event := &waymonProto.MouseEvent{
		Type:        waymonProto.EventType_EVENT_TYPE_MOVE,
		X:           float64(dx), // Relative movement
		Y:           float64(dy),
		TimestampMs: time.Now().UnixMilli(),
	}

	return e.client.SendEvent(event)
}

// HandleMouseButton processes mouse button events when capturing
func (e *EdgeDetector) HandleMouseButton(button waymonProto.MouseButton, pressed bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.capturing || e.client == nil || !e.client.IsConnected() {
		return nil
	}

	event := &waymonProto.MouseEvent{
		Type:        waymonProto.EventType_EVENT_TYPE_CLICK,
		Button:      button,
		X:           float64(e.lastX),
		Y:           float64(e.lastY),
		IsPressed:   pressed,
		TimestampMs: time.Now().UnixMilli(),
	}

	return e.client.SendEvent(event)
}

// HandleMouseScroll processes mouse scroll events when capturing
func (e *EdgeDetector) HandleMouseScroll(direction waymonProto.ScrollDirection, delta int32) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.capturing || e.client == nil || !e.client.IsConnected() {
		return nil
	}

	event := &waymonProto.MouseEvent{
		Type:        waymonProto.EventType_EVENT_TYPE_SCROLL,
		Direction:   direction,
		X:           float64(e.lastX),
		Y:           float64(e.lastY),
		TimestampMs: time.Now().UnixMilli(),
	}

	return e.client.SendEvent(event)
}
