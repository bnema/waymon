//go:build integration

package integration

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bnema/waymon/internal/adapters/out/ssh"
	"github.com/bnema/waymon/internal/domain"
)

// TestSSH_BasicConnection tests the basic connect/disconnect cycle.
func TestSSH_BasicConnection(t *testing.T) {
	ctx := testContext(t)
	port := findAvailablePort(t)
	hostKeyPath := generateTempHostKey(t)
	keyPair := generateTempSSHKeyPair(t)

	// Create and start server
	server := ssh.NewServerAdapter(ssh.ServerConfig{
		Port:         port,
		BindAddress:  "127.0.0.1",
		HostKeyPath:  hostKeyPath,
		AuthKeysPath: keyPair.AuthKeysPath,
		MaxClients:   5,
	})

	// Track connections
	var connected atomic.Bool
	var disconnected atomic.Bool

	server.SetOnClientConnected(func(addr, publicKey string) {
		connected.Store(true)
		t.Logf("Client connected: %s", addr)
	})

	server.SetOnClientDisconnected(func(addr string) {
		disconnected.Store(true)
		t.Logf("Client disconnected: %s", addr)
	})

	// Always allow auth for tests
	server.SetOnAuthRequest(func(addr, publicKey, fingerprint string) bool {
		return true
	})

	require.NoError(t, server.Start(ctx))
	defer server.Stop()

	// Wait for server to be ready
	err := waitForPort(t, port, 5*time.Second)
	require.NoError(t, err, "server should be listening")

	// Create client and connect
	client := ssh.NewClientAdapter()
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	err = client.Connect(ctx, addr, keyPair.PrivateKeyPath)
	require.NoError(t, err, "client should connect successfully")

	// Verify connection state
	assert.True(t, client.IsConnected(), "client should be connected")
	waitForCondition(t, connected.Load, 1*time.Second, "server should receive connection")

	// Disconnect
	err = client.Disconnect()
	require.NoError(t, err)

	assert.False(t, client.IsConnected(), "client should be disconnected")
	waitForCondition(t, disconnected.Load, 1*time.Second, "server should receive disconnection")
}

// TestSSH_EventTransmission tests sending mouse/keyboard events.
func TestSSH_EventTransmission(t *testing.T) {
	ctx := testContext(t)
	port := findAvailablePort(t)
	hostKeyPath := generateTempHostKey(t)
	keyPair := generateTempSSHKeyPair(t)

	// Create server
	server := ssh.NewServerAdapter(ssh.ServerConfig{
		Port:         port,
		BindAddress:  "127.0.0.1",
		HostKeyPath:  hostKeyPath,
		AuthKeysPath: keyPair.AuthKeysPath,
		MaxClients:   5,
	})

	// Track received events
	var receivedEvents []*domain.InputEvent
	var mu sync.Mutex

	server.SetOnInputEvent(func(event *domain.InputEvent) {
		mu.Lock()
		receivedEvents = append(receivedEvents, event)
		mu.Unlock()
	})

	server.SetOnAuthRequest(func(_, _, _ string) bool { return true })

	require.NoError(t, server.Start(ctx))
	defer server.Stop()

	err := waitForPort(t, port, 5*time.Second)
	require.NoError(t, err)

	// Connect client
	client := ssh.NewClientAdapter()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	require.NoError(t, client.Connect(ctx, addr, keyPair.PrivateKeyPath))
	defer func() { _ = client.Disconnect() }()

	// Wait for connection to be established
	time.Sleep(100 * time.Millisecond)

	// Send events
	events := []*domain.InputEvent{
		{
			MouseMove: &domain.MouseMoveEvent{DX: 10, DY: 20},
		},
		{
			MouseMove: &domain.MouseMoveEvent{DX: -5, DY: 15},
		},
		{
			MouseButton: &domain.MouseButtonEvent{Button: 1, Pressed: true},
		},
		{
			MouseButton: &domain.MouseButtonEvent{Button: 1, Pressed: false},
		},
	}

	for _, event := range events {
		err := client.SendEvent(ctx, event)
		require.NoError(t, err, "should send event successfully")
	}

	// Wait for events to be received
	waitForCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(receivedEvents) >= len(events)
	}, 2*time.Second, "all events should be received")

	mu.Lock()
	defer mu.Unlock()
	assert.GreaterOrEqual(t, len(receivedEvents), len(events), "should receive all events")
}

