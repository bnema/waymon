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

func (p *portalBackend) GetMonitors() ([]*Monitor, error) {
	// For now, we'll try multiple methods
	
	// Method 1: Try wlr-randr (wlroots compositors like Sway)
	if monitors, err := p.getMonitorsWlrRandr(); err == nil && len(monitors) > 0 {
		return monitors, nil
	}
	
	// Method 2: Try gnome-randr (GNOME)
	if monitors, err := p.getMonitorsGnomeRandr(); err == nil && len(monitors) > 0 {
		return monitors, nil
	}
	
	// Method 3: Try kscreen-doctor (KDE)
	if monitors, err := p.getMonitorsKScreen(); err == nil && len(monitors) > 0 {
		return monitors, nil
	}
	
	// Method 4: Fallback to xrandr if available (XWayland)
	if monitors, err := p.getMonitorsXRandr(); err == nil && len(monitors) > 0 {
		return monitors, nil
	}
	
	return nil, fmt.Errorf("no monitor detection method available")
}

func (p *portalBackend) getMonitorsWlrRandr() ([]*Monitor, error) {
	cmd := exec.Command("wlr-randr")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	
	// Parse wlr-randr output
	var monitors []*Monitor
	lines := strings.Split(string(output), "\n")
	var current *Monitor
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// New monitor
		if !strings.HasPrefix(line, " ") && strings.Contains(line, " (") {
			if current != nil {
				monitors = append(monitors, current)
			}
			parts := strings.Fields(line)
			if len(parts) > 0 {
				current = &Monitor{
					Name: parts[0],
					ID:   parts[0],
				}
			}
		}
		
		// Parse resolution and position
		if current != nil && strings.Contains(line, "current") {
			// Example: "  1920x1080 px, 60.000000 Hz (preferred, current)"
			parts := strings.Fields(line)
			if len(parts) > 0 && strings.Contains(parts[0], "x") {
				res := strings.Split(parts[0], "x")
				if len(res) == 2 {
					if w, err := strconv.Atoi(res[0]); err == nil {
						current.Width = int32(w)
					}
					if h, err := strconv.Atoi(res[1]); err == nil {
						current.Height = int32(h)
					}
				}
			}
		}
		
		// Parse position
		if current != nil && strings.Contains(line, "Position:") {
			// Example: "  Position: 1920,0"
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				pos := strings.TrimSpace(parts[1])
				coords := strings.Split(pos, ",")
				if len(coords) == 2 {
					if x, err := strconv.Atoi(coords[0]); err == nil {
						current.X = int32(x)
					}
					if y, err := strconv.Atoi(coords[1]); err == nil {
						current.Y = int32(y)
					}
				}
			}
		}
	}
	
	if current != nil {
		monitors = append(monitors, current)
	}
	
	// Set primary if we only have one
	if len(monitors) == 1 {
		monitors[0].Primary = true
	}
	
	return monitors, nil
}

func (p *portalBackend) getMonitorsGnomeRandr() ([]*Monitor, error) {
	// Try GNOME's gnome-randr tool
	cmd := exec.Command("gnome-randr", "query")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	
	// Parse output (this is a simplified parser)
	var monitors []*Monitor
	lines := strings.Split(string(output), "\n")
	
	for _, line := range lines {
		if strings.Contains(line, "connected") && !strings.Contains(line, "disconnected") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				monitor := &Monitor{
					Name: parts[0],
					ID:   parts[0],
				}
				
				// Look for resolution (e.g., "1920x1080+0+0")
				for _, part := range parts {
					if strings.Contains(part, "x") && strings.Contains(part, "+") {
						// Parse "1920x1080+0+0"
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

func (p *portalBackend) getMonitorsKScreen() ([]*Monitor, error) {
	cmd := exec.Command("kscreen-doctor", "-o")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	
	// Parse kscreen-doctor output
	var monitors []*Monitor
	lines := strings.Split(string(output), "\n")
	
	for _, line := range lines {
		// Look for output lines like "Output: 1 DP-2"
		if strings.HasPrefix(line, "Output:") && strings.Contains(line, "enabled") {
			// Basic parsing - would need more work for production
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				monitor := &Monitor{
					Name: parts[2],
					ID:   parts[1],
				}
				monitors = append(monitors, monitor)
			}
		}
	}
	
	if len(monitors) == 0 {
		return nil, fmt.Errorf("no monitors found in kscreen output")
	}
	
	return monitors, nil
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