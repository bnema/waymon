package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/bnema/waymon/internal/logger"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup secure uinput permissions for Waymon",
	Long: `Setup secure uinput permissions for Waymon server mode.
This command creates a dedicated 'waymon' group and configures udev rules
to allow only Waymon users access to the uinput kernel module.`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	logger.Info("Waymon uinput Setup")
	logger.Info("==================")
	logger.Info("")

	// Check if running as root
	if os.Geteuid() == 0 {
		logger.Info("Please run this command as a normal user (not root)")
		logger.Info("The setup will use sudo when needed")
		return fmt.Errorf("cannot run setup as root")
	}

	// Check if uinput module is loaded
	if err := checkAndLoadUinput(); err != nil {
		return err
	}

	// Check if /dev/uinput exists
	if err := checkUinputDevice(); err != nil {
		return err
	}

	// Show current permissions
	if err := showCurrentPermissions(); err != nil {
		return err
	}

	// Create secure udev rule setup
	if err := createSecureSetup(); err != nil {
		return err
	}

	// Test access
	return testUinputAccess()
}

func checkAndLoadUinput() error {
	// Check if uinput module is loaded
	cmd := exec.Command("lsmod")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check loaded modules: %w", err)
	}

	if strings.Contains(string(output), "uinput") {
		logger.Info("✓ uinput module already loaded")
		return nil
	}

	logger.Info("Loading uinput module...")
	cmd = exec.Command("sudo", "modprobe", "uinput")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to load uinput module: %w", err)
	}

	logger.Info("✓ uinput module loaded")
	return nil
}

func checkUinputDevice() error {
	// Check if /dev/uinput exists
	info, err := os.Stat("/dev/uinput")
	if err != nil {
		if os.IsNotExist(err) {
			logger.Error("✗ /dev/uinput not found - this might be a problem")
			return fmt.Errorf("/dev/uinput not found")
		}
		return fmt.Errorf("failed to check /dev/uinput: %w", err)
	}

	// Check if it's a character device
	if info.Mode()&os.ModeCharDevice == 0 {
	logger.Error("✗ /dev/uinput is not a character device")
	return fmt.Errorf("/dev/uinput is not a character device")
	}

	logger.Info("✓ /dev/uinput exists")
	return nil
}

func showCurrentPermissions() error {
	logger.Info("")
	logger.Info("Current /dev/uinput permissions:")
	
	cmd := exec.Command("ls", "-la", "/dev/uinput")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check permissions: %w", err)
	}

	logger.Info(string(output))
	return nil
}

func createSecureSetup() error {
	logger.Info("")
	logger.Info("Setting up secure uinput access...")
	
	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Check if waymon group exists, create if not
	if err := ensureWaymonGroup(); err != nil {
		return fmt.Errorf("failed to setup waymon group: %w", err)
	}

	// Add current user to waymon group
	logger.Infof("Adding %s to waymon group...", currentUser.Username)
	cmd := exec.Command("sudo", "usermod", "-a", "-G", "waymon", currentUser.Username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add user to waymon group: %w", err)
	}

	// Create secure udev rule - only waymon group can access
	logger.Info("Creating secure udev rule...")
	rule := `KERNEL=="uinput", GROUP="waymon", MODE="0660", TAG+="uaccess"`
	
	// Create the udev rule file
	cmd = exec.Command("sudo", "tee", "/etc/udev/rules.d/99-waymon-uinput.rules")
	cmd.Stdin = strings.NewReader(rule)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create udev rule: %w", err)
	}

	// Reload udev rules
	cmd = exec.Command("sudo", "udevadm", "control", "--reload-rules")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload udev rules: %w", err)
	}

	// Trigger udev
	cmd = exec.Command("sudo", "udevadm", "trigger")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to trigger udev: %w", err)
	}

	logger.Info("✓ Secure udev rule created at /etc/udev/rules.d/99-waymon-uinput.rules")
	logger.Infof("✓ User %s added to waymon group", currentUser.Username)
	logger.Info("")
	logger.Info("IMPORTANT: You must log out and back in for the group changes to take effect!")
	return nil
}

func ensureWaymonGroup() error {
	// Check if waymon group exists
	cmd := exec.Command("getent", "group", "waymon")
	if err := cmd.Run(); err == nil {
		logger.Info("✓ waymon group already exists")
		return nil
	}

	// Create waymon group
	logger.Info("Creating waymon group...")
	cmd = exec.Command("sudo", "groupadd", "waymon")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create waymon group: %w", err)
	}

	logger.Info("✓ waymon group created")
	return nil
}

func testUinputAccess() error {
	logger.Info("")
	logger.Info("Testing access...")

	// Try to open /dev/uinput for writing
	file, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err != nil {
		if os.IsPermission(err) {
			logger.Error("✗ No write access to /dev/uinput")
			logger.Info("")
			logger.Info("You may need to log out and back in for group changes to take effect")
			return nil
		}
		return fmt.Errorf("failed to test access: %w", err)
	}
	file.Close()

	logger.Info("✓ You have write access to /dev/uinput")
	logger.Info("")
	logger.Info("Setup complete! You can now run: waymon server or waymon client")
	return nil
}

// VerifyUinputSetup checks if uinput has been properly configured for Waymon
func VerifyUinputSetup() error {
	// Check if uinput module is loaded
	cmd := exec.Command("lsmod")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("uinput module check failed - please run 'waymon setup'")
	}

	if !strings.Contains(string(output), "uinput") {
		return fmt.Errorf("uinput module not loaded - please run 'waymon setup'")
	}

	// Check if /dev/uinput exists
	if _, err := os.Stat("/dev/uinput"); os.IsNotExist(err) {
		return fmt.Errorf("/dev/uinput not found - please run 'waymon setup'")
	}

	// Check if waymon group exists
	cmd = exec.Command("getent", "group", "waymon")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("waymon group not found - please run 'waymon setup'")
	}

	// Check if current user is in waymon group
	// When running with sudo, check the actual user, not root
	username := os.Getenv("SUDO_USER")
	if username == "" {
		currentUser, err := user.Current()
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}
		username = currentUser.Username
	}

	cmd = exec.Command("groups", username)
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check user groups: %w", err)
	}

	if !strings.Contains(string(output), "waymon") {
		return fmt.Errorf("user %s is not in waymon group - please run 'waymon setup' and log out/in", username)
	}

	// Check if udev rule exists
	if _, err := os.Stat("/etc/udev/rules.d/99-waymon-uinput.rules"); os.IsNotExist(err) {
		return fmt.Errorf("waymon udev rule not found - please run 'waymon setup'")
	}

	// Test actual access
	file, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("no write access to /dev/uinput - you may need to log out and back in after running 'waymon setup'")
		}
		return fmt.Errorf("failed to access /dev/uinput: %w", err)
	}
	file.Close()

	return nil
}