// TestSSH_BidirectionalEvents tests sending events from server to client.
func TestSSH_BidirectionalEvents(t *testing.T) {
	ctx := testContext(t)
	port := findAvailablePort(t)
	hostKeyPath := generateTempHostKey(t)
	keyPair := generateTempSSHKeyPair(t)

	server := ssh.NewServerAdapter(ssh.ServerConfig{
		Port:         port,
		BindAddress:  "127.0.0.1",
		HostKeyPath:  hostKeyPath,
		AuthKeysPath: keyPair.AuthKeysPath,
		MaxClients:   5,
	})

	var clientAddr string
	var mu sync.Mutex

	server.SetOnClientConnected(func(addr, _ string) {
		mu.Lock()
		clientAddr = addr
		mu.Unlock()
	})

	server.SetOnAuthRequest(func(_, _, _ string) bool { return true })

	require.NoError(t, server.Start(ctx))
	defer server.Stop()

	err := waitForPort(t, port, 5*time.Second)
	require.NoError(t, err)

	// Track events received by client
	var clientReceivedEvents []*domain.InputEvent
	var clientMu sync.Mutex

	client := ssh.NewClientAdapter()
	client.SetOnInputEvent(func(event *domain.InputEvent) {
		clientMu.Lock()
		clientReceivedEvents = append(clientReceivedEvents, event)
		clientMu.Unlock()
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	require.NoError(t, client.Connect(ctx, addr, keyPair.PrivateKeyPath))
	defer func() { _ = client.Disconnect() }()

	// Wait for connection
	waitForCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return clientAddr != ""
	}, 2*time.Second, "client should connect")

	// Send event from server to client
	mu.Lock()
	targetAddr := clientAddr
	mu.Unlock()

	event := &domain.InputEvent{
		MouseMove: &domain.MouseMoveEvent{DX: 100, DY: 200},
	}

	err = server.SendEventToClient(ctx, targetAddr, event)
	require.NoError(t, err, "server should send event to client")

	// Wait for client to receive event
	waitForCondition(t, func() bool {
		clientMu.Lock()
		defer clientMu.Unlock()
		return len(clientReceivedEvents) > 0
	}, 2*time.Second, "client should receive event")
}

// TestSSH_MultipleClients tests connecting multiple clients.
func TestSSH_MultipleClients(t *testing.T) {
	ctx := testContext(t)
	port := findAvailablePort(t)
	hostKeyPath := generateTempHostKey(t)
	keyPair := generateTempSSHKeyPair(t)

	server := ssh.NewServerAdapter(ssh.ServerConfig{
		Port:         port,
		BindAddress:  "127.0.0.1",
		HostKeyPath:  hostKeyPath,
		AuthKeysPath: keyPair.AuthKeysPath,
		MaxClients:   5,
	})

	var connectedCount atomic.Int32

	server.SetOnClientConnected(func(_, _ string) {
		connectedCount.Add(1)
	})

	server.SetOnClientDisconnected(func(_ string) {
		connectedCount.Add(-1)
	})

	server.SetOnAuthRequest(func(_, _, _ string) bool { return true })

	require.NoError(t, server.Start(ctx))
	defer server.Stop()

	err := waitForPort(t, port, 5*time.Second)
	require.NoError(t, err)

	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Connect multiple clients
	const numClients = 3
	clients := make([]*ssh.ClientAdapter, numClients)

	for i := 0; i < numClients; i++ {
		clients[i] = ssh.NewClientAdapter()
		err := clients[i].Connect(ctx, addr, keyPair.PrivateKeyPath)
		require.NoError(t, err, "client %d should connect", i)
	}

	// Wait for all connections
	waitForCondition(t, func() bool {
		return connectedCount.Load() == int32(numClients)
	}, 2*time.Second, "all clients should connect")

	// Verify connected clients count
	connectedClients := server.GetConnectedClients()
	assert.Len(t, connectedClients, numClients, "should have correct number of connected clients")

	// Disconnect clients
	for i, client := range clients {
		err := client.Disconnect()
		require.NoError(t, err, "client %d should disconnect", i)
	}

	// Wait for all disconnections
	waitForCondition(t, func() bool {
		return connectedCount.Load() == 0
	}, 2*time.Second, "all clients should disconnect")
}

