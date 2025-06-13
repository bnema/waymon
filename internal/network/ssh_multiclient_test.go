package network

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bnema/waymon/internal/config"
	"github.com/spf13/viper"
)

// TestSSHMultipleClients tests handling multiple client connection attempts
func TestSSHMultipleClients(t *testing.T) {
	// Set up test config to disable whitelist-only mode
	viper.Reset()
	config.Init()
	viper.Set("server.ssh_whitelist_only", false)

	// Setup test environment
	tmpDir, err := os.MkdirTemp("", "waymon-ssh-multi")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate host key
	hostKeyPath := filepath.Join(tmpDir, "host_key")
	if err := generateTestHostKey(hostKeyPath); err != nil {
		t.Fatal(err)
	}

	// Generate multiple client keys
	var clientKeys []string
	for i := 0; i < 3; i++ {
		clientKeyPath := filepath.Join(tmpDir, "client_key_"+string(rune('0'+i)))
		clientPubKeyPath := clientKeyPath + ".pub"

		if err := generateTestClientKeys(clientKeyPath, clientPubKeyPath); err != nil {
			t.Fatal(err)
		}
		clientKeys = append(clientKeys, clientKeyPath)

		// Add all keys to authorized_keys
		pubKeyData, err := os.ReadFile(clientPubKeyPath)
		if err != nil {
			t.Fatal(err)
		}
		authKeysPath := filepath.Join(tmpDir, "authorized_keys")
		f, err := os.OpenFile(authKeysPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(pubKeyData)
		f.Close()
	}

	// Start SSH server
	server := NewSSHServer(52528, hostKeyPath, filepath.Join(tmpDir, "authorized_keys"))
	server.SetMaxClients(1) // Explicitly set to only allow 1 client

	// Track events
	events := make(chan string, 10)

	server.OnClientConnected = func(addr, pubKey string) {
		events <- "connected:" + addr
	}

	server.OnClientDisconnected = func(addr string) {
		events <- "disconnected:" + addr
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start SSH server: %v", err)
	}
	defer server.Stop()

	time.Sleep(200 * time.Millisecond)

	// Test case 1: Single client connects successfully
	t.Run("single_client", func(t *testing.T) {
		client1 := NewSSHClient(clientKeys[0])

		if err := client1.Connect(ctx, "localhost:52528"); err != nil {
			t.Fatalf("Client 1 failed to connect: %v", err)
		}

		// Wait for connection event
		select {
		case event := <-events:
			if !strings.HasPrefix(event, "connected:") {
				t.Errorf("Expected connection event, got %s", event)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for connection event")
		}

		// Disconnect
		client1.Disconnect()

		// Wait for disconnection event
		select {
		case event := <-events:
			if !strings.HasPrefix(event, "disconnected:") {
				t.Errorf("Expected disconnection event, got %s", event)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for disconnection event")
		}
	})

	// Test case 2: Second client rejected while first is connected
	t.Run("reject_second_client", func(t *testing.T) {
		client1 := NewSSHClient(clientKeys[0])
		client2 := NewSSHClient(clientKeys[1])

		// First client connects
		if err := client1.Connect(ctx, "localhost:52528"); err != nil {
			t.Fatalf("Client 1 failed to connect: %v", err)
		}
		defer client1.Disconnect()

		// Wait for first connection
		<-events

		// Second client should be rejected
		err := client2.Connect(ctx, "localhost:52528")
		if err == nil {
			client2.Disconnect()
			t.Error("Expected second client to be rejected, but it connected")
		}

		// Should only get disconnection from client1
		client1.Disconnect()
		<-events
	})

	// Test case 3: Rapid connection attempts
	t.Run("rapid_connections", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, 5)

		// Try 5 simultaneous connections
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(clientNum int) {
				defer wg.Done()

				client := NewSSHClient(clientKeys[clientNum%3])
				err := client.Connect(ctx, "localhost:52528")
				if err != nil {
					errors <- err
				} else {
					// If connected, disconnect after a moment
					time.Sleep(100 * time.Millisecond)
					client.Disconnect()
					errors <- nil
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Count successful connections
		successCount := 0
		for err := range errors {
			if err == nil {
				successCount++
			}
		}

		// Only one should succeed
		if successCount != 1 {
			t.Errorf("Expected exactly 1 successful connection, got %d", successCount)
		}

		// Drain events
		for len(events) > 0 {
			<-events
		}
	})

	// Test case 4: Unauthorized client
	t.Run("unauthorized_client", func(t *testing.T) {
		// Create a new key not in authorized_keys
		unauthorizedKeyPath := filepath.Join(tmpDir, "unauthorized_key")
		if err := generateTestClientKeys(unauthorizedKeyPath, unauthorizedKeyPath+".pub"); err != nil {
			t.Fatal(err)
		}

		// Update server to actually check authorized_keys
		// For now, the server accepts all keys, so this test would need
		// the server to implement proper authorized_keys checking
		t.Skip("Server currently accepts all keys - need to implement authorized_keys checking")
	})

	// Test case 5: Client reconnection
	t.Run("client_reconnection", func(t *testing.T) {
		client := NewSSHClient(clientKeys[0])

		// Connect
		if err := client.Connect(ctx, "localhost:52528"); err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		<-events // connection event

		// Disconnect
		client.Disconnect()
		<-events // disconnection event

		// Reconnect
		if err := client.Reconnect(ctx, "localhost:52528"); err != nil {
			t.Fatalf("Failed to reconnect: %v", err)
		}

		// Wait for reconnection event
		select {
		case event := <-events:
			if !strings.HasPrefix(event, "connected:") {
				t.Errorf("Expected reconnection event, got %s", event)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for reconnection event")
		}

		client.Disconnect()
		<-events // final disconnection
	})
}

// TestSSHClientConnectionStates tests various client connection states
func TestSSHClientConnectionStates(t *testing.T) {
	// Test connecting without a server
	t.Run("no_server", func(t *testing.T) {
		client := NewSSHClient("")

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := client.Connect(ctx, "localhost:59999") // Non-existent port
		if err == nil {
			client.Disconnect()
			t.Error("Expected connection to fail with no server")
		}

		if client.IsConnected() {
			t.Error("Client should not report connected after failed connection")
		}
	})

	// Test with invalid key
	t.Run("invalid_key", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "invalid-key")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())

		// Write invalid key data
		tmpFile.WriteString("not a valid private key")
		tmpFile.Close()

		client := NewSSHClient(tmpFile.Name())

		ctx := context.Background()
		err = client.Connect(ctx, "localhost:52525")
		if err == nil {
			client.Disconnect()
			t.Error("Expected connection to fail with invalid key")
		}
	})

	// Test disconnect when not connected
	t.Run("disconnect_not_connected", func(t *testing.T) {
		client := NewSSHClient("")

		err := client.Disconnect()
		if err != nil {
			t.Errorf("Disconnect on non-connected client returned error: %v", err)
		}
	})

	// Test double connect``
	t.Run("double_connect", func(t *testing.T) {
		// This would need a running server
		t.Skip("Requires running SSH server")
	})
}
