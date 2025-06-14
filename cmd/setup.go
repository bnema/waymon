package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/bnema/waymon/internal/logger"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup permissions for Waymon server and client modes",
	Long: `Setup permissions for both Waymon server and client modes.
This command:
- Creates a dedicated 'waymon' group for uinput access (server mode)
- Adds the user to the 'input' group for mouse capture (client mode)
- Configures udev rules for secure uinput access`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	logger.Info("Waymon Permissions Setup")
	logger.Info("========================")
	logger.Info("")

	// Check if running as root
	if os.Geteuid() == 0 {
		logger.Info("Please run this command as a normal user (not root)")
		logger.Info("The setup will use sudo when needed")
		return fmt.Errorf("cannot run setup as root")
	}

	// Ask user what they want to set up
	setupMode, err := askSetupMode()
	if err != nil {
		return err
	}

	// Setup based on user choice
	switch setupMode {
	case "server":
		return setupServerMode()
	case "client":
		return setupClientMode()
	case "both":
		return setupBothModes()
	default:
		return fmt.Errorf("invalid setup mode: %s", setupMode)
	}
}

func askSetupMode() (string, error) {
	var setupMode string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What do you want to set up?").
				Description("Choose the Waymon mode you'll be using on this computer").
				Options(
					huh.NewOption("Server mode - Receive and inject mouse events", "server"),
					huh.NewOption("Client mode - Capture and send mouse events", "client"),
					huh.NewOption("Both modes - Set up for server and client usage", "both"),
				).
				Value(&setupMode),
		),
	)

	if err := form.Run(); err != nil {
		return "", fmt.Errorf("setup cancelled: %w", err)
	}

	return setupMode, nil
}

func setupServerMode() error {
	logger.Info("")
	logger.Info("Setting up SERVER mode permissions...")
	logger.Info("")

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

	// Create secure udev rule setup (server mode)
	if err := createSecureSetup(); err != nil {
		return err
	}

	// Test uinput access
	return testUinputAccess()
}

func setupClientMode() error {
	logger.Info("")
	logger.Info("Setting up CLIENT mode permissions...")
	logger.Info("")

	// Setup input capture permissions (client mode)
	if err := setupInputCapture(); err != nil {
		return err
	}

	// Test input access
	return testInputAccess()
}

func setupBothModes() error {
	logger.Info("")
	logger.Info("Setting up BOTH server and client mode permissions...")
	logger.Info("")

	// Server setup
	if err := checkAndLoadUinput(); err != nil {
		return err
	}
	if err := checkUinputDevice(); err != nil {
		return err
	}
	if err := showCurrentPermissions(); err != nil {
		return err
	}
	if err := createSecureSetup(); err != nil {
		return err
	}

	// Client setup
	if err := setupInputCapture(); err != nil {
		return err
	}

	// Test both
	return testBothAccess()
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

func setupInputCapture() error {
	logger.Info("")
	logger.Info("Setting up input capture permissions (client mode)...")
	
	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Check if user is already in input group
	cmd := exec.Command("groups", currentUser.Username)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check user groups: %w", err)
	}

	if strings.Contains(string(output), "input") {
		logger.Infof("✓ User %s is already in input group", currentUser.Username)
	} else {
		// Add current user to input group
		logger.Infof("Adding %s to input group for mouse capture...", currentUser.Username)
		cmd = exec.Command("sudo", "usermod", "-a", "-G", "input", currentUser.Username)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to add user to input group: %w", err)
		}
		logger.Infof("✓ User %s added to input group", currentUser.Username)
	}

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
	logger.Info("Testing uinput access...")

	file, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err != nil {
		if os.IsPermission(err) {
			logger.Error("✗ No write access to /dev/uinput")
			logger.Info("")
			logger.Info("IMPORTANT: You must log out and back in for the group changes to take effect!")
			logger.Info("After logging back in, server mode should work.")
			return nil
		}
		return fmt.Errorf("failed to test uinput access: %w", err)
	}
	file.Close()

	logger.Info("✓ You have write access to /dev/uinput")
	logger.Info("")
	logger.Info("Setup complete! You can now run: waymon server")
	return nil
}

