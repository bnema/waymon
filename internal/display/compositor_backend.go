package display

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// compositorBackend detects the compositor and uses appropriate method
type compositorBackend struct {
	compositor string
}

func newCompositorBackend() (Backend, error) {
	// Detect compositor type
	compositor := detectCompositor()
	if compositor == "" {
		return nil, fmt.Errorf("unable to detect Wayland compositor")
	}
	
	return &compositorBackend{compositor: compositor}, nil
}

func detectCompositor() string {
	// Check environment variables
	if desktop := getEnv("XDG_CURRENT_DESKTOP"); desktop != "" {
		switch strings.ToLower(desktop) {
		case "hyprland":
			return "hyprland"
		case "gnome", "ubuntu:gnome", "gnome-classic":
			return "gnome"
		case "kde", "plasma":
			return "kde"
		case "sway":
			return "sway"
		}
	}
	
	// Check running processes
	compositors := map[string]string{
		"Hyprland":    "hyprland",
		"gnome-shell": "gnome",
		"kwin_wayland": "kde",
		"sway":        "sway",
		"wayfire":     "wayfire",
		"river":       "river",
	}
	
	for process, name := range compositors {
		if isProcessRunning(process) {
			return name
		}
	}
	
	return ""
}

func getEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func isProcessRunning(name string) bool {
	cmd := exec.Command("pgrep", "-x", name)
	err := cmd.Run()
	return err == nil
}

func (c *compositorBackend) GetMonitors() ([]*Monitor, error) {
	switch c.compositor {
	case "hyprland":
		return c.getMonitorsHyprland()
	case "gnome":
		return c.getMonitorsGnome()
	case "kde":
		return c.getMonitorsKDE()
	case "sway":
		return c.getMonitorsSway()
	default:
		return nil, fmt.Errorf("unsupported compositor: %s", c.compositor)
	}
}

func (c *compositorBackend) getMonitorsHyprland() ([]*Monitor, error) {
	// Use hyprctl to get monitor info
	// Try common paths when running as root/sudo
	hyprctlPaths := []string{
		"hyprctl",
		"/usr/bin/hyprctl",
		"/usr/local/bin/hyprctl",
	}
	
	var cmd *exec.Cmd
	var output []byte
	var err error
	
	for _, path := range hyprctlPaths {
		cmd = exec.Command(path, "monitors", "-j")
		// If running with sudo, preserve user environment
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
			cmd.Env = append(os.Environ(), 
				fmt.Sprintf("HYPRLAND_INSTANCE_SIGNATURE=%s", os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")))
		}
		output, err = cmd.Output()
		if err == nil {
			break
		}
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to run hyprctl: %w", err)
	}
	
	// Parse JSON output
	var hyprMonitors []struct {
		ID          int     `json:"id"`
		Name        string  `json:"name"`
		Width       int     `json:"width"`
		Height      int     `json:"height"`
		X           int     `json:"x"`
		Y           int     `json:"y"`
		Scale       float64 `json:"scale"`
		Transform   int     `json:"transform"`
		Focused     bool    `json:"focused"`
		Description string  `json:"description"`
	}
	
	if err := json.Unmarshal(output, &hyprMonitors); err != nil {
		return nil, fmt.Errorf("failed to parse hyprctl output: %w", err)
	}
	
	var monitors []*Monitor
	for i, hm := range hyprMonitors {
		monitor := &Monitor{
			ID:      fmt.Sprintf("%d", hm.ID),
			Name:    hm.Name,
			X:       int32(hm.X),
			Y:       int32(hm.Y),
			Width:   int32(hm.Width),
			Height:  int32(hm.Height),
			Scale:   hm.Scale,
			Primary: i == 0 || hm.Focused, // First or focused is primary
		}
		monitors = append(monitors, monitor)
	}
	
	return monitors, nil
}

func (c *compositorBackend) getMonitorsGnome() ([]*Monitor, error) {
	// Use gdbus to query mutter/gnome-shell
	cmd := exec.Command("gdbus", "call", "--session",
		"--dest", "org.gnome.Mutter.DisplayConfig",
		"--object-path", "/org/gnome/Mutter/DisplayConfig",
		"--method", "org.gnome.Mutter.DisplayConfig.GetCurrentState")
	
	output, err := cmd.Output()
	if err != nil {
		// Fallback to using looking glass if available
		return c.getMonitorsGnomeLookingGlass()
	}
	
	// Parse the complex D-Bus output
	// This is simplified - real implementation would parse the full structure
	_ = output // TODO: Parse properly
	
	// Extract monitor info from the output
	var monitors []*Monitor
	
	// For now, try a simpler approach using gnome-monitor-config if available
	if configMonitors, err := c.getMonitorsGnomeConfig(); err == nil {
		return configMonitors, nil
	}
	
	// Basic fallback - assume single monitor
	// TODO: Implement proper D-Bus parsing
	monitor := &Monitor{
		ID:      "0",
		Name:    "Unknown",
		X:       0,
		Y:       0,
		Width:   1920, // Default fallback
		Height:  1080,
		Primary: true,
	}
	monitors = append(monitors, monitor)
	
	return monitors, nil
}

func (c *compositorBackend) getMonitorsGnomeLookingGlass() ([]*Monitor, error) {
	// Alternative method using GNOME Looking Glass
	// This would require more complex interaction
	return nil, fmt.Errorf("looking glass method not implemented")
}

func (c *compositorBackend) getMonitorsGnomeConfig() ([]*Monitor, error) {
	// Try using gnome-monitor-config if available
	cmd := exec.Command("gnome-monitor-config", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	
	// Parse the output
	var monitors []*Monitor
	lines := strings.Split(string(output), "\n")
	
	var currentMonitor *Monitor
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Look for monitor definitions
		if strings.Contains(line, "Monitor") && strings.Contains(line, "[") {
			if currentMonitor != nil {
				monitors = append(monitors, currentMonitor)
			}
			currentMonitor = &Monitor{
				Name: extractBetween(line, "[", "]"),
				ID:   fmt.Sprintf("%d", len(monitors)),
			}
		}
		
		// Parse resolution
		if currentMonitor != nil && strings.Contains(line, "x") && strings.Contains(line, "@") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.Contains(part, "x") && !strings.Contains(part, "@") {
					res := strings.Split(part, "x")
					if len(res) == 2 {
						if w, err := strconv.Atoi(res[0]); err == nil {
							currentMonitor.Width = int32(w)
						}
						if h, err := strconv.Atoi(res[1]); err == nil {
							currentMonitor.Height = int32(h)
						}
					}
				}
			}
		}
	}
	
	if currentMonitor != nil {
		monitors = append(monitors, currentMonitor)
	}
	
	return monitors, nil
}

