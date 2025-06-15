package display

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bnema/waymon/internal/logger"
)

// wlrRandrBackend uses wlr-randr for display detection
type wlrRandrBackend struct{}

func newWlrRandrBackend() (Backend, error) {
	// Check if wlr-randr is available
	if _, err := exec.LookPath("wlr-randr"); err != nil {
		return nil, fmt.Errorf("wlr-randr not found. Please install wlr-randr: https://gitlab.freedesktop.org/emersion/wlr-randr")
	}

	return &wlrRandrBackend{}, nil
}

func (w *wlrRandrBackend) GetMonitors() ([]*Monitor, error) {
	// wlr-randr needs proper Wayland environment
	cmd := exec.Command("wlr-randr", "--json")

	// If running with sudo, we need to set the environment variables
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" && os.Geteuid() == 0 {
		logger.Debugf("Running wlr-randr with sudo, SUDO_USER=%s", sudoUser)
		
		sudoUID := os.Getenv("SUDO_UID")
		if sudoUID == "" {
			// Try to get UID from the user
			uidCmd := exec.Command("id", "-u", sudoUser)
			if uidOutput, err := uidCmd.Output(); err == nil {
				sudoUID = strings.TrimSpace(string(uidOutput))
			}
		}

		// Set the required environment variables for wlr-randr
		cmd.Env = os.Environ()
		xdgRuntimeDir := fmt.Sprintf("/run/user/%s", sudoUID)
		cmd.Env = append(cmd.Env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntimeDir))
		logger.Debugf("Setting XDG_RUNTIME_DIR=%s", xdgRuntimeDir)

		// Detect WAYLAND_DISPLAY by looking at the socket files
		waylandDisplay := ""
		socketPath := fmt.Sprintf("/run/user/%s", sudoUID)
		if files, err := os.ReadDir(socketPath); err == nil {
			for _, file := range files {
				if strings.HasPrefix(file.Name(), "wayland-") && !strings.HasSuffix(file.Name(), ".lock") {
					waylandDisplay = file.Name()
					break
				}
			}
		} else {
			logger.Warnf("Could not read socket directory %s: %v", socketPath, err)
		}
		
		if waylandDisplay != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("WAYLAND_DISPLAY=%s", waylandDisplay))
			logger.Debugf("Detected WAYLAND_DISPLAY=%s", waylandDisplay)
		} else {
			// Try to use the existing WAYLAND_DISPLAY from the environment
			if existingDisplay := os.Getenv("WAYLAND_DISPLAY"); existingDisplay != "" {
				cmd.Env = append(cmd.Env, fmt.Sprintf("WAYLAND_DISPLAY=%s", existingDisplay))
				logger.Debugf("Using existing WAYLAND_DISPLAY=%s", existingDisplay)
			} else {
				logger.Warn("Could not detect WAYLAND_DISPLAY for sudo session")
			}
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Log the error output for debugging
		if len(output) > 0 {
			logger.Errorf("wlr-randr --json error: %s", string(output))
		}
		logger.Debug("JSON mode failed, falling back to text parsing")
		// If JSON flag doesn't work, try parsing text output
		return w.getMonitorsText()
	}
	
	logger.Debugf("wlr-randr --json output: %s", string(output))

	// Parse JSON output
	var outputs []struct {
		Name         string  `json:"name"`
		Enabled      bool    `json:"enabled"`
		X            int     `json:"x"`
		Y            int     `json:"y"`
		Width        int     `json:"width"`
		Height       int     `json:"height"`
		Scale        float64 `json:"scale"`
		Transform    string  `json:"transform"`
		Primary      bool    `json:"primary"`
		Model        string  `json:"model"`
		Manufacturer string  `json:"manufacturer"`
		CurrentMode  struct {
			Width   int     `json:"width"`
			Height  int     `json:"height"`
			Refresh float64 `json:"refresh"`
		} `json:"current_mode"`
		Position struct {
			X int `json:"x"`
			Y int `json:"y"`
		} `json:"position"`
	}

	if err := json.Unmarshal(output, &outputs); err != nil {
		// Fallback to text parsing
		return w.getMonitorsText()
	}

	var monitors []*Monitor
	for i, output := range outputs {
		if !output.Enabled {
			continue
		}

		// Use current mode dimensions if available
		width := output.Width
		height := output.Height
		if output.CurrentMode.Width > 0 {
			width = output.CurrentMode.Width
			height = output.CurrentMode.Height
		}
		
		logger.Debugf("Monitor %s: base dimensions %dx%d, current mode %dx%d", 
			output.Name, output.Width, output.Height, 
			output.CurrentMode.Width, output.CurrentMode.Height)

		// Use position if available
		x := output.X
		y := output.Y
		if output.Position.X != 0 || output.Position.Y != 0 {
			x = output.Position.X
			y = output.Position.Y
		}

		scale := output.Scale
		if scale == 0 {
			scale = 1.0
		}

		// Skip monitors with invalid dimensions
		if width == 0 || height == 0 {
			logger.Warnf("Skipping monitor %s with invalid dimensions: %dx%d", output.Name, width, height)
			continue
		}

		monitor := &Monitor{
			ID:      fmt.Sprintf("%d", i),
			Name:    output.Name,
			X:       int32(x),
			Y:       int32(y),
			Width:   int32(width),
			Height:  int32(height),
			Scale:   scale,
			Primary: output.Primary,
		}
		monitors = append(monitors, monitor)
	}

	if len(monitors) == 0 {
		return nil, fmt.Errorf("no active monitors found")
	}

	// If no monitor is explicitly marked as primary, use the one at position (0,0)
	hasPrimary := false
	for _, m := range monitors {
		if m.Primary {
			hasPrimary = true
			break
		}
	}

	if !hasPrimary {
		for _, m := range monitors {
			if m.X == 0 && m.Y == 0 {
				m.Primary = true
				break
			}
		}
		// If still no primary (no monitor at 0,0), fall back to first monitor
		if !hasPrimary && len(monitors) > 0 {
			monitors[0].Primary = true
		}
	}

	return monitors, nil
}

