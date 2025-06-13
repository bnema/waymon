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
	"github.com/bnema/waymon/internal/proto"
)

// WaylandBarrier implements edge barriers using Wayland pointer constraints
// This is the proper way to implement mouse sharing on Wayland:
// 1. Create invisible edge windows at screen boundaries
// 2. Use pointer constraints to confine/lock when hovering edges
// 3. Capture relative movements while locked
// 4. Release constraint when returning from remote
type WaylandBarrier struct {
	display      *display.Display
	client       network.Client
	threshold    int32
	edgeMappings []config.EdgeMapping
	
	mu           sync.Mutex
	active       bool
	capturing    bool
	currentEdge  display.Edge
	activeHost   string
	
	// Pointer constraint state
	locked       bool
	lockX, lockY float64 // Position where we locked
	
	// Callbacks
	onEdgeEnter  func(edge display.Edge, host string)
	onEdgeLeave  func()
	
	wg           sync.WaitGroup
	cancel       context.CancelFunc
}

// NewWaylandBarrier creates a new Wayland-compatible edge barrier
func NewWaylandBarrier(disp *display.Display, client network.Client, threshold int32) *WaylandBarrier {
	cfg := config.Get()
	
	return &WaylandBarrier{
		display:      disp,
		client:       client,
		threshold:    threshold,
		edgeMappings: cfg.Client.EdgeMappings,
	}
}

// SetCallbacks sets the edge enter/leave callbacks
func (w *WaylandBarrier) SetCallbacks(onEnter func(edge display.Edge, host string), onLeave func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onEdgeEnter = onEnter
	w.onEdgeLeave = onLeave
}

// Start begins the barrier system
func (w *WaylandBarrier) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.active {
		return nil
	}
	
	w.active = true
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	
	// Start barrier windows
	w.wg.Add(1)
	go w.runBarriers(ctx)
	
	logger.Info("Wayland barrier system started")
	logger.Infof("Monitoring %d edge mappings", len(w.edgeMappings))
	
	return nil
}

// Stop stops the barrier system
func (w *WaylandBarrier) Stop() {
	w.mu.Lock()
	if !w.active {
		w.mu.Unlock()
		return
	}
	
	w.active = false
	if w.cancel != nil {
		w.cancel()
	}
	w.mu.Unlock()
	
	w.wg.Wait()
	logger.Info("Wayland barrier system stopped")
}

// runBarriers manages the edge barrier windows
func (w *WaylandBarrier) runBarriers(ctx context.Context) {
	defer w.wg.Done()
	
	// In a full implementation, this would:
	// 1. Create transparent Wayland surfaces at screen edges
	// 2. Use zwp_pointer_constraints_v1 to set up confinement regions
	// 3. Listen for pointer enter/leave events
	
	// For now, we'll simulate with a simple approach
	ticker := time.NewTicker(50 * time.Millisecond) // 20Hz for demo
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// In production, this would be event-driven
			w.checkEdgeHover()
		}
	}
}

// checkEdgeHover checks if pointer is hovering an edge
func (w *WaylandBarrier) checkEdgeHover() {
	// In a real implementation, this would be triggered by Wayland events
	// For now, we'll use a simplified approach
	
	// This is where the magic happens:
	// 1. Wayland surface receives pointer_enter event
	// 2. We request pointer lock using zwp_locked_pointer_v1
	// 3. While locked, we get relative motion events
	// 4. On unlock, we release control
}