func testInputAccess() error {
	logger.Info("")
	logger.Info("Testing input capture access...")

	// Try to open a common input device (just test one)
	inputDevices := []string{"/dev/input/event0", "/dev/input/event1", "/dev/input/event2"}
	var testDevice string
	for _, device := range inputDevices {
		if _, err := os.Stat(device); err == nil {
			testDevice = device
			break
		}
	}

	if testDevice == "" {
		logger.Info("No input devices found to test - this might be normal")
		logger.Info("Setup complete! You can now run: waymon client")
		return nil
	}

	file, err := os.OpenFile(testDevice, os.O_RDONLY, 0)
	if err != nil {
		if os.IsPermission(err) {
			logger.Error("✗ No read access to input devices")
			logger.Info("")
			logger.Info("IMPORTANT: You must log out and back in for the group changes to take effect!")
			logger.Info("After logging back in, client mode should work.")
			return nil
		}
		return fmt.Errorf("failed to test input access: %w", err)
	}
	file.Close()

	logger.Info("✓ You have read access to input devices")
	logger.Info("")
	logger.Info("Setup complete! You can now run: waymon client")
	return nil
}

func testBothAccess() error {
	logger.Info("")
	logger.Info("Testing both server and client access...")

	// Test uinput access (server mode)
	uinputOk := true
	file, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err != nil {
		if os.IsPermission(err) {
			logger.Error("✗ No write access to /dev/uinput (server mode)")
			uinputOk = false
		} else {
			return fmt.Errorf("failed to test uinput access: %w", err)
		}
	} else {
		file.Close()
		logger.Info("✓ You have write access to /dev/uinput (server mode)")
	}

	// Test input capture access (client mode)
	inputOk := true
	inputDevices := []string{"/dev/input/event0", "/dev/input/event1", "/dev/input/event2"}
	var testDevice string
	for _, device := range inputDevices {
		if _, err := os.Stat(device); err == nil {
			testDevice = device
			break
		}
	}

	if testDevice != "" {
		file, err := os.OpenFile(testDevice, os.O_RDONLY, 0)
		if err != nil {
			if os.IsPermission(err) {
				logger.Error("✗ No read access to input devices (client mode)")
				inputOk = false
			}
		} else {
			file.Close()
			logger.Info("✓ You have read access to input devices (client mode)")
		}
	}

	logger.Info("")
	if !uinputOk || !inputOk {
		logger.Info("IMPORTANT: You must log out and back in for the group changes to take effect!")
		logger.Info("After logging back in, both server and client modes should work.")
	} else {
		logger.Info("Setup complete! You can now run:")
		logger.Info("  waymon server  (for server mode)")
		logger.Info("  waymon client  (for client mode)")
	}
	
	return nil
}

// CheckUinputAvailable checks if uinput is available but doesn't fail if no access
func CheckUinputAvailable() error {
	// Check if uinput module is loaded
	cmd := exec.Command("lsmod")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("uinput module check failed")
	}

	if !strings.Contains(string(output), "uinput") {
		return fmt.Errorf("uinput module not loaded")
	}

	// Check if /dev/uinput exists
	if _, err := os.Stat("/dev/uinput"); os.IsNotExist(err) {
		return fmt.Errorf("/dev/uinput not found")
	}

	// Try to test access (but don't fail if no permission)
	file, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("no write access to /dev/uinput (will use sudo when needed)")
		}
		return fmt.Errorf("failed to check /dev/uinput: %w", err)
	}
	file.Close()

	return nil
}

// VerifyWaymonSetup checks if Waymon has been properly configured for both server and client modes
func VerifyWaymonSetup() error {
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

	// Check if current user is in waymon group (for server mode)
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

	hasWaymonGroup := strings.Contains(string(output), "waymon")
	hasInputGroup := strings.Contains(string(output), "input")

	// For now, we require at least one of the groups (could be server-only or client-only setup)
	// In the future, we might want to be more specific based on the intended use
	if !hasWaymonGroup && !hasInputGroup {
		return fmt.Errorf("user %s is not in waymon or input groups - please run 'waymon setup' and log out/in", username)
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