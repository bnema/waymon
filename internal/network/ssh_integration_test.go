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

	"github.com/bnema/waymon/internal/config"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
)

// TestSSHIntegration tests the full SSH server/client flow
func TestSSHIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up test config to disable whitelist-only mode
	viper.Reset()
	config.Init()
	viper.Set("server.ssh_whitelist_only", false)

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "waymon-ssh-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate host key
	hostKeyPath := filepath.Join(tmpDir, "host_key")
	if err := generateTestHostKey(hostKeyPath); err != nil {
		t.Fatal(err)
	}

	// Generate client key pair
	clientKeyPath := filepath.Join(tmpDir, "client_key")
	clientPubKeyPath := filepath.Join(tmpDir, "client_key.pub")
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

	// Start SSH server
	server := NewSSHServer(52527, hostKeyPath, authKeysPath)

	// Set up a simple auth handler for testing
	server.OnAuthRequest = func(addr, publicKey, fingerprint string) bool {
		// Accept all keys for testing
		return true
	}

	// Track connections
	connected := make(chan string, 1)
	disconnected := make(chan string, 1)

	server.OnClientConnected = func(addr, pubKey string) {
		t.Logf("Client connected: %s with key %s", addr, pubKey)
		connected <- addr
	}

	server.OnClientDisconnected = func(addr string) {
		t.Logf("Client disconnected: %s", addr)
		disconnected <- addr
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start SSH server: %v", err)
	}
	defer server.Stop()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Create SSH client
	client := NewSSHClient(clientKeyPath)

	// Connect to server
	if err := client.Connect(ctx, "localhost:52527"); err != nil {
		t.Fatalf("Failed to connect SSH client: %v", err)
	}

	// Wait for connection
	select {
	case addr := <-connected:
		t.Logf("Server reported connection from: %s", addr)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for connection")
	}

	// Verify client is connected
	if !client.IsConnected() {
		t.Error("Client reports not connected")
	}

	// Disconnect
	if err := client.Disconnect(); err != nil {
		t.Errorf("Failed to disconnect: %v", err)
	}

	// Wait for disconnection
	select {
	case addr := <-disconnected:
		t.Logf("Server reported disconnection from: %s", addr)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for disconnection")
	}
}

// generateTestClientKeys generates a test SSH client key pair
func generateTestClientKeys(privateKeyPath, publicKeyPath string) error {
	// Generate RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// Save private key
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	privFile, err := os.OpenFile(privateKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer privFile.Close()

	if err := pem.Encode(privFile, privateKeyPEM); err != nil {
		return err
	}

	// Generate SSH public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}

	// Format as authorized_keys line
	authorizedKey := ssh.MarshalAuthorizedKey(pub)

	return os.WriteFile(publicKeyPath, authorizedKey, 0644)
}