// StartCapture activates pointer lock for edge
func (w *WaylandBarrier) StartCapture(edge display.Edge, monitor *display.Monitor) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.capturing {
		return nil
	}
	
	// Find target host for this edge
	host := w.getHostForEdge(monitor, edge)
	if host == "" {
		return fmt.Errorf("no host configured for %s edge of monitor %s", edge.String(), monitor.Name)
	}
	
	w.capturing = true
	w.currentEdge = edge
	w.activeHost = host
	w.locked = true
	
	logger.Infof("Activating edge barrier: %s edge -> %s", edge.String(), host)
	logger.Info("Pointer locked - move mouse to control remote system")
	
	// Send enter event to remote
	if w.client != nil && w.client.IsConnected() {
		event := &network.MouseEvent{
			MouseEvent: &proto.MouseEvent{
				Type:        proto.EventType_EVENT_TYPE_ENTER,
				X:           w.lockX,
				Y:           w.lockY,
				TimestampMs: time.Now().UnixMilli(),
			},
		}
		w.client.SendMouseEvent(event)
	}
	
	// Notify callback
	if w.onEdgeEnter != nil {
		w.onEdgeEnter(edge, host)
	}
	
	return nil
}

// StopCapture releases pointer lock
func (w *WaylandBarrier) StopCapture() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if !w.capturing {
		return nil
	}
	
	w.capturing = false
	w.locked = false
	
	logger.Info("Releasing edge barrier - pointer unlocked")
	
	// Send leave event to remote
	if w.client != nil && w.client.IsConnected() {
		event := &network.MouseEvent{
			MouseEvent: &proto.MouseEvent{
				Type:        proto.EventType_EVENT_TYPE_LEAVE,
				X:           w.lockX,
				Y:           w.lockY,
				TimestampMs: time.Now().UnixMilli(),
			},
		}
		w.client.SendMouseEvent(event)
	}
	
	// Notify callback
	if w.onEdgeLeave != nil {
		w.onEdgeLeave()
	}
	
	return nil
}

// HandleRelativeMotion processes relative pointer motion while locked
func (w *WaylandBarrier) HandleRelativeMotion(dx, dy float64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if !w.capturing || !w.locked {
		return nil
	}
	
	// Send relative motion to remote
	if w.client != nil && w.client.IsConnected() {
		event := &network.MouseEvent{
			MouseEvent: &proto.MouseEvent{
				Type:        proto.EventType_EVENT_TYPE_MOVE,
				X:           dx,
				Y:           dy,
				TimestampMs: time.Now().UnixMilli(),
			},
		}
		
		logger.Debugf("Sending relative motion: dx=%.2f, dy=%.2f", dx, dy)
		return w.client.SendMouseEvent(event)
	}
	
	return nil
}

// HandleButton processes button events while locked
func (w *WaylandBarrier) HandleButton(button proto.MouseButton, pressed bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if !w.capturing || !w.locked {
		return nil
	}
	
	if w.client != nil && w.client.IsConnected() {
		event := &network.MouseEvent{
			MouseEvent: &proto.MouseEvent{
				Type:        proto.EventType_EVENT_TYPE_CLICK,
				Button:      button,
				IsPressed:   pressed,
				TimestampMs: time.Now().UnixMilli(),
			},
		}
		
		action := "pressed"
		if !pressed {
			action = "released"
		}
		logger.Debugf("Sending button %s: %s", button.String(), action)
		
		return w.client.SendMouseEvent(event)
	}
	
	return nil
}

// getHostForEdge finds which host to connect to based on monitor and edge
func (w *WaylandBarrier) getHostForEdge(monitor *display.Monitor, edge display.Edge) string {
	edgeStr := edge.String()
	
	// Look for exact monitor match
	for _, mapping := range w.edgeMappings {
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
	if len(w.edgeMappings) == 0 && cfg.Client.ScreenPosition != "" && cfg.Client.ServerAddress != "" {
		// Only use legacy config if edge matches screen position
		if cfg.Client.ScreenPosition == edgeStr {
			return cfg.Client.ServerAddress
		}
	}
	
	return ""
}

// IsCapturing returns whether we're currently capturing
func (w *WaylandBarrier) IsCapturing() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.capturing
}

// GetActiveHost returns the currently connected host
func (w *WaylandBarrier) GetActiveHost() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.activeHost
}