package display

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bnema/waymon/internal/logger"
)

// sudoBackend is a special backend for running display detection as the original user when running with sudo
type sudoBackend struct{}

func newSudoBackend() (Backend, error) {
	// Only use this backend when running with sudo
	if os.Getenv("SUDO_USER") == "" || os.Geteuid() != 0 {
		return nil, fmt.Errorf("not running with sudo")
	}

	return &sudoBackend{}, nil
}

func (s *sudoBackend) GetMonitors() ([]*Monitor, error) {
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		return nil, fmt.Errorf("SUDO_USER not set")
	}

	// Get the user info
	u, err := user.Lookup(sudoUser)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup user %s: %w", sudoUser, err)
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, fmt.Errorf("invalid UID: %w", err)
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return nil, fmt.Errorf("invalid GID: %w", err)
	}

	// Create a helper program that will be executed as the original user
	helperPath := filepath.Join("/tmp", "waymon-display-helper")
	helperCode := `#!/bin/bash
export XDG_RUNTIME_DIR="/run/user/$UID"

# Try to detect WAYLAND_DISPLAY if not set
if [ -z "$WAYLAND_DISPLAY" ]; then
    for sock in /run/user/$UID/wayland-*; do
        if [ -S "$sock" ] && [[ ! "$sock" =~ \.lock$ ]]; then
            export WAYLAND_DISPLAY=$(basename "$sock")
            break
        fi
    done
fi

# Try wlr-randr first
if command -v wlr-randr >/dev/null 2>&1; then
    wlr-randr --json 2>/dev/null && exit 0
fi

# Fallback to xrandr if available (for XWayland)
if command -v xrandr >/dev/null 2>&1; then
    # Convert xrandr output to JSON format
    xrandr | awk '
    BEGIN { print "[" }
    / connected/ {
        if (NR > 2) print ","
        name = $1
        primary = ($3 == "primary") ? "true" : "false"
        
        # Find resolution and position
        for (i = 1; i <= NF; i++) {
            if ($i ~ /^[0-9]+x[0-9]+\+[0-9]+\+[0-9]+/) {
                split($i, res, /[x+]/)
                width = res[1]
                height = res[2]
                x = res[3]
                y = res[4]
                break
            }
        }
        
        printf "  {\"name\": \"%s\", \"enabled\": true, \"x\": %s, \"y\": %s, \"width\": %s, \"height\": %s, \"primary\": %s, \"scale\": 1.0}",
               name, x ? x : "0", y ? y : "0", width ? width : "0", height ? height : "0", primary
    }
    END { print "\n]" }
    ' 2>/dev/null && exit 0
fi

# No display detection method available
echo "[]"
exit 1
`

	// Write the helper script
	if err := os.WriteFile(helperPath, []byte(helperCode), 0755); err != nil {
		return nil, fmt.Errorf("failed to write helper script: %w", err)
	}
	defer os.Remove(helperPath)

	// Execute the helper as the original user
	cmd := exec.Command("sudo", "-u", sudoUser, "-i", "/bin/bash", helperPath)
	
	// Set up environment
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("HOME=%s", u.HomeDir),
		fmt.Sprintf("USER=%s", sudoUser),
		fmt.Sprintf("UID=%d", uid),
		fmt.Sprintf("GID=%d", gid),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("sudo backend helper error: %v, output: %s", err, string(output))
		return nil, fmt.Errorf("failed to run display helper: %w", err)
	}

	// Parse JSON output
	var monitors []*Monitor
	if err := json.Unmarshal(output, &monitors); err != nil {
		// If JSON parsing fails, try to extract error message
		outputStr := strings.TrimSpace(string(output))
		if outputStr != "" {
			logger.Errorf("Helper output: %s", outputStr)
		}
		return nil, fmt.Errorf("failed to parse monitor data: %w", err)
	}

	if len(monitors) == 0 {
		return nil, fmt.Errorf("no monitors detected")
	}

	// Ensure we have valid monitors
	validMonitors := make([]*Monitor, 0, len(monitors))
	for _, m := range monitors {
		if m.Width > 0 && m.Height > 0 {
			validMonitors = append(validMonitors, m)
		} else {
			logger.Warnf("Skipping invalid monitor %s with dimensions %dx%d", m.Name, m.Width, m.Height)
		}
	}

	if len(validMonitors) == 0 {
		return nil, fmt.Errorf("no valid monitors found (all had 0x0 resolution)")
	}

	return validMonitors, nil
}

func (s *sudoBackend) GetCursorPosition() (x, y int32, err error) {
	// Cursor position tracking is not available via this method
	return 0, 0, fmt.Errorf("cursor position not available via sudo backend")
}

func (s *sudoBackend) Close() error {
	return nil
}