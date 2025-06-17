package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/bnema/waymon/internal/client"
	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/network"
	"github.com/bnema/waymon/internal/protocol"
	"github.com/bnema/waymon/internal/server"
	"github.com/charmbracelet/lipgloss"
)

var (
	verbose = flag.Bool("v", false, "verbose output")
	port    = flag.Int("port", 52530, "SSH port for test server")
	timeout = flag.Duration("timeout", 30*time.Second, "test timeout")

	// Lipgloss styles
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).MarginTop(1).MarginBottom(1)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	testStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	dimStyle     = lipgloss.NewStyle().Faint(true)
)

type TestResult struct {
	Name    string
	Passed  bool
	Message string
	Events  int
}

func main() {
	flag.Parse()

	// Suppress logs unless verbose
	if !*verbose {
		os.Setenv("LOG_LEVEL", "FATAL")
	}

	fmt.Println(titleStyle.Render("=== Waymon End-to-End Integration Test ==="))
	fmt.Println(infoStyle.Render("Testing complete flow: Capture → Network → Injection"))
	fmt.Println()

	// Create temp directory for test keys
	tempDir, err := os.MkdirTemp("", "waymon-e2e-test-*")
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to create temp dir: %v", err)))
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	result := testEndToEndFlow(tempDir)
	
	// Print summary
	fmt.Println("\n" + titleStyle.Render("=== Test Summary ==="))
	if result.Passed {
		fmt.Println(successStyle.Render(fmt.Sprintf("✓ %s: %s", result.Name, result.Message)))
		if result.Events > 0 {
			fmt.Println(dimStyle.Render(fmt.Sprintf("  Events processed: %d", result.Events)))
		}
	} else {
		fmt.Println(errorStyle.Render(fmt.Sprintf("✗ %s: %s", result.Name, result.Message)))
		os.Exit(1)
	}
}

