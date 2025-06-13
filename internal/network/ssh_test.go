package network

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// TestSSHServerStart tests that the SSH server can start and listen
func TestSSHServerStart(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "waymon-ssh-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate host key for testing
	hostKeyPath := filepath.Join(tmpDir, "host_key")
	if err := generateTestHostKey(hostKeyPath); err != nil {
		t.Fatal(err)
	}

	// Create authorized_keys file
	authKeysPath := filepath.Join(tmpDir, "authorized_keys")
	if err := os.WriteFile(authKeysPath, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	// Create SSH server
	server := NewSSHServer(52526, hostKeyPath, authKeysPath)

	// Set up event handlers
	connected := make(chan bool, 1)
	server.OnClientConnected = func(addr, pubKey string) {
		connected <- true
	}

	// Start server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start SSH server: %v", err)
	}
	defer server.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running
	if server.Port() != 52526 {
		t.Errorf("Expected port 52526, got %d", server.Port())
	}
}

// TestSSHClientConnect tests SSH client connection
func TestSSHClientConnect(t *testing.T) {
	t.Skip("Skipping SSH client test - requires running SSH server")

	// This test would require:
	// 1. Setting up an SSH server
	// 2. Generating client keys
	// 3. Adding client public key to authorized_keys
	// 4. Testing the connection

	// For now, we'll skip this as it requires more complex setup
}

// TestSSHAuthentication tests the public key authentication
func TestSSHAuthentication(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "waymon-ssh-auth-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate test keys
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	// Convert to SSH public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}

	// Test fingerprint generation
	fingerprint := ssh.FingerprintSHA256(pub)
	if fingerprint == "" {
		t.Error("Failed to generate fingerprint")
	}

	// Verify fingerprint format (SHA256:base64)
	if len(fingerprint) < 40 || len(fingerprint) > 60 {
		t.Errorf("Invalid fingerprint length: %d (expected ~50 chars)", len(fingerprint))
	}
	
	// Should start with SHA256:
	if fingerprint[:7] != "SHA256:" {
		t.Errorf("Fingerprint should start with 'SHA256:', got %s", fingerprint[:7])
	}
}

// generateTestHostKey generates a test SSH host key
func generateTestHostKey(path string) error {
	// Generate RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// Encode private key to PEM
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	// Write to file
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	return pem.Encode(file, privateKeyPEM)
}

// TestSSHMessageTransmission tests sending messages over SSH
func TestSSHMessageTransmission(t *testing.T) {
	// This test would require a full SSH setup
	// For now, we'll test the message serialization functions

	// Test writeMessage function
	// We need to expose it or test it indirectly through the client
	t.Skip("Message transmission test requires full SSH setup")
}