package display

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/bnema/waymon/internal/logger"
)

// sudoBackend runs display detection as the actual user when running with sudo
type sudoBackend struct {
	actualBackend Backend
}

func newSudoBackend() (Backend, error) {
	// Only use this backend when running with sudo
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		return nil, fmt.Errorf("not running with sudo")
	}

	return &sudoBackend{}, nil
}

func (s *sudoBackend) GetMonitors() ([]*Monitor, error) {
	sudoUser := os.Getenv("SUDO_USER")
	sudoUID := os.Getenv("SUDO_UID")
	
	if os.Getenv("WAYMON_DISPLAY_HELPER") != "1" {
		logger.Debugf("sudoBackend - SUDO_USER=%s, SUDO_UID=%s", sudoUser, sudoUID)
	}
	
	if sudoUID == "" {
		// Try to get UID from the user
		uidCmd := exec.Command("id", "-u", sudoUser)
		if uidOutput, err := uidCmd.Output(); err == nil {
			sudoUID = strings.TrimSpace(string(uidOutput))
		}
	}

	// Get the path to our executable
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Get WAYLAND_DISPLAY - look for the socket file
	waylandDisplay := ""
	
	// Check for existing wayland sockets
	socketPath := fmt.Sprintf("/run/user/%s", sudoUID)
	if files, err := os.ReadDir(socketPath); err == nil {
		for _, file := range files {
			if strings.HasPrefix(file.Name(), "wayland-") && !strings.HasSuffix(file.Name(), ".lock") {
				waylandDisplay = file.Name()
				break
			}
		}
	}
	
	// Fallback to wayland-1 which seems to be your default
	if waylandDisplay == "" {
		waylandDisplay = "wayland-1"
	}

	// Run monitors --json directly with proper environment
	cmd := exec.Command(exePath, "monitors", "--json")
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, 
		fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%s", sudoUID),
		fmt.Sprintf("WAYLAND_DISPLAY=%s", waylandDisplay))

	if os.Getenv("WAYMON_DISPLAY_HELPER") != "1" {
		logger.Debugf("Running command: %s", cmd.String())
		logger.Debugf("With XDG_RUNTIME_DIR=/run/user/%s", sudoUID)
		logger.Debugf("With WAYLAND_DISPLAY=%s", waylandDisplay)
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	cmd = exec.CommandContext(ctx, exePath, "monitors", "--json")
	cmd.Env = os.Environ()
	
	// Remove SUDO_* vars to prevent recursive sudoBackend usage
	newEnv := make([]string, 0, len(cmd.Env))
	for _, env := range cmd.Env {
		if !strings.HasPrefix(env, "SUDO_") {
			newEnv = append(newEnv, env)
		}
	}
	cmd.Env = newEnv
	
	// Add required environment
	cmd.Env = append(cmd.Env, 
		fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%s", sudoUID),
		fmt.Sprintf("WAYLAND_DISPLAY=%s", waylandDisplay),
		"WAYMON_DISPLAY_HELPER=1")

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("monitors command timed out")
	}
	if err != nil {
		return nil, fmt.Errorf("monitors command failed: %w\nOutput: %s", err, output)
	}

	// Debug output
	if os.Getenv("WAYMON_DISPLAY_HELPER") != "1" {
		logger.Debugf("Monitors command output: %s", string(output))
	}

	// Parse JSON output
	var info struct {
		Monitors []struct {
			ID      string  `json:"id"`
			Name    string  `json:"name"`
			X       int32   `json:"x"`
			Y       int32   `json:"y"`
			Width   int32   `json:"width"`
			Height  int32   `json:"height"`
			Primary bool    `json:"primary"`
			Scale   float64 `json:"scale"`
		} `json:"monitors"`
		Error string `json:"error,omitempty"`
	}

	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse monitors output: %w", err)
	}

	if info.Error != "" {
		return nil, fmt.Errorf("display detection error: %s", info.Error)
	}

	// Convert to Monitor structs
	monitors := make([]*Monitor, len(info.Monitors))
	for i, m := range info.Monitors {
		monitors[i] = &Monitor{
			ID:      m.ID,
			Name:    m.Name,
			X:       m.X,
			Y:       m.Y,
			Width:   m.Width,
			Height:  m.Height,
			Primary: m.Primary,
			Scale:   m.Scale,
		}
	}

	if len(monitors) == 0 {
		return nil, fmt.Errorf("no monitors detected")
	}

	return monitors, nil
}

func (s *sudoBackend) GetCursorPosition() (x, y int32, err error) {
	// Cursor position is not available through this backend
	return 0, 0, fmt.Errorf("cursor position not available")
}

func (s *sudoBackend) Close() error {
	if s.actualBackend != nil {
		return s.actualBackend.Close()
	}
	return nil
}

// GetEffectiveUID returns the UID to use for the current process
func GetEffectiveUID() int {
	if sudoUID := os.Getenv("SUDO_UID"); sudoUID != "" {
		if uid, err := strconv.Atoi(sudoUID); err == nil {
			return uid
		}
	}
	return os.Getuid()
}

// GetEffectiveUser returns the username to use for the current process
func GetEffectiveUser() string {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		return sudoUser
	}
	return os.Getenv("USER")
}