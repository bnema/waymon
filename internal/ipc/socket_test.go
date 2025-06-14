package ipc

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	pb "github.com/bnema/waymon/internal/proto"
)

// MockHandler implements MessageHandler for testing
type MockHandler struct {
	switchCalled bool
	statusCalled bool
	switchResponse *pb.IPCMessage
	statusResponse *pb.IPCMessage
	switchError error
	statusError error
}

func (m *MockHandler) HandleSwitchCommand(cmd *pb.SwitchCommand) (*pb.IPCMessage, error) {
	m.switchCalled = true
	if m.switchError != nil {
		return nil, m.switchError
	}
	if m.switchResponse != nil {
		return m.switchResponse, nil
	}
	// Default response
	return NewStatusResponseMessageLegacy(true, true, "test:52525")
}

func (m *MockHandler) HandleStatusQuery(query *pb.StatusQuery) (*pb.IPCMessage, error) {
	m.statusCalled = true
	if m.statusError != nil {
		return nil, m.statusError
	}
	if m.statusResponse != nil {
		return m.statusResponse, nil
	}
	// Default response
	return NewStatusResponseMessageLegacy(false, false, "")
}

func TestNewSocketServer(t *testing.T) {
	handler := &MockHandler{}
	server, err := NewSocketServer(handler)
	if err != nil {
		t.Fatalf("NewSocketServer() error = %v", err)
	}

	if server.handler != handler {
		t.Error("Handler not set correctly")
	}

	if server.socketPath == "" {
		t.Error("Socket path not set")
	}
}

func TestSocketServerStartStop(t *testing.T) {
	handler := &MockHandler{}
	server, err := NewSocketServer(handler)
	if err != nil {
		t.Fatalf("NewSocketServer() error = %v", err)
	}

	// Use a test socket path
	tempDir := t.TempDir()
	server.socketPath = filepath.Join(tempDir, "test.sock")

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Check that socket file exists
	if _, err := os.Stat(server.socketPath); os.IsNotExist(err) {
		t.Error("Socket file was not created")
	}

	// Starting again should not error
	if err := server.Start(); err != nil {
		t.Errorf("Start() on running server error = %v", err)
	}

	// Stop server
	server.Stop()

	// Check that socket file is cleaned up
	if _, err := os.Stat(server.socketPath); !os.IsNotExist(err) {
		t.Error("Socket file was not cleaned up")
	}

	// Stopping again should not panic
	server.Stop()
}

func TestGetSocketPath(t *testing.T) {
	path, err := GetSocketPath()
	if err != nil {
		t.Fatalf("GetSocketPath() error = %v", err)
	}

	if path == "" {
		t.Error("Socket path is empty")
	}

	if !filepath.IsAbs(path) {
		t.Error("Socket path is not absolute")
	}

	// Should contain the current user's name
	if !contains(path, "/tmp/waymon-") {
		t.Errorf("Socket path should contain '/tmp/waymon-', got %s", path)
	}
}

func TestSocketServerMultipleStarts(t *testing.T) {
	handler := &MockHandler{}
	server, err := NewSocketServer(handler)
	if err != nil {
		t.Fatalf("NewSocketServer() error = %v", err)
	}

	// Use a test socket path
	tempDir := t.TempDir()
	server.socketPath = filepath.Join(tempDir, "test.sock")

	// Start multiple times should not error
	for i := 0; i < 3; i++ {
		if err := server.Start(); err != nil {
			t.Fatalf("Start() iteration %d error = %v", i, err)
		}
	}

	server.Stop()
}

func TestSocketServerCleanupExistingSocket(t *testing.T) {
	handler := &MockHandler{}
	server, err := NewSocketServer(handler)
	if err != nil {
		t.Fatalf("NewSocketServer() error = %v", err)
	}

	// Use a test socket path
	tempDir := t.TempDir()
	server.socketPath = filepath.Join(tempDir, "test.sock")

	// Create a dummy socket file
	file, err := os.Create(server.socketPath)
	if err != nil {
		t.Fatalf("Failed to create dummy socket file: %v", err)
	}
	file.Close()

	// Start should succeed and clean up the existing file
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	server.Stop()
}

func TestSocketServerContextCancellation(t *testing.T) {
	handler := &MockHandler{}
	server, err := NewSocketServer(handler)
	if err != nil {
		t.Fatalf("NewSocketServer() error = %v", err)
	}

	// Use a test socket path
	tempDir := t.TempDir()
	server.socketPath = filepath.Join(tempDir, "test.sock")

	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Stop should complete quickly
	done := make(chan struct{})
	go func() {
		server.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Error("Stop() took too long")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		 containsInMiddle(s, substr))))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}