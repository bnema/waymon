package display

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"

	"github.com/bnema/waymon/internal/domain"
	"github.com/rs/zerolog"
)

// wlrRandrBackend uses wlr-randr for display detection.
type wlrRandrBackend struct{}

func newWlrRandrBackend(ctx context.Context) (Backend, error) {
	log := zerolog.Ctx(ctx)

	// Check if wlr-randr is available
	if _, err := exec.LookPath("wlr-randr"); err != nil {
		return nil, fmt.Errorf("wlr-randr not found")
	}

	log.Debug().Msg("wlr-randr backend available")
	return &wlrRandrBackend{}, nil
}

func (w *wlrRandrBackend) GetMonitors() ([]*domain.Monitor, error) {
	// wlr-randr needs proper Wayland environment
	cmd := exec.Command("wlr-randr", "--json")

	// If running with sudo, we need to set the environment variables
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" && os.Geteuid() == 0 {
		sudoUID := os.Getenv("SUDO_UID")
		if sudoUID == "" {
			// Try to get UID from the user
			uidCmd := exec.Command("id", "-u", sudoUser) //nolint:gosec // sudoUser is from environment variable
			if uidOutput, err := uidCmd.Output(); err == nil {
				sudoUID = strings.TrimSpace(string(uidOutput))
			}
		}

		// Set the required environment variables for wlr-randr
		cmd.Env = os.Environ()
		xdgRuntimeDir := fmt.Sprintf("/run/user/%s", sudoUID)
		cmd.Env = append(cmd.Env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntimeDir))

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
		}

		if waylandDisplay != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("WAYLAND_DISPLAY=%s", waylandDisplay))
		} else if existingDisplay := os.Getenv("WAYLAND_DISPLAY"); existingDisplay != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("WAYLAND_DISPLAY=%s", existingDisplay))
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// If JSON flag doesn't work, try parsing text output
		return w.getMonitorsText()
	}

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

	var monitors []*domain.Monitor
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
			continue
		}

		refreshRate := int32(0)
		if output.CurrentMode.Refresh > 0 {
			refreshRate = int32(output.CurrentMode.Refresh * 1000) // Convert to mHz
		}

		monitor := &domain.Monitor{
			ID:          fmt.Sprintf("%d", i),
			Name:        output.Name,
			X:           int32(x),      //nolint:gosec // display coordinates are safe to convert
			Y:           int32(y),      //nolint:gosec // display coordinates are safe to convert
			Width:       int32(width),  //nolint:gosec // display coordinates are safe to convert
			Height:      int32(height), //nolint:gosec // display coordinates are safe to convert
			Scale:       scale,
			Primary:     output.Primary,
			RefreshRate: refreshRate,
		}
		monitors = append(monitors, monitor)
	}

	if len(monitors) == 0 {
		return nil, fmt.Errorf("no active monitors found")
	}

	// Determine primary monitor
	determinePrimaryMonitor(monitors)

	return monitors, nil
}

func (w *wlrRandrBackend) getMonitorsText() ([]*domain.Monitor, error) {
	// Fallback: parse text output from wlr-randr
	cmd := exec.Command("wlr-randr")

	// If running with sudo, we need to set the environment variables
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" && os.Geteuid() == 0 {
		sudoUID := os.Getenv("SUDO_UID")
		if sudoUID == "" {
			uidCmd := exec.Command("id", "-u", sudoUser) //nolint:gosec // sudoUser is from environment variable
			if uidOutput, err := uidCmd.Output(); err == nil {
				sudoUID = strings.TrimSpace(string(uidOutput))
			}
		}

		cmd.Env = os.Environ()
		xdgRuntimeDir := fmt.Sprintf("/run/user/%s", sudoUID)
		cmd.Env = append(cmd.Env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntimeDir))

		waylandDisplay := ""
		socketPath := fmt.Sprintf("/run/user/%s", sudoUID)
		if files, err := os.ReadDir(socketPath); err == nil {
			for _, file := range files {
				if strings.HasPrefix(file.Name(), "wayland-") && !strings.HasSuffix(file.Name(), ".lock") {
					waylandDisplay = file.Name()
					break
				}
			}
		}

		if waylandDisplay != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("WAYLAND_DISPLAY=%s", waylandDisplay))
		} else if existingDisplay := os.Getenv("WAYLAND_DISPLAY"); existingDisplay != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("WAYLAND_DISPLAY=%s", existingDisplay))
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run wlr-randr: %w", err)
	}

	// Parse text output
	var monitors []*domain.Monitor
	lines := strings.Split(string(output), "\n")

	var currentMonitor *domain.Monitor
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for output name (no leading spaces in original)
		if len(line) > 0 && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			if currentMonitor != nil && currentMonitor.Width > 0 {
				monitors = append(monitors, currentMonitor)
			}

			parts := strings.Fields(line)
			if len(parts) > 0 {
				currentMonitor = &domain.Monitor{
					ID:    fmt.Sprintf("%d", len(monitors)),
					Name:  parts[0],
					Scale: 1.0,
				}
			}
		}

		if currentMonitor == nil {
			continue
		}

		// Parse enabled status
		if strings.Contains(line, "Enabled:") {
			if !strings.Contains(line, "yes") {
				currentMonitor = nil
				continue
			}
		}

		// Parse position
		if strings.Contains(line, "Position:") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "Position:" && i+1 < len(parts) {
					coords := strings.Split(parts[i+1], ",")
					if len(coords) == 2 {
						var x, y int32
						if _, err := fmt.Sscanf(coords[0], "%d", &x); err == nil {
							currentMonitor.X = x
						}
						if _, err := fmt.Sscanf(coords[1], "%d", &y); err == nil {
							currentMonitor.Y = y
						}
					}
				}
			}
		}

		// Parse mode (resolution)
		if strings.Contains(line, "current") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.Contains(part, "x") {
					dims := strings.Split(part, "x")
					if len(dims) == 2 {
						var w, h int
						if _, err := fmt.Sscanf(dims[0], "%d", &w); err == nil {
							if _, err := fmt.Sscanf(dims[1], "%d", &h); err == nil {
								// Safe conversion with bounds check to prevent integer overflow
								if w >= 0 && w <= math.MaxInt32 {
									currentMonitor.Width = int32(w)
								}
								if h >= 0 && h <= math.MaxInt32 {
									currentMonitor.Height = int32(h)
								}
							}
						}
					}
				}
			}
		}

		// Parse scale
		if strings.Contains(line, "Scale:") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "Scale:" && i+1 < len(parts) {
					var scale float64
					if _, err := fmt.Sscanf(parts[i+1], "%f", &scale); err == nil {
						currentMonitor.Scale = scale
					}
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

	// Determine primary monitor
	determinePrimaryMonitor(monitors)

	return monitors, nil
}

func (w *wlrRandrBackend) GetCursorPosition() (x, y int32, err error) {
	// wlr-randr doesn't provide cursor position
	return 0, 0, fmt.Errorf("cursor position not available via wlr-randr")
}

func (w *wlrRandrBackend) Close() error {
	return nil
}
