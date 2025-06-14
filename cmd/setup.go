package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/bnema/waymon/internal/ui"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

type setupResult struct {
	step    string
	success bool
	message string
	action  string
}

type setupPhase struct {
	name    string
	results []setupResult
}

func (p *setupPhase) addResult(step string, success bool, message string, action ...string) {
	actionStr := ""
	if len(action) > 0 {
		actionStr = action[0]
	}
	p.results = append(p.results, setupResult{
		step:    step,
		success: success,
		message: message,
		action:  actionStr,
	})
}

func printPhase(phase setupPhase) {
	fmt.Print(ui.FormatSetupPhase(phase.name))
	for _, result := range phase.results {
		fmt.Println(ui.FormatSetupResult(result.success, result.step, result.message))
	}
	fmt.Println()
}

func printSummary(phases []setupPhase, needsRelogin bool) {
	fmt.Println(ui.FormatSummaryHeader("Setup Summary"))
	allSuccess := true
	var actions []string
	
	for _, phase := range phases {
		for _, result := range phase.results {
			if !result.success {
				allSuccess = false
			}
			if result.action != "" {
				actions = append(actions, result.action)
			}
		}
	}
	
	fmt.Print(ui.FormatSummaryStatus(allSuccess, needsRelogin))
	
	if allSuccess && !needsRelogin {
		fmt.Println("   You can now run Waymon in the configured modes.")
	} else if needsRelogin {
		fmt.Println("   Please log out and back in for group changes to take effect.")
	} else {
		fmt.Println("   Please review the results above and try again if needed.")
	}
	
	if len(actions) > 0 {
		fmt.Print(ui.FormatNextStepsHeader())
		for i, action := range actions {
			fmt.Println(ui.FormatActionItem(i+1, action))
		}
	}
}

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

var (
	setupModeFlag string
)

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().StringVarP(&setupModeFlag, "mode", "m", "", "Setup mode: server, client, or both (skips interactive prompt)")
}

func runSetup(cmd *cobra.Command, args []string) error {
	fmt.Println(ui.FormatSetupHeader("Waymon Setup"))
	// Check if running as root
	if os.Geteuid() == 0 {
		fmt.Println(ui.ErrorStyle.Render(ui.IconError + " Please run this command as a normal user (not root)"))
		fmt.Println("   The setup will use sudo when needed")
		return fmt.Errorf("cannot run setup as root")
	}

	// Explain sudo usage upfront
	fmt.Println(ui.InfoStyle.Render("This setup requires sudo permissions to:"))
	fmt.Println("   • Load the uinput kernel module")
	fmt.Println("   • Create system groups (waymon)")
	fmt.Println("   • Add your user to system groups (waymon, input)")
	fmt.Println("   • Create udev rules for device access")
	fmt.Println("   • Reload system configuration")
	fmt.Println()

	// Determine setup mode (from flag or interactive prompt)
	var setupMode string
	var err error
	
	if setupModeFlag != "" {
		setupMode = setupModeFlag
		// Validate the flag value
		if setupMode != "server" && setupMode != "client" && setupMode != "both" {
			return fmt.Errorf("invalid setup mode: %s (must be server, client, or both)", setupMode)
		}
	} else {
		setupMode, err = askSetupMode()
		if err != nil {
			return err
		}
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
	// Perform server setup and collect results
	serverPhase, serverErr := performServerSetup()
	if serverErr != nil {
		printPhase(serverPhase)
		return serverErr
	}
	printPhase(serverPhase)

	// Test uinput access
	testPhase := setupPhase{name: "Access Testing"}
	err := testUinputAccess()
	testPhase.addResult("Server mode access", err == nil, getErrorMessage(err))
	printPhase(testPhase)

	needsRelogin := err != nil && strings.Contains(err.Error(), "relogin")
	printSummary([]setupPhase{serverPhase, testPhase}, needsRelogin)

	return nil
}

func setupClientMode() error {
	// Perform client setup and collect results
	clientPhase, clientErr := performClientSetup()
	if clientErr != nil {
		printPhase(clientPhase)
		return clientErr
	}
	printPhase(clientPhase)

	// Test input access
	testPhase := setupPhase{name: "Access Testing"}
	err := testInputAccess()
	testPhase.addResult("Client mode access", err == nil, getErrorMessage(err))
	printPhase(testPhase)

	needsRelogin := err != nil && strings.Contains(err.Error(), "relogin")
	printSummary([]setupPhase{clientPhase, testPhase}, needsRelogin)

	return nil
}

func setupBothModes() error {
	var phases []setupPhase
	needsRelogin := false

	// Perform server setup and collect results
	serverPhase, serverErr := performServerSetup()
	if serverErr != nil {
		phases = append(phases, serverPhase)
		printPhase(serverPhase)
		return serverErr
	}
	phases = append(phases, serverPhase)
	printPhase(serverPhase)

	// Perform client setup and collect results
	clientPhase, clientErr := performClientSetup()
	if clientErr != nil {
		phases = append(phases, clientPhase)
		printPhase(clientPhase)
		return clientErr
	}
	phases = append(phases, clientPhase)
	printPhase(clientPhase)

	// Testing phase
	testPhase := setupPhase{name: "Access Testing"}
	
	uinputOk, inputOk := testBothAccessStructured()
	testPhase.addResult("Server mode access", uinputOk, "")
	testPhase.addResult("Client mode access", inputOk, "")
	
	if !uinputOk || !inputOk {
		needsRelogin = true
	}

	phases = append(phases, testPhase)
	printPhase(testPhase)

	// Print summary
	printSummary(phases, needsRelogin)
	
	return nil
}

func getErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func checkAndLoadUinput() error {
	// Check if uinput module is loaded
	cmd := exec.Command("lsmod")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check loaded modules: %w", err)
	}

	if strings.Contains(string(output), "uinput") {
		return nil
	}

	cmd = exec.Command("sudo", "modprobe", "uinput")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to load uinput module: %w", err)
	}
	return nil
}