// TestSSH_ClientReconnection tests reconnecting after disconnect.
func TestSSH_ClientReconnection(t *testing.T) {
	ctx := testContext(t)
	port := findAvailablePort(t)
	hostKeyPath := generateTempHostKey(t)
	keyPair := generateTempSSHKeyPair(t)

	server := ssh.NewServerAdapter(ssh.ServerConfig{
		Port:         port,
		BindAddress:  "127.0.0.1",
		HostKeyPath:  hostKeyPath,
		AuthKeysPath: keyPair.AuthKeysPath,
		MaxClients:   5,
	})

	var connectionCount atomic.Int32

	server.SetOnClientConnected(func(_, _ string) {
		connectionCount.Add(1)
	})

	server.SetOnAuthRequest(func(_, _, _ string) bool { return true })

	require.NoError(t, server.Start(ctx))
	defer server.Stop()

	err := waitForPort(t, port, 5*time.Second)
	require.NoError(t, err)

	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// First connection
	client := ssh.NewClientAdapter()
	require.NoError(t, client.Connect(ctx, addr, keyPair.PrivateKeyPath))

	waitForCondition(t, func() bool {
		return connectionCount.Load() >= 1
	}, 2*time.Second, "first connection should succeed")

	// Disconnect
	require.NoError(t, client.Disconnect())
	time.Sleep(100 * time.Millisecond)

	// Reconnect with new client (old one is spent)
	client2 := ssh.NewClientAdapter()
	require.NoError(t, client2.Connect(ctx, addr, keyPair.PrivateKeyPath))
	defer func() { _ = client2.Disconnect() }()

	waitForCondition(t, func() bool {
		return connectionCount.Load() >= 2
	}, 2*time.Second, "reconnection should succeed")

	assert.True(t, client2.IsConnected(), "client should be connected after reconnection")
}

// TestSSH_AuthenticationFailure tests rejection of invalid credentials.
func TestSSH_AuthenticationFailure(t *testing.T) {
	ctx := testContext(t)
	port := findAvailablePort(t)
	hostKeyPath := generateTempHostKey(t)
	validKeyPair := generateTempSSHKeyPair(t)
	invalidKeyPair := generateTempSSHKeyPair(t) // Different key pair

	server := ssh.NewServerAdapter(ssh.ServerConfig{
		Port:         port,
		BindAddress:  "127.0.0.1",
		HostKeyPath:  hostKeyPath,
		AuthKeysPath: validKeyPair.AuthKeysPath,
		MaxClients:   5,
	})

	// Reject all auth for this test
	server.SetOnAuthRequest(func(_, _, _ string) bool {
		return false
	})

	require.NoError(t, server.Start(ctx))
	defer server.Stop()

	err := waitForPort(t, port, 5*time.Second)
	require.NoError(t, err)

	// Try to connect with wrong key
	client := ssh.NewClientAdapter()
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	err = client.Connect(ctx, addr, invalidKeyPair.PrivateKeyPath)
	assert.Error(t, err, "connection should fail with invalid key")
	assert.False(t, client.IsConnected(), "client should not be connected")
}

