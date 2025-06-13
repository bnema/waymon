package input

import (
	"fmt"
	"os"
	"os/exec"
	
	"github.com/bnema/waymon/internal/logger"
)

// PrivilegedHelper handles uinput operations with appropriate privileges
type PrivilegedHelper struct {
	needsSudo bool
	helperPath string
}

// NewPrivilegedHelper creates a new privileged helper
func NewPrivilegedHelper() (*PrivilegedHelper, error) {
	h := &PrivilegedHelper{}
	
	// First check if we have direct access to /dev/uinput
	if err := h.checkDirectAccess(); err == nil {
		logger.Info("Direct uinput access available (waymon setup completed)")
		h.needsSudo = false
		return h, nil
	}
	
	// We need sudo access
	logger.Info("No direct uinput access, will use sudo when needed")
	h.needsSudo = true
	
	// Get the path to our helper binary
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	h.helperPath = exePath
	
	return h, nil
}

// checkDirectAccess tests if we can access /dev/uinput directly
func (h *PrivilegedHelper) checkDirectAccess() error {
	file, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	file.Close()
	return nil
}

// OpenUinput opens the uinput device with appropriate privileges
func (h *PrivilegedHelper) OpenUinput() (*os.File, error) {
	if !h.needsSudo {
		// Direct access
		return os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	}
	
	// Use sudo helper - we'll implement a special command for this
	// For now, return an error
	return nil, fmt.Errorf("sudo helper not yet implemented")
}

// RunUinputCommand runs a command that needs uinput access
func (h *PrivilegedHelper) RunUinputCommand(args ...string) error {
	if !h.needsSudo {
		// We have direct access, no need for special handling
		return nil
	}
	
	// Prompt user that we need sudo
	logger.Info("Waymon needs sudo access to inject mouse events")
	logger.Info("This is only required because 'waymon setup' hasn't been run")
	
	// Run with sudo
	cmd := exec.Command("sudo", append([]string{h.helperPath, "uinput-helper"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	
	return cmd.Run()
}