func (w *wlrRandrBackend) getMonitorsText() ([]*Monitor, error) {
	// Fallback: parse text output from wlr-randr
	cmd := exec.Command("wlr-randr")

	// If running with sudo, we need to set the environment variables
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" && os.Geteuid() == 0 {
		logger.Debugf("Running wlr-randr with sudo, SUDO_USER=%s", sudoUser)
		
		sudoUID := os.Getenv("SUDO_UID")
		if sudoUID == "" {
			// Try to get UID from the user
			uidCmd := exec.Command("id", "-u", sudoUser)
			if uidOutput, err := uidCmd.Output(); err == nil {
				sudoUID = strings.TrimSpace(string(uidOutput))
			}
		}

		// Set the required environment variables for wlr-randr
		cmd.Env = os.Environ()
		xdgRuntimeDir := fmt.Sprintf("/run/user/%s", sudoUID)
		cmd.Env = append(cmd.Env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntimeDir))
		logger.Debugf("Setting XDG_RUNTIME_DIR=%s", xdgRuntimeDir)

		// Detect WAYLAND_DISPLAY by looking at the socket files
		waylandDisplay := ""
		socketPath := fmt.Sprintf("/run/user/%s", sudoUID)
		if files, err := os.ReadDir(socketPath); err == nil {
			for _, file := range files {
				if strings.HasPrefix(file.Name(), "wayland-") && !strings.HasSuffix(file.Name(), ".lock") {
					waylandDisplay = file.Name()
					break
				}
			}
		} else {
			logger.Warnf("Could not read socket directory %s: %v", socketPath, err)
		}
		
		if waylandDisplay != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("WAYLAND_DISPLAY=%s", waylandDisplay))
			logger.Debugf("Detected WAYLAND_DISPLAY=%s", waylandDisplay)
		} else {
			// Try to use the existing WAYLAND_DISPLAY from the environment
			if existingDisplay := os.Getenv("WAYLAND_DISPLAY"); existingDisplay != "" {
				cmd.Env = append(cmd.Env, fmt.Sprintf("WAYLAND_DISPLAY=%s", existingDisplay))
				logger.Debugf("Using existing WAYLAND_DISPLAY=%s", existingDisplay)
			} else {
				logger.Warn("Could not detect WAYLAND_DISPLAY for sudo session")
			}
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			logger.Errorf("wlr-randr error: %s", string(output))
		}
		return nil, fmt.Errorf("failed to run wlr-randr: %w", err)
	}

	// Parse text output
	var monitors []*Monitor
	lines := strings.Split(string(output), "\n")

	var currentMonitor *Monitor
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for output name (no leading spaces)
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			if currentMonitor != nil {
				monitors = append(monitors, currentMonitor)
			}

			// Extract output name
			parts := strings.Fields(line)
			if len(parts) > 0 {
				currentMonitor = &Monitor{
					ID:    fmt.Sprintf("%d", len(monitors)),
					Name:  parts[0],
					Scale: 1.0, // Default scale
				}
			}
		}

		if currentMonitor == nil {
			continue
		}

		// Parse enabled status
		if strings.Contains(line, "Enabled:") {
			if strings.Contains(line, "yes") {
				// Keep processing
			} else {
				// Skip disabled monitors
				currentMonitor = nil
				continue
			}
		}

		// Parse position
		if strings.Contains(line, "Position:") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "Position:" && i+1 < len(parts) {
					// Format is usually "x,y"
					coords := strings.Split(parts[i+1], ",")
					if len(coords) == 2 {
						fmt.Sscanf(coords[0], "%d", &currentMonitor.X)
						fmt.Sscanf(coords[1], "%d", &currentMonitor.Y)
					}
				}
			}
		}

		// Parse mode (resolution)
		if strings.Contains(line, "current") {
			// Format: "  1920x1080 px, 60.000000 Hz (current)"
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.Contains(part, "x") {
					dims := strings.Split(part, "x")
					if len(dims) == 2 {
						var w, h int
						fmt.Sscanf(dims[0], "%d", &w)
						fmt.Sscanf(dims[1], "%d", &h)
						currentMonitor.Width = int32(w)
						currentMonitor.Height = int32(h)
					}
				}
			}
		}

		// Parse scale
		if strings.Contains(line, "Scale:") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "Scale:" && i+1 < len(parts) {
					fmt.Sscanf(parts[i+1], "%f", &currentMonitor.Scale)
				}
			}
		}
	}

	// Add the last monitor
	if currentMonitor != nil && currentMonitor.Width > 0 {
		monitors = append(monitors, currentMonitor)
	}

	if len(monitors) == 0 {
		return nil, fmt.Errorf("no monitors detected from wlr-randr output")
	}

	// If no monitor is explicitly marked as primary, use the one at position (0,0)
	hasPrimary := false
	for _, m := range monitors {
		if m.Primary {
			hasPrimary = true
			break
		}
	}

	if !hasPrimary {
		for _, m := range monitors {
			if m.X == 0 && m.Y == 0 {
				m.Primary = true
				hasPrimary = true
				break
			}
		}
		// If still no primary (no monitor at 0,0), fall back to first monitor
		if !hasPrimary && len(monitors) > 0 {
			monitors[0].Primary = true
		}
	}

	return monitors, nil
}

func (w *wlrRandrBackend) GetCursorPosition() (x, y int32, err error) {
	// wlr-randr doesn't provide cursor position
	return 0, 0, fmt.Errorf("cursor position not available via wlr-randr")
}

func (w *wlrRandrBackend) Close() error {
	return nil
}
