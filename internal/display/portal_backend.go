package display

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// portalBackend uses XDG Desktop Portal for display info
// This works on most modern Wayland compositors
type portalBackend struct {
	// We'll use shell commands for now, could use D-Bus API later
}

func newPortalBackend() (Backend, error) {
	// Check if we have necessary tools
	if _, err := exec.LookPath("gdbus"); err != nil {
		return nil, fmt.Errorf("gdbus not found")
	}

	return &portalBackend{}, nil
}

// newRandrBackend is an alias for portal backend (uses xrandr)
func newRandrBackend() (Backend, error) {
	return newPortalBackend()
}

func (p *portalBackend) GetMonitors() ([]*Monitor, error) {
	// Portal backend is now just a fallback - we prefer wlr-randr
	// Try xrandr if available (XWayland)
	if monitors, err := p.getMonitorsXRandr(); err == nil && len(monitors) > 0 {
		return monitors, nil
	}

	return nil, fmt.Errorf("no monitor detection method available")
}

func (p *portalBackend) getMonitorsXRandr() ([]*Monitor, error) {
	cmd := exec.Command("xrandr")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var monitors []*Monitor
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if strings.Contains(line, " connected") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				monitor := &Monitor{
					Name: parts[0],
					ID:   parts[0],
				}

				// Parse resolution and position
				for _, part := range parts[2:] {
					if strings.Contains(part, "x") && (strings.Contains(part, "+") || strings.HasSuffix(part, "x")) {
						// Handle "1920x1080+0+0" format
						part = strings.TrimSuffix(part, "*") // Remove refresh rate marker
						part = strings.Split(part, " ")[0]   // Take first part if space-separated

						if strings.Contains(part, "+") {
							resPos := strings.Split(part, "+")
							if len(resPos) >= 3 {
								res := strings.Split(resPos[0], "x")
								if len(res) == 2 {
									if w, err := strconv.Atoi(res[0]); err == nil {
										monitor.Width = int32(w)
									}
									if h, err := strconv.Atoi(res[1]); err == nil {
										monitor.Height = int32(h)
									}
								}
								if x, err := strconv.Atoi(resPos[1]); err == nil {
									monitor.X = int32(x)
								}
								if y, err := strconv.Atoi(resPos[2]); err == nil {
									monitor.Y = int32(y)
								}
							}
						}
						break
					}
				}

				if strings.Contains(line, "primary") {
					monitor.Primary = true
				}

				monitors = append(monitors, monitor)
			}
		}
	}

	return monitors, nil
}

func (p *portalBackend) GetCursorPosition() (x, y int32, err error) {
	// This is tricky on Wayland - we might need to use different methods

	// Method 1: Try to get from compositor-specific tools
	// Method 2: Track internally based on our own movements
	// Method 3: Use a small utility that captures mouse position

	// For now, return an error - we'll track position internally
	return 0, 0, fmt.Errorf("cursor position tracking not available on Wayland")
}

func (p *portalBackend) Close() error {
	return nil
}
