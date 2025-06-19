package network

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bnema/waymon/internal/protocol"
)

// TestSSHClientServerIntegration tests the full client-server communication
func TestSSHClientServerIntegration(t *testing.T) {
	// Skip if running in CI or short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create temporary directory for test keys
	tmpDir, err := os.MkdirTemp("", "waymon-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hostKeyPath := filepath.Join(tmpDir, "host_key")
	authKeysPath := filepath.Join(tmpDir, "authorized_keys")
	clientKeyPath := filepath.Join(tmpDir, "client_key")

	// Generate test keys
	if err := GenerateTestKeys(hostKeyPath, clientKeyPath, authKeysPath); err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	// Create server
	server := NewSSHServer(52526, hostKeyPath, authKeysPath)

	// Track server events
	var serverReceivedEvent *protocol.InputEvent
	server.OnInputEvent = func(event *protocol.InputEvent) {
		serverReceivedEvent = event
		t.Logf("Server received event: %T from %s", event.Event, event.SourceId)
	}

	var connectedClientAddr string
	server.OnClientConnected = func(addr, publicKey string) {
		connectedClientAddr = addr
		t.Logf("Client connected: %s with key %s", addr, publicKey)
	}

	// Set auth handler to accept all connections for testing
	server.SetAuthHandlers(func(addr, publicKey, fingerprint string) bool {
		t.Logf("Auth request from %s with fingerprint %s", addr, fingerprint)
		return true
	})

	// Start server
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Create client
	client := NewSSHClient(clientKeyPath)

	// Track received events on client
	var clientReceivedEvent *protocol.InputEvent
	client.OnInputEvent(func(event *protocol.InputEvent) {
		clientReceivedEvent = event
		t.Logf("Client received event: %T from %s", event.Event, event.SourceId)
	})

	// Connect client
	err = client.Connect(ctx, "localhost:52526")
	if err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}
	defer client.Disconnect()

	// Wait for connection to establish
	time.Sleep(200 * time.Millisecond)

	// Test 1: Send event from client to server
	t.Run("ClientToServer", func(t *testing.T) {
		clientEvent := &protocol.InputEvent{
			Event: &protocol.InputEvent_Control{
				Control: &protocol.ControlEvent{
					Type: protocol.ControlEvent_CLIENT_CONFIG,
					ClientConfig: &protocol.ClientConfig{
						ClientId:   "test-client",
						ClientName: "Test Client",
						Monitors: []*protocol.Monitor{
							{
								Name:    "TEST-1",
								X:       0,
								Y:       0,
								Width:   1920,
								Height:  1080,
								Primary: true,
								Scale:   1.0,
							},
						},
						Capabilities: &protocol.ClientCapabilities{
							CanReceiveKeyboard: true,
							CanReceiveMouse:    true,
							WaylandCompositor:  "test",
						},
					},
				},
			},
			Timestamp: time.Now().UnixNano(),
			SourceId:  "test-client",
		}

		if err := client.SendInputEvent(clientEvent); err != nil {
			t.Errorf("Failed to send event from client: %v", err)
		}

		// Wait for event to be received
		time.Sleep(200 * time.Millisecond)

		// Verify server received the event
		if serverReceivedEvent == nil {
			t.Fatal("Server did not receive event")
		}

		if control := serverReceivedEvent.GetControl(); control == nil || control.Type != protocol.ControlEvent_CLIENT_CONFIG {
			t.Errorf("Unexpected event received by server: %v", serverReceivedEvent)
		}
	})

	// Test 2: Send event from server to client
	t.Run("ServerToClient", func(t *testing.T) {
		// Reset received event
		clientReceivedEvent = nil

		// Server needs to send to a specific client address
		if connectedClientAddr == "" {
			t.Fatal("No client connected to server")
		}

		serverEvent := &protocol.InputEvent{
			Event: &protocol.InputEvent_Control{
				Control: &protocol.ControlEvent{
					Type:     protocol.ControlEvent_REQUEST_CONTROL,
					TargetId: "test-server",
				},
			},
			Timestamp: time.Now().UnixNano(),
			SourceId:  "server",
		}

		if err := server.SendEventToClient(connectedClientAddr, serverEvent); err != nil {
			t.Errorf("Failed to send event from server: %v", err)
		}

		// Wait for event to be received
		time.Sleep(200 * time.Millisecond)

		// Verify client received the event
		if clientReceivedEvent == nil {
			t.Fatal("Client did not receive event")
		}

		if control := clientReceivedEvent.GetControl(); control == nil || control.Type != protocol.ControlEvent_REQUEST_CONTROL {
			t.Errorf("Unexpected event received by client: %v", clientReceivedEvent)
		}
	})

	// Test 3: Multiple events in sequence
	t.Run("MultipleEvents", func(t *testing.T) {
		// Send multiple mouse move events
		for i := 0; i < 5; i++ {
			mouseEvent := &protocol.InputEvent{
				Event: &protocol.InputEvent_MouseMove{
					MouseMove: &protocol.MouseMoveEvent{
						Dx: float64(i * 10),
						Dy: float64(i * 5),
					},
				},
				Timestamp: time.Now().UnixNano(),
				SourceId:  "test-client",
			}

			if err := client.SendInputEvent(mouseEvent); err != nil {
				t.Errorf("Failed to send mouse event %d: %v", i, err)
			}
		}

		// Give some time for all events to be processed
		time.Sleep(500 * time.Millisecond)
	})
}