// TestSSH_GracefulShutdown tests server shutdown with active clients.
func TestSSH_GracefulShutdown(t *testing.T) {
	ctx := testContext(t)
	port := findAvailablePort(t)
	hostKeyPath := generateTempHostKey(t)
	keyPair := generateTempSSHKeyPair(t)

	server := ssh.NewServerAdapter(ssh.ServerConfig{
		Port:         port,
		BindAddress:  "127.0.0.1",
		HostKeyPath:  hostKeyPath,
		AuthKeysPath: keyPair.AuthKeysPath,
		MaxClients:   5,
	})

	server.SetOnAuthRequest(func(_, _, _ string) bool { return true })

	require.NoError(t, server.Start(ctx))

	err := waitForPort(t, port, 5*time.Second)
	require.NoError(t, err)

	// Connect client
	client := ssh.NewClientAdapter()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	require.NoError(t, client.Connect(ctx, addr, keyPair.PrivateKeyPath))

	time.Sleep(100 * time.Millisecond)
	assert.True(t, client.IsConnected(), "client should be connected")

	// Stop server
	server.Stop()

	// Wait for port to close
	err = waitForPortClosed(t, port, 5*time.Second)
	require.NoError(t, err, "server port should close")
}

// TestSSH_EventOrdering tests that events are received in order.
func TestSSH_EventOrdering(t *testing.T) {
	ctx := testContext(t)
	port := findAvailablePort(t)
	hostKeyPath := generateTempHostKey(t)
	keyPair := generateTempSSHKeyPair(t)

	server := ssh.NewServerAdapter(ssh.ServerConfig{
		Port:         port,
		BindAddress:  "127.0.0.1",
		HostKeyPath:  hostKeyPath,
		AuthKeysPath: keyPair.AuthKeysPath,
		MaxClients:   5,
	})

	var receivedEvents []*domain.InputEvent
	var mu sync.Mutex

	server.SetOnInputEvent(func(event *domain.InputEvent) {
		mu.Lock()
		receivedEvents = append(receivedEvents, event)
		mu.Unlock()
	})

	server.SetOnAuthRequest(func(_, _, _ string) bool { return true })

	require.NoError(t, server.Start(ctx))
	defer server.Stop()

	err := waitForPort(t, port, 5*time.Second)
	require.NoError(t, err)

	client := ssh.NewClientAdapter()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	require.NoError(t, client.Connect(ctx, addr, keyPair.PrivateKeyPath))
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	// Send numbered events
	const numEvents = 50
	for i := 0; i < numEvents; i++ {
		event := &domain.InputEvent{
			MouseMove: &domain.MouseMoveEvent{
				DX: float64(i),
				DY: float64(i * 10),
			},
		}
		require.NoError(t, client.SendEvent(ctx, event))
	}

	// Wait for all events
	waitForCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(receivedEvents) >= numEvents
	}, 5*time.Second, "all events should be received")

	mu.Lock()
	defer mu.Unlock()

	// Verify ordering
	for i := 0; i < numEvents && i < len(receivedEvents); i++ {
		event := receivedEvents[i]
		require.NotNil(t, event.MouseMove, "event %d should be mouse move", i)
		assert.Equal(t, float64(i), event.MouseMove.DX, "event %d DX should match", i)
	}
}

// TestSSH_LargeEventBurst tests handling of many events in quick succession.
func TestSSH_LargeEventBurst(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large burst test in short mode")
	}

	ctx := testContext(t)
	port := findAvailablePort(t)
	hostKeyPath := generateTempHostKey(t)
	keyPair := generateTempSSHKeyPair(t)

	server := ssh.NewServerAdapter(ssh.ServerConfig{
		Port:         port,
		BindAddress:  "127.0.0.1",
		HostKeyPath:  hostKeyPath,
		AuthKeysPath: keyPair.AuthKeysPath,
		MaxClients:   5,
	})

	var receivedCount atomic.Int32

	server.SetOnInputEvent(func(_ *domain.InputEvent) {
		receivedCount.Add(1)
	})

	server.SetOnAuthRequest(func(_, _, _ string) bool { return true })

	require.NoError(t, server.Start(ctx))
	defer server.Stop()

	err := waitForPort(t, port, 5*time.Second)
	require.NoError(t, err)

	client := ssh.NewClientAdapter()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	require.NoError(t, client.Connect(ctx, addr, keyPair.PrivateKeyPath))
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	// Send many events quickly
	const numEvents = 500
	start := time.Now()

	for i := 0; i < numEvents; i++ {
		event := &domain.InputEvent{
			MouseMove: &domain.MouseMoveEvent{DX: 1, DY: 1},
		}
		err := client.SendEvent(ctx, event)
		require.NoError(t, err)
	}

	elapsed := time.Since(start)
	t.Logf("Sent %d events in %v (%.0f events/sec)", numEvents, elapsed, float64(numEvents)/elapsed.Seconds())

	// Wait for events to be processed
	waitForCondition(t, func() bool {
		return receivedCount.Load() >= int32(numEvents)
	}, 10*time.Second, "all events should be received")

	assert.GreaterOrEqual(t, receivedCount.Load(), int32(numEvents), "should receive all events")
}

