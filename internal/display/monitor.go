// Package display handles monitor detection and cursor tracking
package display

import (
	"fmt"
)

// Monitor represents a physical display
type Monitor struct {
	ID       string
	Name     string
	X        int32 // Position in global coordinate space
	Y        int32
	Width    int32
	Height   int32
	Primary  bool
	Scale    float64
}

// Bounds returns the monitor's boundaries
func (m *Monitor) Bounds() (x1, y1, x2, y2 int32) {
	return m.X, m.Y, m.X + m.Width, m.Y + m.Height
}

// Contains checks if a point is within this monitor
func (m *Monitor) Contains(x, y int32) bool {
	return x >= m.X && x < m.X+m.Width && y >= m.Y && y < m.Y+m.Height
}

// Display manages monitor configuration and cursor tracking
type Display struct {
	monitors []*Monitor
	backend  Backend
}

// Backend interface for different display detection methods
type Backend interface {
	GetMonitors() ([]*Monitor, error)
	GetCursorPosition() (x, y int32, err error)
	Close() error
}

// New creates a new display manager
func New() (*Display, error) {
	// Try different backends in order of preference
	backends := []func() (Backend, error){
		newCompositorBackend, // Our compositor-specific backend
		newWaylandBackend,    // Direct Wayland protocols
		newPortalBackend,     // XDG Desktop Portal
		newRandrBackend,      // X11/XWayland fallback
		newSysfsBackend,      // /sys/class/drm fallback
	}

	var backend Backend
	var err error
	
	for _, createBackend := range backends {
		backend, err = createBackend()
		if err == nil {
			break
		}
	}
	
	if backend == nil {
		return nil, fmt.Errorf("no display backend available")
	}

	monitors, err := backend.GetMonitors()
	if err != nil {
		backend.Close()
		return nil, err
	}

	return &Display{
		monitors: monitors,
		backend:  backend,
	}, nil
}

// GetMonitors returns all detected monitors
func (d *Display) GetMonitors() []*Monitor {
	return d.monitors
}

// GetPrimaryMonitor returns the primary monitor
func (d *Display) GetPrimaryMonitor() *Monitor {
	for _, m := range d.monitors {
		if m.Primary {
			return m
		}
	}
	// Fallback to first monitor
	if len(d.monitors) > 0 {
		return d.monitors[0]
	}
	return nil
}

// GetMonitorAt returns the monitor containing the given coordinates
func (d *Display) GetMonitorAt(x, y int32) *Monitor {
	for _, m := range d.monitors {
		if m.Contains(x, y) {
			return m
		}
	}
	return nil
}

// GetCursorPosition returns the current cursor position
func (d *Display) GetCursorPosition() (x, y int32, monitor *Monitor, err error) {
	x, y, err = d.backend.GetCursorPosition()
	if err != nil {
		return 0, 0, nil, err
	}
	
	monitor = d.GetMonitorAt(x, y)
	return x, y, monitor, nil
}

// GetEdge determines which edge the cursor is near
func (d *Display) GetEdge(x, y int32, threshold int32) Edge {
	monitor := d.GetMonitorAt(x, y)
	if monitor == nil {
		return EdgeNone
	}

	x1, y1, x2, y2 := monitor.Bounds()
	
	// Check each edge with threshold
	if x-x1 < threshold {
		return EdgeLeft
	}
	if x2-x < threshold {
		return EdgeRight
	}
	if y-y1 < threshold {
		return EdgeTop
	}
	if y2-y < threshold {
		return EdgeBottom
	}
	
	return EdgeNone
}

// Close cleans up resources
func (d *Display) Close() error {
	if d.backend != nil {
		return d.backend.Close()
	}
	return nil
}

// Edge represents screen edges
type Edge int

const (
	EdgeNone Edge = iota
	EdgeLeft
	EdgeRight
	EdgeTop
	EdgeBottom
)

func (e Edge) String() string {
	switch e {
	case EdgeLeft:
		return "left"
	case EdgeRight:
		return "right"
	case EdgeTop:
		return "top"
	case EdgeBottom:
		return "bottom"
	default:
		return "none"
	}
}