func (c *compositorBackend) getMonitorsKDE() ([]*Monitor, error) {
	// Use kscreen-doctor or qdbus
	cmd := exec.Command("kscreen-doctor", "-j")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to parsing non-JSON output
		return c.getMonitorsKDEText()
	}
	
	// Parse JSON if available
	var kdeOutputs map[string]interface{}
	if err := json.Unmarshal(output, &kdeOutputs); err != nil {
		return c.getMonitorsKDEText()
	}
	
	// TODO: Parse KDE's JSON format properly
	_ = kdeOutputs
	
	// For now, fallback to text parsing
	return c.getMonitorsKDEText()
}

func (c *compositorBackend) getMonitorsKDEText() ([]*Monitor, error) {
	cmd := exec.Command("kscreen-doctor", "-o")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	
	// Parse text output
	_ = output // TODO: Parse kscreen-doctor output
	
	// For now, return empty
	return nil, fmt.Errorf("KDE monitor detection not fully implemented")
}

func (c *compositorBackend) getMonitorsSway() ([]*Monitor, error) {
	// Use swaymsg to get outputs
	cmd := exec.Command("swaymsg", "-t", "get_outputs", "-r")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run swaymsg: %w", err)
	}
	
	// Parse JSON output
	var swayOutputs []struct {
		Name   string `json:"name"`
		Active bool   `json:"active"`
		Rect   struct {
			X      int `json:"x"`
			Y      int `json:"y"`
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"rect"`
		Scale           float64 `json:"scale"`
		CurrentMode     struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"current_mode"`
		Focused bool `json:"focused"`
	}
	
	if err := json.Unmarshal(output, &swayOutputs); err != nil {
		return nil, fmt.Errorf("failed to parse sway output: %w", err)
	}
	
	var monitors []*Monitor
	for i, so := range swayOutputs {
		if !so.Active {
			continue
		}
		
		monitor := &Monitor{
			ID:      fmt.Sprintf("%d", i),
			Name:    so.Name,
			X:       int32(so.Rect.X),
			Y:       int32(so.Rect.Y),
			Width:   int32(so.Rect.Width),
			Height:  int32(so.Rect.Height),
			Scale:   so.Scale,
			Primary: so.Focused,
		}
		monitors = append(monitors, monitor)
	}
	
	return monitors, nil
}

func (c *compositorBackend) GetCursorPosition() (x, y int32, err error) {
	switch c.compositor {
	case "hyprland":
		return c.getCursorHyprland()
	default:
		// Most compositors don't expose cursor position
		// We'll track it internally
		return 0, 0, fmt.Errorf("cursor tracking not available on %s", c.compositor)
	}
}

func (c *compositorBackend) getCursorHyprland() (x, y int32, err error) {
	// Try to get cursor position from hyprctl
	cmd := exec.Command("hyprctl", "cursorpos", "-j")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	
	// Parse JSON output
	var pos struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	
	if err := json.Unmarshal(output, &pos); err != nil {
		// Try parsing text output
		parts := strings.Fields(string(output))
		if len(parts) >= 2 {
			if x64, err := strconv.ParseInt(parts[0], 10, 32); err == nil {
				x = int32(x64)
			}
			if y64, err := strconv.ParseInt(parts[1], 10, 32); err == nil {
				y = int32(y64)
			}
			return x, y, nil
		}
		return 0, 0, err
	}
	
	return int32(pos.X), int32(pos.Y), nil
}

func (c *compositorBackend) Close() error {
	return nil
}

// Helper function
func extractBetween(str, start, end string) string {
	s := strings.Index(str, start)
	if s == -1 {
		return ""
	}
	s += len(start)
	e := strings.Index(str[s:], end)
	if e == -1 {
		return ""
	}
	return str[s : s+e]
}