func checkUinputDevice() error {
	// Check if /dev/uinput exists
	info, err := os.Stat("/dev/uinput")
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("/dev/uinput not found")
		}
		return fmt.Errorf("failed to check /dev/uinput: %w", err)
	}

	// Check if it's a character device
	if info.Mode()&os.ModeCharDevice == 0 {
		return fmt.Errorf("/dev/uinput is not a character device")
	}
	return nil
}

func showCurrentPermissions() error {
	// Silently check permissions - we don't need to display them in structured output
	return nil
}

func setupInputCapture() error {
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

	if !strings.Contains(string(output), "input") {
		// Add current user to input group
		cmd = exec.Command("sudo", "usermod", "-a", "-G", "input", currentUser.Username)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to add user to input group: %w", err)
		}
	}

	return nil
}

func createSecureSetup() error {
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
	cmd := exec.Command("sudo", "usermod", "-a", "-G", "waymon", currentUser.Username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add user to waymon group: %w", err)
	}

	// Create secure udev rule - only waymon group can access
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

	// Rules created and user added successfully
	return nil
}

func ensureWaymonGroup() error {
	// Check if waymon group exists
	cmd := exec.Command("getent", "group", "waymon")
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Create waymon group
	cmd = exec.Command("sudo", "groupadd", "waymon")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create waymon group: %w", err)
	}
	return nil
}

func testUinputAccess() error {
	file, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("no write access to /dev/uinput - relogin required")
		}
		return fmt.Errorf("failed to test uinput access: %w", err)
	}
	file.Close()
	return nil
}

func testInputAccess() error {
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
		return nil
	}

	file, err := os.OpenFile(testDevice, os.O_RDONLY, 0)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("no read access to input devices - relogin required")
		}
		return fmt.Errorf("failed to test input access: %w", err)
	}
	file.Close()
	return nil
}

func testBothAccess() error {
	// This function is replaced by testBothAccessStructured
	// which returns the test results for structured output
	return nil
}

func testBothAccessStructured() (bool, bool) {
	// Test uinput access (server mode)
	uinputOk := true
	file, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err != nil {
		if os.IsPermission(err) {
			uinputOk = false
		}
	} else {
		file.Close()
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
				inputOk = false
			}
		} else {
			file.Close()
		}
	}

	return uinputOk, inputOk
}

// performServerSetup performs the server setup steps and returns the phase with results
func performServerSetup() (setupPhase, error) {
	phase := setupPhase{name: "Server Mode Setup"}

	// Check if uinput module is loaded
	err := checkAndLoadUinput()
	phase.addResult("Load uinput module", err == nil, getErrorMessage(err))
	if err != nil {
		return phase, err
	}

	// Check if /dev/uinput exists
	err = checkUinputDevice()
	phase.addResult("Verify /dev/uinput device", err == nil, getErrorMessage(err))
	if err != nil {
		return phase, err
	}

	// Create secure udev rule setup (server mode)
	err = createSecureSetup()
	phase.addResult("Configure uinput permissions", err == nil, getErrorMessage(err))
	if err != nil {
		return phase, err
	}

	return phase, nil
}

// performClientSetup performs the client setup steps and returns the phase with results
func performClientSetup() (setupPhase, error) {
	phase := setupPhase{name: "Client Mode Setup"}

	// Setup input capture permissions (client mode)
	err := setupInputCapture()
	phase.addResult("Configure input capture", err == nil, getErrorMessage(err))
	if err != nil {
		return phase, err
	}

	return phase, nil
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

// VerifyClientSetup checks if Waymon has been properly configured for client mode
func VerifyClientSetup() error {
	// Check if current user is in input group
	username := os.Getenv("SUDO_USER")
	if username == "" {
		currentUser, err := user.Current()
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}
		username = currentUser.Username
	}

	cmd := exec.Command("groups", username)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check user groups: %w", err)
	}

	if !strings.Contains(string(output), "input") {
		return fmt.Errorf("user %s is not in input group - please run 'waymon setup client' and log out/in", username)
	}

	// For client mode, we only need input group access
	// No need to check uinput module or /dev/uinput access
	return nil
}