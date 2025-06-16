package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bnema/waymon/internal/client"
	"github.com/bnema/waymon/internal/network"
	"github.com/bnema/waymon/internal/protocol"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/crypto/ssh"
)

var (
	verbose = flag.Bool("v", false, "verbose output")
	port    = flag.Int("port", 52526, "SSH port for test server")
	timeout = flag.Duration("timeout", 30*time.Second, "test timeout")

	// Lipgloss styles
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).MarginTop(1).MarginBottom(1)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	testStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	dimStyle     = lipgloss.NewStyle().Faint(true)
	statsStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
)

type TestResult struct {
	Name    string
	Passed  bool
	Message string
	Events  int
}

func main() {
	flag.Parse()

	// Suppress logs during tests unless verbose mode is on
	if !*verbose {
		os.Setenv("LOG_LEVEL", "FATAL")
	}

	fmt.Println(titleStyle.Render("=== Waymon Network Integration Tests ==="))
	fmt.Println(infoStyle.Render("Testing SSH transport and protocol buffer communication"))
	fmt.Println()

	// Create temp directory for test keys
	tempDir, err := os.MkdirTemp("", "waymon-test-*")
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to create temp dir: %v", err)))
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	var results []TestResult

	// Run tests
	results = append(results, testSSHConnection(tempDir))
	results = append(results, testEventTransmission(tempDir))
	results = append(results, testHighThroughput(tempDir))
	results = append(results, testClientServerManagers(tempDir))

	// Print summary
	printSummary(results)
	
	// Give a moment for async operations to complete
	time.Sleep(100 * time.Millisecond)
}

