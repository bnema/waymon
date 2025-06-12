package network

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestServer_Start(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{
			name:    "valid port",
			port:    52525,
			wantErr: false,
		},
		{
			name:    "invalid port",
			port:    -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(tt.port)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			err := server.Start(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Server.Start() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify server is listening
				conn, err := net.Dial("tcp", server.Address())
				if err != nil {
					t.Errorf("Failed to connect to server: %v", err)
				} else {
					_ = conn.Close()
				}
				server.Stop()
			}
		})
	}
}

func TestServer_AcceptConnections(t *testing.T) {
	server := NewServer(52526)
	ctx := context.Background()

	// Start server in background
	go func() {
		if err := server.Start(ctx); err != nil {
			t.Errorf("Server failed to start: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Connect a client
	conn, err := net.Dial("tcp", "localhost:52526")
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Check if server reports the connection
	select {
	case <-server.Connected():
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Server did not report client connection")
	}

	// Close connection
	_ = conn.Close()

	// Check if server reports disconnection
	select {
	case <-server.Disconnected():
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Server did not report client disconnection")
	}

	server.Stop()
}

func TestServer_OnlyOneClient(t *testing.T) {
	server := NewServer(52527)
	ctx := context.Background()

	go func() {
		if err := server.Start(ctx); err != nil {
			t.Errorf("Server failed to start: %v", err)
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// Connect first client
	conn1, err := net.Dial("tcp", "localhost:52527")
	if err != nil {
		t.Fatalf("Failed to connect first client: %v", err)
	}
	defer func() { _ = conn1.Close() }()

	// Try to connect second client
	conn2, err := net.Dial("tcp", "localhost:52527")
	if err != nil {
		t.Fatalf("Failed to connect second client: %v", err)
	}
	defer func() { _ = conn2.Close() }()

	// Second connection should be rejected
	// Try to read from second connection, should fail
	buf := make([]byte, 1)
	_ = conn2.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, err = conn2.Read(buf)
	if err == nil {
		t.Error("Second client connection should have been rejected")
	}

	server.Stop()
}