func testEndToEndFlow(tempDir string) TestResult {
	fmt.Println(testStyle.Render("[Test: End-to-End Event Flow]"))
	fmt.Println(infoStyle.Render("This test simulates the complete KVM flow"))

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Generate test SSH keys
	hostKeyPath := tempDir + "/host_key"
	clientKeyPath := tempDir + "/client_key"
	authKeysPath := tempDir + "/authorized_keys"
	
	if err := generateTestKeys(hostKeyPath, clientKeyPath, authKeysPath); err != nil {
		return TestResult{
			Name:    "End-to-End Flow",
			Passed:  false,
			Message: fmt.Sprintf("Failed to generate test keys: %v", err),
		}
	}

	// Step 1: Create and start SSH server
	fmt.Println(dimStyle.Render("1. Starting SSH server..."))
	sshServer := network.NewSSHServer(*port, hostKeyPath, authKeysPath)
	sshServer.SetAuthHandlers(func(addr, publicKey, fingerprint string) bool {
		return true // Accept all for testing
	})

	// Step 2: Create server-side components
	fmt.Println(dimStyle.Render("2. Creating server capture backend..."))
	captureBackend, err := input.CreateServerBackend()
	if err != nil {
		return TestResult{
			Name:    "End-to-End Flow",
			Passed:  false,
			Message: fmt.Sprintf("Failed to create capture backend: %v", err),
		}
	}

	// Create server manager
	serverManager, err := server.NewClientManager()
	if err != nil {
		return TestResult{
			Name:    "End-to-End Flow",
			Passed:  false,
			Message: fmt.Sprintf("Failed to create server manager: %v", err),
		}
	}

	// Track events at each stage
	var capturedEvents atomic.Int64
	var networkEvents atomic.Int64
	var injectedEvents atomic.Int64

	// Wire up capture backend to send events over network
	captureBackend.OnInputEvent(func(event *protocol.InputEvent) {
		capturedEvents.Add(1)
		if *verbose {
			fmt.Printf("[CAPTURE] Event: %T\n", event.Event)
		}
		
		// Server manager would normally handle this
		// For testing, we'll forward directly to connected clients
		if serverManager.IsControllingLocal() {
			// Not forwarding when controlling local
			return
		}
		
		// Get active client
		if activeClient := serverManager.GetActiveClient(); activeClient != nil {
			// In real implementation, this would send via SSH to the client
			networkEvents.Add(1)
		}
	})

	// Wire up SSH server events to server manager
	sshServer.OnInputEvent = func(event *protocol.InputEvent) {
		// This is called when server receives events from clients
		serverManager.HandleInputEvent(event)
	}

	// Start SSH server
	go func() {
		if err := sshServer.Start(ctx); err != nil && ctx.Err() == nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("SSH server error: %v", err)))
		}
	}()
	time.Sleep(200 * time.Millisecond)

	// Step 3: Create client-side components
	fmt.Println(dimStyle.Render("3. Creating client injection backend..."))
	
	// Create input receiver (client)
	inputReceiver, err := client.NewInputReceiver(fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		return TestResult{
			Name:    "End-to-End Flow",
			Passed:  false,
			Message: fmt.Sprintf("Failed to create input receiver: %v", err),
		}
	}

	// Connect client
	fmt.Println(dimStyle.Render("4. Connecting client to server..."))
	if err := inputReceiver.Connect(ctx, clientKeyPath); err != nil {
		return TestResult{
			Name:    "End-to-End Flow",
			Passed:  false,
			Message: fmt.Sprintf("Failed to connect client: %v", err),
		}
	}
	defer inputReceiver.Disconnect()

	// Wait for connection to establish
	time.Sleep(500 * time.Millisecond)

	// Step 4: Start capture backend
	fmt.Println(dimStyle.Render("5. Starting event capture..."))
	if err := captureBackend.Start(ctx); err != nil {
		return TestResult{
			Name:    "End-to-End Flow",
			Passed:  false,
			Message: fmt.Sprintf("Failed to start capture backend: %v", err),
		}
	}
	defer captureBackend.Stop()

	// Step 5: Switch server to control the connected client
	fmt.Println(dimStyle.Render("6. Switching server to control client..."))
	clients := serverManager.GetConnectedClients()
	if len(clients) == 0 {
		return TestResult{
			Name:    "End-to-End Flow",
			Passed:  false,
			Message: "No clients connected to server",
		}
	}

	if err := serverManager.SwitchToClient(clients[0].ID); err != nil {
		return TestResult{
			Name:    "End-to-End Flow",
			Passed:  false,
			Message: fmt.Sprintf("Failed to switch to client: %v", err),
		}
	}

	// Step 6: Generate test events
	fmt.Println(dimStyle.Render("7. Generating test events..."))
	fmt.Println(infoStyle.Render("   Move your mouse and press some keys..."))
	
	// Wait for events to flow through the system
	testDuration := 5 * time.Second
	fmt.Printf("   Capturing events for %v...\n", testDuration)
	time.Sleep(testDuration)

	// Step 7: Verify results
	captured := capturedEvents.Load()
	network := networkEvents.Load()
	injected := injectedEvents.Load()

	fmt.Println(dimStyle.Render("\n8. Results:"))
	fmt.Printf("   - Events captured: %d\n", captured)
	fmt.Printf("   - Events sent over network: %d\n", network)
	fmt.Printf("   - Events injected on client: %d\n", injected)

	// Determine success
	if captured == 0 {
		return TestResult{
			Name:    "End-to-End Flow",
			Passed:  false,
			Message: "No events captured - make sure you have access to /dev/input devices",
		}
	}

	if network == 0 {
		return TestResult{
			Name:    "End-to-End Flow",
			Passed:  false,
			Message: fmt.Sprintf("Events captured (%d) but none sent over network", captured),
		}
	}

	if injected == 0 {
		return TestResult{
			Name:    "End-to-End Flow",
			Passed:  false,
			Message: fmt.Sprintf("Events sent over network (%d) but none injected on client", network),
		}
	}

	// Success!
	successRate := float64(injected) / float64(captured) * 100
	return TestResult{
		Name:    "End-to-End Flow",
		Passed:  true,
		Message: fmt.Sprintf("%.1f%% of captured events successfully injected (%d/%d)", 
			successRate, injected, captured),
		Events:  int(captured),
	}
}

// generateTestKeys creates test SSH keys
func generateTestKeys(hostKeyPath, clientKeyPath, authKeysPath string) error {
	// Use the helper from network test
	if err := network.GenerateTestKeys(hostKeyPath, clientKeyPath, authKeysPath); err != nil {
		return err
	}
	return nil
}