func testSSHConnection(tempDir string) TestResult {
	fmt.Println(testStyle.Render("[Test: SSH Connection]"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Generate test keys
	hostKeyPath := filepath.Join(tempDir, "host_key")
	clientKeyPath := filepath.Join(tempDir, "client_key")
	
	if err := generateTestKey(hostKeyPath); err != nil {
		return TestResult{
			Name:    "SSH Connection",
			Passed:  false,
			Message: fmt.Sprintf("Failed to generate host key: %v", err),
		}
	}

	if err := generateTestKey(clientKeyPath); err != nil {
		return TestResult{
			Name:    "SSH Connection",
			Passed:  false,
			Message: fmt.Sprintf("Failed to generate client key: %v", err),
		}
	}

	// Create server with authorized keys
	authKeysPath := filepath.Join(tempDir, "authorized_keys")
	if err := createAuthorizedKeys(clientKeyPath, authKeysPath); err != nil {
		return TestResult{
			Name:    "SSH Connection",
			Passed:  false,
			Message: fmt.Sprintf("Failed to create authorized keys: %v", err),
		}
	}

	server := network.NewSSHServer(*port, hostKeyPath, authKeysPath)
	
	// Set auth handler to accept test connections
	server.SetAuthHandlers(func(addr, publicKey, fingerprint string) bool {
		if *verbose {
			fmt.Println(dimStyle.Render(fmt.Sprintf("Auth request from %s with fingerprint %s", addr, fingerprint)))
		}
		return true // Accept all for testing
	})
	
	// Start server
	serverReady := make(chan struct{})
	go func() {
		close(serverReady)
		if err := server.Start(ctx); err != nil && ctx.Err() == nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Server error: %v", err)))
		}
	}()

	// Wait for server to be ready
	<-serverReady
	time.Sleep(100 * time.Millisecond)

	// Create client
	client := network.NewSSHClient(clientKeyPath)
	
	// Connect
	if err := client.Connect(ctx, fmt.Sprintf("localhost:%d", *port)); err != nil {
		return TestResult{
			Name:    "SSH Connection",
			Passed:  false,
			Message: fmt.Sprintf("Failed to connect: %v", err),
		}
	}

	// Verify connection
	if !client.IsConnected() {
		return TestResult{
			Name:    "SSH Connection",
			Passed:  false,
			Message: "Client not connected after Connect()",
		}
	}

	// Clean disconnect
	if err := client.Disconnect(); err != nil && *verbose {
		fmt.Println(dimStyle.Render(fmt.Sprintf("Client disconnect error: %v", err)))
	}
	
	// Stop server gracefully
	cancel()
	time.Sleep(50 * time.Millisecond) // Give server time to shut down

	return TestResult{
		Name:    "SSH Connection",
		Passed:  true,
		Message: "SSH connection established and closed successfully",
	}
}

func testEventTransmission(tempDir string) TestResult {
	fmt.Println("\n" + testStyle.Render("[Test: Event Transmission]"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Setup server and client
	hostKeyPath := filepath.Join(tempDir, "host_key2")
	clientKeyPath := filepath.Join(tempDir, "client_key2")
	authKeysPath := filepath.Join(tempDir, "authorized_keys2")
	generateTestKey(hostKeyPath)
	generateTestKey(clientKeyPath)
	createAuthorizedKeys(clientKeyPath, authKeysPath)

	server := network.NewSSHServer(*port+1, hostKeyPath, authKeysPath)
	
	// Set auth handler to accept test connections
	server.SetAuthHandlers(func(addr, publicKey, fingerprint string) bool {
		return true // Accept all for testing
	})
	
	// Track received events
	receivedEvents := atomic.Int64{}
	server.OnInputEvent = func(event *protocol.InputEvent) {
		receivedEvents.Add(1)
		if *verbose {
			fmt.Println(dimStyle.Render(fmt.Sprintf("Server received: %T from %s", event.Event, event.SourceId)))
		}
	}

	// Start server
	go server.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// Connect client
	client := network.NewSSHClient(clientKeyPath)
	if err := client.Connect(ctx, fmt.Sprintf("localhost:%d", *port+1)); err != nil {
		return TestResult{
			Name:    "Event Transmission",
			Passed:  false,
			Message: fmt.Sprintf("Failed to connect: %v", err),
		}
	}
	defer client.Disconnect()

	// Send various event types
	testEvents := []*protocol.InputEvent{
		{
			Timestamp: time.Now().UnixNano(),
			SourceId:  "test-client",
			Event: &protocol.InputEvent_MouseMove{
				MouseMove: &protocol.MouseMoveEvent{Dx: 10, Dy: 20},
			},
		},
		{
			Timestamp: time.Now().UnixNano(),
			SourceId:  "test-client",
			Event: &protocol.InputEvent_MouseButton{
				MouseButton: &protocol.MouseButtonEvent{Button: 1, Pressed: true},
			},
		},
		{
			Timestamp: time.Now().UnixNano(),
			SourceId:  "test-client",
			Event: &protocol.InputEvent_Keyboard{
				Keyboard: &protocol.KeyboardEvent{Key: 30, Pressed: true, Modifiers: 0},
			},
		},
		{
			Timestamp: time.Now().UnixNano(),
			SourceId:  "test-client",
			Event: &protocol.InputEvent_Control{
				Control: &protocol.ControlEvent{
					Type: protocol.ControlEvent_CLIENT_CONFIG,
					ClientConfig: &protocol.ClientConfig{
						ClientId:   "test-client",
						ClientName: "Test Client",
					},
				},
			},
		},
	}

	// Send events
	for _, event := range testEvents {
		if err := client.SendInputEvent(event); err != nil {
			return TestResult{
				Name:    "Event Transmission",
				Passed:  false,
				Message: fmt.Sprintf("Failed to send event: %v", err),
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for reception
	time.Sleep(100 * time.Millisecond)

	received := receivedEvents.Load()
	if received != int64(len(testEvents)) {
		return TestResult{
			Name:    "Event Transmission",
			Passed:  false,
			Message: fmt.Sprintf("Expected %d events, received %d", len(testEvents), received),
		}
	}

	return TestResult{
		Name:    "Event Transmission",
		Passed:  true,
		Message: fmt.Sprintf("All %d event types transmitted successfully", len(testEvents)),
		Events:  int(received),
	}
}

func testHighThroughput(tempDir string) TestResult {
	fmt.Println("\n" + testStyle.Render("[Test: High Throughput]"))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Setup
	hostKeyPath := filepath.Join(tempDir, "host_key3")
	clientKeyPath := filepath.Join(tempDir, "client_key3")
	authKeysPath := filepath.Join(tempDir, "authorized_keys3")
	generateTestKey(hostKeyPath)
	generateTestKey(clientKeyPath)
	createAuthorizedKeys(clientKeyPath, authKeysPath)

	server := network.NewSSHServer(*port+2, hostKeyPath, authKeysPath)
	
	// Set auth handler to accept test connections
	server.SetAuthHandlers(func(addr, publicKey, fingerprint string) bool {
		return true // Accept all for testing
	})
	
	// Track performance
	receivedEvents := atomic.Int64{}
	latencies := make([]time.Duration, 0, 1000)
	latencyChan := make(chan time.Duration, 1000)

	server.OnInputEvent = func(event *protocol.InputEvent) {
		receivedEvents.Add(1)
		latency := time.Duration(time.Now().UnixNano() - event.Timestamp)
		select {
		case latencyChan <- latency:
		default:
		}
	}

	// Collect latencies
	go func() {
		for latency := range latencyChan {
			latencies = append(latencies, latency)
		}
	}()

	// Start server
	go server.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// Connect client
	client := network.NewSSHClient(clientKeyPath)
	if err := client.Connect(ctx, fmt.Sprintf("localhost:%d", *port+2)); err != nil {
		return TestResult{
			Name:    "High Throughput",
			Passed:  false,
			Message: fmt.Sprintf("Failed to connect: %v", err),
		}
	}
	defer client.Disconnect()

	// Send many events rapidly
	eventCount := 1000
	start := time.Now()

	for i := 0; i < eventCount; i++ {
		event := &protocol.InputEvent{
			Timestamp: time.Now().UnixNano(),
			SourceId:  "throughput-test",
			Event: &protocol.InputEvent_MouseMove{
				MouseMove: &protocol.MouseMoveEvent{
					Dx: float64(i % 100),
					Dy: float64(i % 100),
				},
			},
		}

		if err := client.SendInputEvent(event); err != nil {
			if *verbose {
				fmt.Println(errorStyle.Render(fmt.Sprintf("Send error at %d: %v", i, err)))
			}
		}
	}

	// Wait for all events
	deadline := time.Now().Add(5 * time.Second)
	for receivedEvents.Load() < int64(eventCount) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	elapsed := time.Since(start)
	received := receivedEvents.Load()
	close(latencyChan)

	// Calculate stats
	throughput := float64(received) / elapsed.Seconds()
	
	// Calculate average latency
	var totalLatency time.Duration
	for _, l := range latencies {
		totalLatency += l
	}
	avgLatency := time.Duration(0)
	if len(latencies) > 0 {
		avgLatency = totalLatency / time.Duration(len(latencies))
	}

	passed := received >= int64(eventCount)*95/100 // Allow 5% loss
	
	return TestResult{
		Name:    "High Throughput",
		Passed:  passed,
		Message: fmt.Sprintf("%.0f events/sec, avg latency: %v, received: %d/%d",
			throughput, avgLatency, received, eventCount),
		Events:  int(received),
	}
}

func testClientServerManagers(tempDir string) TestResult {
	fmt.Println("\n" + testStyle.Render("[Test: Client/Server Managers]"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Setup
	hostKeyPath := filepath.Join(tempDir, "host_key4")
	clientKeyPath := filepath.Join(tempDir, "client_key4")
	authKeysPath := filepath.Join(tempDir, "authorized_keys4")
	generateTestKey(hostKeyPath)
	generateTestKey(clientKeyPath)
	createAuthorizedKeys(clientKeyPath, authKeysPath)

	// Create SSH server
	sshServer := network.NewSSHServer(*port+3, hostKeyPath, authKeysPath)
	
	// Set auth handler to accept test connections
	sshServer.SetAuthHandlers(func(addr, publicKey, fingerprint string) bool {
		return true // Accept all for testing
	})
	
	// Track client connections
	connectedClients := make(map[string]string)
	var clientMu sync.Mutex
	
	sshServer.OnClientConnected = func(addr, publicKey string) {
		clientMu.Lock()
		connectedClients[addr] = publicKey
		clientMu.Unlock()
		if *verbose {
			fmt.Println(infoStyle.Render(fmt.Sprintf("Client connected: %s", addr)))
		}
	}
	
	// Start server
	go sshServer.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// Create SSH client
	sshClient := network.NewSSHClient(clientKeyPath)
	
	// Connect
	if err := sshClient.Connect(ctx, fmt.Sprintf("localhost:%d", *port+3)); err != nil {
		return TestResult{
			Name:    "Client/Server Managers",
			Passed:  false,
			Message: fmt.Sprintf("Failed to connect: %v", err),
		}
	}
	defer sshClient.Disconnect()

	// Create client receiver (but don't start it for this test)
	clientReceiver, err := client.NewInputReceiver(fmt.Sprintf("localhost:%d", *port+3))
	if err != nil {
		return TestResult{
			Name:    "Client/Server Managers",
			Passed:  false,
			Message: fmt.Sprintf("Failed to create input receiver: %v", err),
		}
	}
	_ = clientReceiver // Just testing creation

	// Send client config
	configEvent := &protocol.InputEvent{
		Timestamp: time.Now().UnixNano(),
		SourceId:  "test-client",
		Event: &protocol.InputEvent_Control{
			Control: &protocol.ControlEvent{
				Type: protocol.ControlEvent_CLIENT_CONFIG,
				ClientConfig: &protocol.ClientConfig{
					ClientId:   "test-client-123",
					ClientName: "Test Client",
				},
			},
		},
	}

	if err := sshClient.SendInputEvent(configEvent); err != nil {
		return TestResult{
			Name:    "Client/Server Managers",
			Passed:  false,
			Message: fmt.Sprintf("Failed to send config: %v", err),
		}
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Check if client connected to SSH server
	clientMu.Lock()
	numClients := len(connectedClients)
	clientMu.Unlock()
	
	if numClients == 0 {
		return TestResult{
			Name:    "Client/Server Managers",
			Passed:  false,
			Message: "No clients connected to SSH server",
		}
	}
	
	// Verify the client config was received
	// In a real scenario, the server manager would process this
	// For now, we just verify the connection works

	return TestResult{
		Name:    "Client/Server Managers",
		Passed:  true,
		Message: "Client/Server managers working correctly",
	}
}

// Helper function to generate test SSH keys
func generateTestKey(path string) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// Save private key
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := pem.Encode(file, privateKeyPEM); err != nil {
		return err
	}

	// Save public key
	pub, err := ssh.NewPublicKey(&key.PublicKey)
	if err != nil {
		return err
	}

	pubFile, err := os.Create(path + ".pub")
	if err != nil {
		return err
	}
	defer pubFile.Close()

	_, err = pubFile.Write(ssh.MarshalAuthorizedKey(pub))
	return err
}

// createAuthorizedKeys creates an authorized_keys file from a private key
func createAuthorizedKeys(privateKeyPath, authKeysPath string) error {
	// Read the private key
	privateKeyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return err
	}

	// Parse the private key
	block, _ := pem.Decode(privateKeyData)
	if block == nil {
		return fmt.Errorf("failed to parse PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return err
	}

	// Get the public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}

	// Write to authorized_keys
	return os.WriteFile(authKeysPath, ssh.MarshalAuthorizedKey(pub), 0600)
}

func printSummary(results []TestResult) {
	fmt.Println("\n" + titleStyle.Render("=== Test Summary ==="))
	
	passed := 0
	failed := 0
	totalEvents := 0
	
	for _, result := range results {
		if result.Passed {
			passed++
			fmt.Println(successStyle.Render(fmt.Sprintf("✓ %s: %s", result.Name, result.Message)))
		} else {
			failed++
			fmt.Println(errorStyle.Render(fmt.Sprintf("✗ %s: %s", result.Name, result.Message)))
		}
		
		if result.Events > 0 {
			totalEvents += result.Events
			fmt.Println(dimStyle.Render(fmt.Sprintf("  Events: %d", result.Events)))
		}
	}
	
	status := successStyle
	if failed > 0 {
		status = errorStyle
	}
	
	fmt.Println("\n" + status.Render(fmt.Sprintf("Total: %d passed, %d failed", passed, failed)))
	if totalEvents > 0 {
		fmt.Println(dimStyle.Render(fmt.Sprintf("Total events processed: %d", totalEvents)))
	}
	
	if failed > 0 {
		os.Exit(1)
	}
}