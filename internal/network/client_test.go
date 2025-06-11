package network

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestClient_Connect(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer func() { _ = listener.Close() }()

	// Accept connections in background
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		// Keep connection open
		time.Sleep(100 * time.Millisecond)
	}()

	tests := []struct {
		name    string
		address string
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "valid connection",
			address: listener.Addr().String(),
			timeout: 1 * time.Second,
			wantErr: false,
		},
		{
			name:    "invalid address",
			address: "invalid:address",
			timeout: 100 * time.Millisecond,
			wantErr: true,
		},
		{
			name:    "connection refused",
			address: "localhost:59999",
			timeout: 100 * time.Millisecond,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient()
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			err := client.Connect(ctx, tt.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.Connect() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if !client.IsConnected() {
					t.Error("Client should be connected")
				}
				client.Disconnect()
			}
		})
	}
}

func TestClient_Disconnect(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer func() { _ = listener.Close() }()

	// Accept connections
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Keep connection open until client disconnects
			buf := make([]byte, 1)
			_, _ = conn.Read(buf)
			_ = conn.Close()
		}
	}()

	client := NewClient()
	ctx := context.Background()

	// Connect
	err = client.Connect(ctx, listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if !client.IsConnected() {
		t.Error("Client should be connected")
	}

	// Disconnect
	client.Disconnect()

	if client.IsConnected() {
		t.Error("Client should be disconnected")
	}

	// Disconnect again should not panic
	client.Disconnect()
}

func TestClient_Reconnect(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer func() { _ = listener.Close() }()

	// Accept connections
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			buf := make([]byte, 1)
			_, _ = conn.Read(buf)
			_ = conn.Close()
		}
	}()

	client := NewClient()
	ctx := context.Background()
	address := listener.Addr().String()

	// First connection
	err = client.Connect(ctx, address)
	if err != nil {
		t.Fatalf("First connection failed: %v", err)
	}

	// Disconnect
	client.Disconnect()

	// Reconnect
	err = client.Connect(ctx, address)
	if err != nil {
		t.Fatalf("Reconnection failed: %v", err)
	}

	if !client.IsConnected() {
		t.Error("Client should be connected after reconnection")
	}

	client.Disconnect()
}