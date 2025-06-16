package network

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"

	"golang.org/x/crypto/ssh"
)

// GenerateTestKeys generates RSA keys for testing
func GenerateTestKeys(hostKeyPath, clientKeyPath, authKeysPath string) error {
	// Generate host key
	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// Generate client key
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// Save host private key
	hostKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(hostKey),
	}
	hostKeyFile, err := os.Create(hostKeyPath)
	if err != nil {
		return err
	}
	defer hostKeyFile.Close()
	if err := pem.Encode(hostKeyFile, hostKeyPEM); err != nil {
		return err
	}

	// Save client private key
	clientKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(clientKey),
	}
	clientKeyFile, err := os.Create(clientKeyPath)
	if err != nil {
		return err
	}
	defer clientKeyFile.Close()
	if err := pem.Encode(clientKeyFile, clientKeyPEM); err != nil {
		return err
	}

	// Create authorized_keys with client public key
	sshPubKey, err := ssh.NewPublicKey(&clientKey.PublicKey)
	if err != nil {
		return err
	}
	pubKeyBytes := ssh.MarshalAuthorizedKey(sshPubKey)
	
	return os.WriteFile(authKeysPath, pubKeyBytes, 0600)
}