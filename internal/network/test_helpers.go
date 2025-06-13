package network

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

// GenerateTestSSHKey generates a test RSA key pair for testing
func GenerateTestSSHKey() (gossh.Signer, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	
	signer, err := gossh.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, err
	}
	
	return signer, nil
}

// GenerateTestSSHKeyFiles generates test SSH key files and returns paths
func GenerateTestSSHKeyFiles(t *testing.T) (privateKeyPath, publicKeyPath string) {
	t.Helper()
	
	// Create temp directory
	tempDir := t.TempDir()
	
	// Generate key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	
	// Write private key
	privateKeyPath = filepath.Join(tempDir, "id_rsa")
	privateKeyFile, err := os.Create(privateKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer privateKeyFile.Close()
	
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		t.Fatal(err)
	}
	
	// Generate public key
	publicKey, err := gossh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	
	// Write public key
	publicKeyPath = filepath.Join(tempDir, "id_rsa.pub")
	publicKeyData := gossh.MarshalAuthorizedKey(publicKey)
	if err := os.WriteFile(publicKeyPath, publicKeyData, 0644); err != nil {
		t.Fatal(err)
	}
	
	return privateKeyPath, publicKeyPath
}

// GenerateTestHostKey generates a test host key file and returns the path
func GenerateTestHostKey(t *testing.T) string {
	t.Helper()
	
	// Create temp directory
	tempDir := t.TempDir()
	
	// Generate key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	
	// Write host key
	hostKeyPath := filepath.Join(tempDir, "host_key")
	hostKeyFile, err := os.Create(hostKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer hostKeyFile.Close()
	
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	if err := pem.Encode(hostKeyFile, privateKeyPEM); err != nil {
		t.Fatal(err)
	}
	
	return hostKeyPath
}