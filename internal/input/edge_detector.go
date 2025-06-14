package input

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/network"
	waymonProto "github.com/bnema/waymon/internal/proto"
)

// EdgeDetector monitors cursor position and triggers edge events
type EdgeDetector struct {
	display      *display.Display
	client       network.Client
	threshold    int32
	edgeMappings []config.EdgeMapping

	mu         sync.Mutex
	active     bool
	capturing  bool
	lastX      int32
	lastY      int32
	lastEdge   display.Edge
	activeHost string // Currently connected host

	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Callbacks
	onEdgeEnter func(edge display.Edge, x, y int32)
	onEdgeLeave func()
}

// NewEdgeDetector creates a new edge detector
func NewEdgeDetector(disp *display.Display, client network.Client, threshold int32) *EdgeDetector {
	if threshold <= 0 {
		threshold = 5 // Default 5 pixels
	}

	cfg := config.Get()
	logger.Infof("EdgeDetector: Created with %d edge mappings", len(cfg.Client.EdgeMappings))
	for _, mapping := range cfg.Client.EdgeMappings {
		logger.Debugf("  Edge mapping: %s edge of monitor '%s' -> host '%s'",
			mapping.Edge, mapping.MonitorID, mapping.Host)
	}

	return &EdgeDetector{
		display:      disp,
		client:       client,
		threshold:    threshold,
		edgeMappings: cfg.Client.EdgeMappings,
		lastEdge:     display.EdgeNone,
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

	// Since we track cursor internally, we poll our last known position

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Use our tracked position
			e.checkCursorPosition(e.lastX, e.lastY)
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

		// Find which host this edge should connect to
		monitor := e.display.GetMonitorAt(x, y)
		if monitor != nil && !e.capturing {
			host := e.getHostForEdge(monitor, edge)
			if host != "" {
				logger.Infof("Edge detected: %s edge of monitor '%s' -> connecting to host '%s'",
					edge.String(), monitor.Name, host)
				e.activeHost = host
				if e.onEdgeEnter != nil {
					e.onEdgeEnter(edge, x, y)
				}
			} else {
				logger.Debugf("Edge detected: %s edge of monitor '%s' but no mapping found",
					edge.String(), monitor.Name)
			}
		}
	} else if edge == display.EdgeNone && e.lastEdge != display.EdgeNone {
		// We've left the edge
		e.lastEdge = edge
		if e.onEdgeLeave != nil && e.capturing {
			logger.Info("Left screen edge, stopping capture")
			e.onEdgeLeave()
		}
	}
}

// StartCapture begins capturing mouse events for a specific edge/monitor
func (e *EdgeDetector) StartCapture(edge display.Edge, monitor *display.Monitor) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.capturing {
		logger.Debug("Already capturing, ignoring StartCapture")
		return nil
	}

	// Find the host for this edge/monitor combination
	host := e.getHostForEdge(monitor, edge)
	if host == "" {
		return fmt.Errorf("no host configured for %s edge of monitor %s", edge.String(), monitor.Name)
	}

	e.capturing = true
	e.activeHost = host
	e.lastEdge = edge
	logger.Infof("Starting mouse capture mode - %s edge of %s -> host '%s'", edge.String(), monitor.Name, host)

	// Send MouseEnter event to server
	if e.client != nil && e.client.IsConnected() {
		event := &network.MouseEvent{
			MouseEvent: &waymonProto.MouseEvent{
				Type:        waymonProto.EventType_EVENT_TYPE_ENTER,
				X:           float64(e.lastX),
				Y:           float64(e.lastY),
				TimestampMs: time.Now().UnixMilli(),
			},
		}

		logger.Debugf("Sending MouseEnter event: pos=(%d,%d)", e.lastX, e.lastY)
		return e.client.SendMouseEvent(event)
	}

	return nil
}

// StopCapture stops capturing mouse events
func (e *EdgeDetector) StopCapture() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.capturing {
		logger.Debug("Not capturing, ignoring StopCapture")
		return nil
	}

	e.capturing = false
	logger.Info("Stopping mouse capture mode")

	// Send MouseLeave event to server
	if e.client != nil && e.client.IsConnected() {
		event := &network.MouseEvent{
			MouseEvent: &waymonProto.MouseEvent{
				Type:        waymonProto.EventType_EVENT_TYPE_LEAVE,
				X:           float64(e.lastX),
				Y:           float64(e.lastY),
				TimestampMs: time.Now().UnixMilli(),
			},
		}

		logger.Debugf("Sending MouseLeave event: pos=(%d,%d)", e.lastX, e.lastY)
		return e.client.SendMouseEvent(event)
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

	logger.Debugf("Mouse move: dx=%d, dy=%d, pos=(%d,%d)", dx, dy, e.lastX, e.lastY)

	// Send relative movement to server
	event := &network.MouseEvent{
		MouseEvent: &waymonProto.MouseEvent{
			Type:        waymonProto.EventType_EVENT_TYPE_MOVE,
			X:           float64(dx), // Relative movement
			Y:           float64(dy),
			TimestampMs: time.Now().UnixMilli(),
		},
	}

	return e.client.SendMouseEvent(event)
}

// HandleMouseButton processes mouse button events when capturing
func (e *EdgeDetector) HandleMouseButton(button waymonProto.MouseButton, pressed bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.capturing || e.client == nil || !e.client.IsConnected() {
		return nil
	}

	event := &network.MouseEvent{
		MouseEvent: &waymonProto.MouseEvent{
			Type:        waymonProto.EventType_EVENT_TYPE_CLICK,
			Button:      button,
			X:           float64(e.lastX),
			Y:           float64(e.lastY),
			IsPressed:   pressed,
			TimestampMs: time.Now().UnixMilli(),
		},
	}

	return e.client.SendMouseEvent(event)
}

// HandleMouseScroll processes mouse scroll events when capturing
func (e *EdgeDetector) HandleMouseScroll(direction waymonProto.ScrollDirection, delta int32) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.capturing || e.client == nil || !e.client.IsConnected() {
		return nil
	}

	logger.Debugf("Mouse scroll: direction=%s, delta=%d", direction.String(), delta)

	event := &network.MouseEvent{
		MouseEvent: &waymonProto.MouseEvent{
			Type:        waymonProto.EventType_EVENT_TYPE_SCROLL,
			Direction:   direction,
			X:           float64(e.lastX),
			Y:           float64(e.lastY),
			TimestampMs: time.Now().UnixMilli(),
		},
	}

	return e.client.SendMouseEvent(event)
}

// getHostForEdge finds which host to connect to based on monitor and edge
func (e *EdgeDetector) getHostForEdge(monitor *display.Monitor, edge display.Edge) string {
	edgeStr := edge.String()

	// Look for exact monitor match
	for _, mapping := range e.edgeMappings {
		if mapping.Edge == edgeStr {
			// Check monitor match
			if mapping.MonitorID == monitor.ID ||
				mapping.MonitorID == monitor.Name ||
				(mapping.MonitorID == "primary" && monitor.Primary) ||
				mapping.MonitorID == "*" {
				return mapping.Host
			}
		}
	}

	// Fallback to legacy screen_position if no mappings configured
	cfg := config.Get()
	if len(e.edgeMappings) == 0 && cfg.Client.ScreenPosition != "" && cfg.Client.ServerAddress != "" {
		// Only use legacy config if edge matches screen position
		if cfg.Client.ScreenPosition == edgeStr {
			return cfg.Client.ServerAddress
		}
	}

	return ""
}
