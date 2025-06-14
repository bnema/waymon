package network

import (
	"context"
	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/logger"
	"github.com/spf13/viper"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDebugSSHConnection helps diagnose why the client hangs
func TestDebugSSHConnection(t *testing.T) {
	// Set up debug logging to stdout for test visibility
	os.Setenv("LOG_LEVEL", "DEBUG")
	logger.SetLevel("DEBUG")
	logger.SetOutput(os.Stdout)

	// Set up test config
	viper.Reset()
	viper.Set("server.ssh_whitelist_only", false)
	config.Init()

	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "waymon-debug-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate test keys
	hostKeyPath := filepath.Join(tmpDir, "host_key")
	if err := generateTestHostKey(hostKeyPath); err != nil {
		t.Fatal(err)
	}

	clientKeyPath := filepath.Join(tmpDir, "client_key")
	clientPubKeyPath := clientKeyPath + ".pub"
	if err := generateTestClientKeys(clientKeyPath, clientPubKeyPath); err != nil {
		t.Fatal(err)
	}

	// Create authorized_keys with client public key
	authKeysPath := filepath.Join(tmpDir, "authorized_keys")
	pubKeyData, err := os.ReadFile(clientPubKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authKeysPath, pubKeyData, 0600); err != nil {
		t.Fatal(err)
	}

	// Start SSH server on port 52534
	server := NewSSHServer(52534, hostKeyPath, authKeysPath)
	server.SetAuthHandlers(func(addr, publicKey, fingerprint string) bool {
		return true // Accept all for testing
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start SSH server: %v", err)
	}
	defer server.Stop()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	t.Run("Step1_TCPConnectivity", func(t *testing.T) {
		t.Log("Testing basic TCP connectivity to localhost:52534...")
		conn, err := net.DialTimeout("tcp", "localhost:52534", 5*time.Second)
		if err != nil {
			t.Fatalf("TCP connection failed: %v", err)
		}
		t.Log("✓ TCP connection successful")
		conn.Close()
	})

	t.Run("Step2_SSHClientCreation", func(t *testing.T) {
		t.Log("Creating SSH client...")
		client := NewSSHClient(clientKeyPath)
		if client == nil {
			t.Fatal("Failed to create SSH client")
		}
		t.Log("✓ SSH client created successfully")
	})

	t.Run("Step3_SSHConnectionWithTimeout", func(t *testing.T) {
		client := NewSSHClient(clientKeyPath)

		// Create a context with explicit timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		t.Log("Starting SSH connection attempt...")
		startTime := time.Now()

		// Create a channel to signal completion
		done := make(chan error, 1)

		// Run connection in goroutine
		go func() {
			t.Log("Goroutine: Calling client.Connect...")
			err := client.Connect(ctx, "localhost:52534")
			t.Logf("Goroutine: client.Connect returned: %v", err)
			done <- err
		}()

		// Wait for completion or timeout
		select {
		case err := <-done:
			duration := time.Since(startTime)
			t.Logf("Connection completed in %v", duration)
			if err != nil {
				t.Errorf("Connection failed: %v", err)
			} else {
				t.Log("✓ Successfully connected!")
				client.Disconnect()
			}
		case <-time.After(15 * time.Second):
			t.Fatal("Connection attempt timed out after 15 seconds - client.Connect is hanging")
		}
	})
}

// TestMinimalSSHClient tests the SSH client connection with minimal setup
func TestMinimalSSHClient(t *testing.T) {
	// Enable all debug output
	os.Setenv("LOG_LEVEL", "DEBUG")
	logger.SetLevel("DEBUG")

	homeDir, _ := os.UserHomeDir()
	privateKeyPath := filepath.Join(homeDir, ".ssh", "id_ed25519")

	t.Logf("Using private key: %s", privateKeyPath)

	// Check server is reachable
	conn, err := net.DialTimeout("tcp", "127.0.0.1:52525", 2*time.Second)
	if err != nil {
		t.Skipf("Server not reachable at 127.0.0.1:52525: %v", err)
	}
	conn.Close()

	// Test connection
	client := NewSSHClient(privateKeyPath)
	ctx := context.Background()

	t.Log("Calling Connect()...")
	err = client.Connect(ctx, "127.0.0.1:52525")
	if err != nil {
		t.Fatalf("Connection failed: %v", err)
	}

	t.Log("Connection successful!")
	client.Disconnect()
}
