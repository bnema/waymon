//go:build integration

package integration

import "github.com/bnema/waymon/internal/domain"

// MonitorLayout represents predefined monitor configurations for testing.
type MonitorLayout int

const (
	// SingleMonitor represents a single 1920x1080 monitor.
	SingleMonitor MonitorLayout = iota
	// DualHorizontal represents two monitors arranged side-by-side.
	DualHorizontal
	// DualVertical represents two monitors stacked vertically.
	DualVertical
	// TripleHorizontal represents three monitors in a row.
	TripleHorizontal
	// QuadGrid represents a 2x2 monitor grid.
	QuadGrid
	// MixedScales represents monitors with different DPI scales.
	MixedScales
	// OffsetLayout represents non-aligned monitors with Y offset.
	OffsetLayout
	// PortraitMode represents monitors in portrait orientation.
	PortraitMode
	// UltraWide represents a 21:9 ultrawide monitor.
	UltraWide
	// Monitor4K represents a 4K monitor.
	Monitor4K
)

// CreateTestMonitors generates monitor configurations for testing.
func CreateTestMonitors(layout MonitorLayout) []domain.Monitor {
	switch layout {
	case SingleMonitor:
		return []domain.Monitor{
			{ID: "monitor-1", Name: "DP-1", X: 0, Y: 0, Width: 1920, Height: 1080, Scale: 1.0, Primary: true},
		}

	case DualHorizontal:
		return []domain.Monitor{
			{ID: "monitor-1", Name: "DP-1", X: 0, Y: 0, Width: 1920, Height: 1080, Scale: 1.0, Primary: true},
			{ID: "monitor-2", Name: "DP-2", X: 1920, Y: 0, Width: 1920, Height: 1080, Scale: 1.0, Primary: false},
		}

	case DualVertical:
		return []domain.Monitor{
			{ID: "monitor-1", Name: "DP-1", X: 0, Y: 0, Width: 1920, Height: 1080, Scale: 1.0, Primary: true},
			{ID: "monitor-2", Name: "DP-2", X: 0, Y: 1080, Width: 1920, Height: 1080, Scale: 1.0, Primary: false},
		}

	case TripleHorizontal:
		return []domain.Monitor{
			{ID: "monitor-1", Name: "DP-1", X: 0, Y: 0, Width: 1920, Height: 1080, Scale: 1.0, Primary: false},
			{ID: "monitor-2", Name: "DP-2", X: 1920, Y: 0, Width: 1920, Height: 1080, Scale: 1.0, Primary: true},
			{ID: "monitor-3", Name: "DP-3", X: 3840, Y: 0, Width: 1920, Height: 1080, Scale: 1.0, Primary: false},
		}

	case QuadGrid:
		return []domain.Monitor{
			{ID: "monitor-1", Name: "DP-1", X: 0, Y: 0, Width: 1920, Height: 1080, Scale: 1.0, Primary: true},
			{ID: "monitor-2", Name: "DP-2", X: 1920, Y: 0, Width: 1920, Height: 1080, Scale: 1.0, Primary: false},
			{ID: "monitor-3", Name: "DP-3", X: 0, Y: 1080, Width: 1920, Height: 1080, Scale: 1.0, Primary: false},
			{ID: "monitor-4", Name: "DP-4", X: 1920, Y: 1080, Width: 1920, Height: 1080, Scale: 1.0, Primary: false},
		}

	case MixedScales:
		return []domain.Monitor{
			// 4K at 2x scale = 1920x1080 logical
			{ID: "monitor-1", Name: "DP-1", X: 0, Y: 0, Width: 3840, Height: 2160, Scale: 2.0, Primary: true},
			// 1080p at 1x scale
			{ID: "monitor-2", Name: "DP-2", X: 1920, Y: 0, Width: 1920, Height: 1080, Scale: 1.0, Primary: false},
		}

	case OffsetLayout:
		return []domain.Monitor{
			{ID: "monitor-1", Name: "DP-1", X: 0, Y: 200, Width: 1920, Height: 1080, Scale: 1.0, Primary: true},
			{ID: "monitor-2", Name: "DP-2", X: 1920, Y: 0, Width: 2560, Height: 1440, Scale: 1.0, Primary: false},
		}

	case PortraitMode:
		return []domain.Monitor{
			{ID: "monitor-1", Name: "DP-1", X: 0, Y: 0, Width: 1920, Height: 1080, Scale: 1.0, Primary: true},
			// Rotated 90 degrees
			{ID: "monitor-2", Name: "DP-2", X: 1920, Y: 0, Width: 1080, Height: 1920, Scale: 1.0, Primary: false},
		}

	case UltraWide:
		return []domain.Monitor{
			{ID: "monitor-1", Name: "DP-1", X: 0, Y: 0, Width: 3440, Height: 1440, Scale: 1.0, Primary: true},
		}

	case Monitor4K:
		return []domain.Monitor{
			{ID: "monitor-1", Name: "DP-1", X: 0, Y: 0, Width: 3840, Height: 2160, Scale: 1.0, Primary: true},
		}

	default:
		return CreateTestMonitors(SingleMonitor)
	}
}

