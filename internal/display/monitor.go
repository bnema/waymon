// Package display handles monitor detection and cursor tracking
package display

import (
	"fmt"
	"os"

	"github.com/bnema/waymon/internal/logger"
)

// Monitor represents a physical display
type Monitor struct {
	ID      string
	Name    string
	X       int32 // Position in global coordinate space
	Y       int32
	Width   int32
	Height  int32
	Primary bool
	Scale   float64
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
	logger.Debug("Display.New: Starting display manager creation")

	// Try different backends in order of preference
	backends := []func() (Backend, error){
		newSudoBackend,                // Special backend for sudo that runs as original user
		newWlrOutputManagementBackend, // Native wlr-output-management protocol
		newWlrCgoBackend,              // Basic Wayland via CGO (limited info)
		newWlrRandrBackend,            // wlr-randr command (fallback)
		newPortalBackend,              // XDG Desktop Portal (xrandr fallback)
		newRandrBackend,               // X11/XWayland fallback
	}

	var backend Backend
	var err error

	backendNames := []string{"sudoBackend", "wlrOutputManagementBackend", "wlrCgoBackend", "wlrRandrBackend", "portalBackend", "randrBackend"}

	for i, createBackend := range backends {
		// Only show debug if not running as monitors --json
		if os.Getenv("WAYMON_DISPLAY_HELPER") != "1" {
			logger.Debugf("Display.New: Trying backend %d: %s", i, backendNames[i])
		}

		backend, err = createBackend()

		if os.Getenv("WAYMON_DISPLAY_HELPER") != "1" {
			if err == nil {
				logger.Debugf("Display.New: Successfully created backend: %s", backendNames[i])
				break
			}
			logger.Debugf("Display.New: Backend %s failed: %v", backendNames[i], err)
		} else if err == nil {
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

// determinePrimaryMonitor sets the primary monitor based on position
// The monitor at position (0,0) is considered primary, with fallback to first monitor
func determinePrimaryMonitor(monitors []*Monitor) {
	// Reset all monitors to non-primary
	for _, monitor := range monitors {
		monitor.Primary = false
	}

	// Determine primary monitor - the one at position (0,0) should be primary
	// If no monitor is at (0,0), use the first monitor as fallback
	primarySet := false
	for _, monitor := range monitors {
		if monitor.X == 0 && monitor.Y == 0 {
			monitor.Primary = true
			primarySet = true
			break
		}
	}

	// Fallback to first monitor if no monitor is at (0,0)
	if !primarySet && len(monitors) > 0 {
		monitors[0].Primary = true
	}
}