// TestSSH_MaxClientsLimit tests that max client limit is enforced.
func TestSSH_MaxClientsLimit(t *testing.T) {
	ctx := testContext(t)
	port := findAvailablePort(t)
	hostKeyPath := generateTempHostKey(t)
	keyPair := generateTempSSHKeyPair(t)

	const maxClients = 2

	server := ssh.NewServerAdapter(ssh.ServerConfig{
		Port:         port,
		BindAddress:  "127.0.0.1",
		HostKeyPath:  hostKeyPath,
		AuthKeysPath: keyPair.AuthKeysPath,
		MaxClients:   maxClients,
	})

	var connectedCount atomic.Int32

	server.SetOnClientConnected(func(_, _ string) {
		connectedCount.Add(1)
	})

	server.SetOnAuthRequest(func(_, _, _ string) bool { return true })

	require.NoError(t, server.Start(ctx))
	defer server.Stop()

	err := waitForPort(t, port, 5*time.Second)
	require.NoError(t, err)

	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Connect max number of clients
	clients := make([]*ssh.ClientAdapter, maxClients)
	for i := 0; i < maxClients; i++ {
		clients[i] = ssh.NewClientAdapter()
		err := clients[i].Connect(ctx, addr, keyPair.PrivateKeyPath)
		require.NoError(t, err, "client %d should connect", i)
	}

	waitForCondition(t, func() bool {
		return connectedCount.Load() == int32(maxClients)
	}, 2*time.Second, "max clients should connect")

	// Try to connect one more client - should be rejected
	extraClient := ssh.NewClientAdapter()
	_ = extraClient.Connect(ctx, addr, keyPair.PrivateKeyPath) // May fail or succeed at TCP level

	// The connection may succeed at TCP level but session should fail
	// or server may reject it - either way verify we don't exceed max
	time.Sleep(500 * time.Millisecond)

	connectedClients := server.GetConnectedClients()
	assert.LessOrEqual(t, len(connectedClients), maxClients, "should not exceed max clients")

	// Cleanup
	for _, client := range clients {
		_ = client.Disconnect()
	}
	_ = extraClient.Disconnect()
}

// TestSSH_Ping tests the ping/pong mechanism.
func TestSSH_Ping(t *testing.T) {
	ctx := testContext(t)
	port := findAvailablePort(t)
	hostKeyPath := generateTempHostKey(t)
	keyPair := generateTempSSHKeyPair(t)

	server := ssh.NewServerAdapter(ssh.ServerConfig{
		Port:         port,
		BindAddress:  "127.0.0.1",
		HostKeyPath:  hostKeyPath,
		AuthKeysPath: keyPair.AuthKeysPath,
		MaxClients:   5,
	})

	server.SetOnAuthRequest(func(_, _, _ string) bool { return true })

	require.NoError(t, server.Start(ctx))
	defer server.Stop()

	err := waitForPort(t, port, 5*time.Second)
	require.NoError(t, err)

	client := ssh.NewClientAdapter()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	require.NoError(t, client.Connect(ctx, addr, keyPair.PrivateKeyPath))
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	// Send ping
	err = client.SendPing()
	require.NoError(t, err, "should send ping successfully")

	// Give time for pong response
	time.Sleep(200 * time.Millisecond)

	assert.True(t, client.IsConnected(), "client should still be connected after ping")
}