// MonitorLayoutName returns human-readable name for layout.
func MonitorLayoutName(layout MonitorLayout) string {
	names := map[MonitorLayout]string{
		SingleMonitor:    "SingleMonitor",
		DualHorizontal:   "DualHorizontal",
		DualVertical:     "DualVertical",
		TripleHorizontal: "TripleHorizontal",
		QuadGrid:         "QuadGrid",
		MixedScales:      "MixedScales",
		OffsetLayout:     "OffsetLayout",
		PortraitMode:     "PortraitMode",
		UltraWide:        "UltraWide",
		Monitor4K:        "Monitor4K",
	}
	if name, ok := names[layout]; ok {
		return name
	}
	return "Unknown"
}

// AllMonitorLayouts returns all defined layouts for iteration in tests.
func AllMonitorLayouts() []MonitorLayout {
	return []MonitorLayout{
		SingleMonitor,
		DualHorizontal,
		DualVertical,
		TripleHorizontal,
		QuadGrid,
		MixedScales,
		OffsetLayout,
		PortraitMode,
		UltraWide,
		Monitor4K,
	}
}

// CalculateTotalBounds calculates the bounding rectangle for a set of monitors.
func CalculateTotalBounds(monitors []domain.Monitor) (minX, minY, maxX, maxY int32) {
	if len(monitors) == 0 {
		return 0, 0, 0, 0
	}

	minX, minY = monitors[0].X, monitors[0].Y
	maxX = monitors[0].X + monitors[0].Width
	maxY = monitors[0].Y + monitors[0].Height

	for _, m := range monitors[1:] {
		if m.X < minX {
			minX = m.X
		}
		if m.Y < minY {
			minY = m.Y
		}
		if m.X+m.Width > maxX {
			maxX = m.X + m.Width
		}
		if m.Y+m.Height > maxY {
			maxY = m.Y + m.Height
		}
	}

	return minX, minY, maxX, maxY
}

// FindPrimaryMonitor returns the primary monitor from the list.
func FindPrimaryMonitor(monitors []domain.Monitor) *domain.Monitor {
	for i := range monitors {
		if monitors[i].Primary {
			return &monitors[i]
		}
	}
	if len(monitors) > 0 {
		return &monitors[0]
	}
	return nil
}

// TestPosition represents a position for testing cursor behavior.
type TestPosition struct {
	Name        string
	X, Y        int32
	Description string
}

// GetTestPositions returns test positions for a monitor layout.
func GetTestPositions(monitors []domain.Monitor) []TestPosition {
	if len(monitors) == 0 {
		return nil
	}

	minX, minY, maxX, maxY := CalculateTotalBounds(monitors)
	primary := FindPrimaryMonitor(monitors)
	if primary == nil {
		return nil
	}

	centerX := primary.X + primary.Width/2
	centerY := primary.Y + primary.Height/2

	positions := []TestPosition{
		{Name: "center", X: centerX, Y: centerY, Description: "Center of primary monitor"},
		{Name: "top_left", X: minX, Y: minY, Description: "Top-left corner of total bounds"},
		{Name: "top_right", X: maxX - 1, Y: minY, Description: "Top-right corner of total bounds"},
		{Name: "bottom_left", X: minX, Y: maxY - 1, Description: "Bottom-left corner of total bounds"},
		{Name: "bottom_right", X: maxX - 1, Y: maxY - 1, Description: "Bottom-right corner of total bounds"},
		{Name: "left_edge", X: minX, Y: centerY, Description: "Left edge of total bounds"},
		{Name: "right_edge", X: maxX - 1, Y: centerY, Description: "Right edge of total bounds"},
		{Name: "top_edge", X: centerX, Y: minY, Description: "Top edge of total bounds"},
		{Name: "bottom_edge", X: centerX, Y: maxY - 1, Description: "Bottom edge of total bounds"},
	}

	// Add positions at monitor boundaries if multiple monitors
	if len(monitors) > 1 {
		for i := range monitors {
			m := &monitors[i]
			positions = append(positions,
				TestPosition{
					Name:        m.Name + "_center",
					X:           m.X + m.Width/2,
					Y:           m.Y + m.Height/2,
					Description: "Center of " + m.Name,
				},
			)
		}
	}

	return positions
}
