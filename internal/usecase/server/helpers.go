package server

import "github.com/bnema/waymon/internal/domain"

// CalculateTotalDisplayBounds calculates the bounding box of all monitors.
func CalculateTotalDisplayBounds(monitors []domain.Monitor) domain.DisplayBounds {
	if len(monitors) == 0 {
		// Default to 1080p
		return domain.DisplayBounds{
			MinX: 0,
			MinY: 0,
			MaxX: 1920,
			MaxY: 1080,
		}
	}

	// Initialize bounds with first monitor
	bounds := domain.DisplayBounds{
		MinX: float64(monitors[0].X),
		MinY: float64(monitors[0].Y),
		MaxX: float64(monitors[0].X + monitors[0].Width),
		MaxY: float64(monitors[0].Y + monitors[0].Height),
	}

	// Expand bounds to include all monitors
	for _, monitor := range monitors[1:] {
		minX := float64(monitor.X)
		minY := float64(monitor.Y)
		maxX := float64(monitor.X + monitor.Width)
		maxY := float64(monitor.Y + monitor.Height)

		if minX < bounds.MinX {
			bounds.MinX = minX
		}
		if minY < bounds.MinY {
			bounds.MinY = minY
		}
		if maxX > bounds.MaxX {
			bounds.MaxX = maxX
		}
		if maxY > bounds.MaxY {
			bounds.MaxY = maxY
		}
	}

	return bounds
}

// FindMainMonitor finds the primary monitor or the monitor at position 0,0.
func FindMainMonitor(monitors []domain.Monitor) *domain.Monitor {
	if len(monitors) == 0 {
		return nil
	}

	// First try to find primary monitor
	for i := range monitors {
		if monitors[i].Primary {
			return &monitors[i]
		}
	}

	// If no primary monitor found, find monitor at position 0,0
	for i := range monitors {
		if monitors[i].X == 0 && monitors[i].Y == 0 {
			return &monitors[i]
		}
	}

	// If still no monitor found, return the first one
	return &monitors[0]
}

// FindPrimaryMonitor returns the primary monitor from a list.
func FindPrimaryMonitor(monitors []domain.Monitor) *domain.Monitor {
	for i := range monitors {
		if monitors[i].Primary {
			return &monitors[i]
		}
	}
	return nil
}

// FindMonitorAtOrigin returns the monitor at position (0, 0).
func FindMonitorAtOrigin(monitors []domain.Monitor) *domain.Monitor {
	for i := range monitors {
		if monitors[i].X == 0 && monitors[i].Y == 0 {
			return &monitors[i]
		}
	}
	return nil
}

// ConstrainCursorPosition constrains cursor position within the given bounds.
func ConstrainCursorPosition(x, y float64, bounds domain.DisplayBounds) (float64, float64) {
	// Constrain X coordinate
	if x < bounds.MinX {
		x = bounds.MinX
	} else if x > bounds.MaxX {
		x = bounds.MaxX
	}

	// Constrain Y coordinate
	if y < bounds.MinY {
		y = bounds.MinY
	} else if y > bounds.MaxY {
		y = bounds.MaxY
	}

	return x, y
}

// IsWithinBounds checks if a position is within the given bounds.
func IsWithinBounds(x, y float64, bounds domain.DisplayBounds) bool {
	return x >= bounds.MinX && x <= bounds.MaxX &&
		y >= bounds.MinY && y <= bounds.MaxY
}

// CalculateMonitorCenter calculates the center point of a monitor.
func CalculateMonitorCenter(monitor *domain.Monitor) (float64, float64) {
	centerX := float64(monitor.X) + float64(monitor.Width)/2
	centerY := float64(monitor.Y) + float64(monitor.Height)/2
	return centerX, centerY
}
