//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
)

// Environment detection helpers

// skipIfNoWayland skips the test if WAYLAND_DISPLAY is not set.
func skipIfNoWayland(t *testing.T) {
	t.Helper()
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("Skipping: WAYLAND_DISPLAY not set")
	}
}

// skipIfNotRoot skips the test if not running as root.
func skipIfNotRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("Skipping: requires root privileges")
	}
}

// skipIfNoInputDevices skips the test if no input devices are available.
func skipIfNoInputDevices(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir("/dev/input")
	if err != nil || len(entries) == 0 {
		t.Skip("Skipping: no input devices found")
	}

	hasEventDevice := false
	for _, e := range entries {
		if len(e.Name()) > 5 && e.Name()[:5] == "event" {
			hasEventDevice = true
			break
		}
	}
	if !hasEventDevice {
		t.Skip("Skipping: no event devices found")
	}
}

// isCI returns true if running in a CI environment.
func isCI() bool {
	return os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != ""
}

// SSH key management (ephemeral per test)

// SSHKeyPair holds paths to ephemeral SSH key files.
type SSHKeyPair struct {
	PrivateKeyPath string
	PublicKeyPath  string
	AuthKeysPath   string // authorized_keys file with the public key
}

// generateTempSSHKeyPair generates an ephemeral RSA key pair for testing.
// The keys are automatically cleaned up when the test completes.
func generateTempSSHKeyPair(t *testing.T) *SSHKeyPair {
	t.Helper()

	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Create temp directory for keys
	tmpDir := t.TempDir()
	privateKeyPath := filepath.Join(tmpDir, "id_rsa")
	publicKeyPath := filepath.Join(tmpDir, "id_rsa.pub")
	authKeysPath := filepath.Join(tmpDir, "authorized_keys")

	// Write private key
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if err := os.WriteFile(privateKeyPath, privateKeyPEM, 0600); err != nil {
		t.Fatalf("Failed to write private key: %v", err)
	}

	// Create SSH public key
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to create SSH public key: %v", err)
	}
	publicKeyBytes := ssh.MarshalAuthorizedKey(publicKey)

	// Write public key (readable by all for authorized_keys lookup)
	if err := os.WriteFile(publicKeyPath, publicKeyBytes, 0600); err != nil { //nolint:gosec // Test file permissions
		t.Fatalf("Failed to write public key: %v", err)
	}

	// Write authorized_keys (readable by all for SSH server)
	if err := os.WriteFile(authKeysPath, publicKeyBytes, 0600); err != nil { //nolint:gosec // Test file permissions
		t.Fatalf("Failed to write authorized_keys: %v", err)
	}

	return &SSHKeyPair{
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  publicKeyPath,
		AuthKeysPath:   authKeysPath,
	}
}

// generateTempHostKey generates a temporary host key for SSH server testing.
func generateTempHostKey(t *testing.T) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate host key: %v", err)
	}

	tmpDir := t.TempDir()
	hostKeyPath := filepath.Join(tmpDir, "host_key")

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if err := os.WriteFile(hostKeyPath, privateKeyPEM, 0600); err != nil {
		t.Fatalf("Failed to write host key: %v", err)
	}

	return hostKeyPath
}

// Port management

// findAvailablePort finds an available TCP port for testing.
func findAvailablePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("Failed to close listener: %v", err)
	}
	return port
}

// waitForPort waits for a TCP port to become available.
func waitForPort(t *testing.T, port int, timeout time.Duration) error {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			_ = conn.Close() // Best effort close on successful probe
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("port %d not available after %v", port, timeout)
}

// waitForPortClosed waits for a TCP port to become unavailable.
func waitForPortClosed(t *testing.T, port int, timeout time.Duration) error {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err != nil {
			return nil // Port is closed
		}
		_ = conn.Close() // Best effort close on successful probe
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("port %d still open after %v", port, timeout)
}

// Context helpers

// testContext returns a context with a zerolog logger for testing.
func testContext(t *testing.T) context.Context {
	t.Helper()

	logger := zerolog.New(zerolog.NewTestWriter(t)).
		With().
		Timestamp().
		Logger()

	return logger.WithContext(t.Context())
}

// testContextWithTimeout returns a context with a zerolog logger and timeout.
func testContextWithTimeout(t *testing.T, d time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()

	ctx := testContext(t)
	return context.WithTimeout(ctx, d)
}

// Socket path helpers

// tempSocketPath returns a unique temporary socket path for IPC testing.
func tempSocketPath(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	return filepath.Join(tmpDir, "waymon-test.sock")
}

// tempConfigPath returns a unique temporary config file path.
func tempConfigPath(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	return filepath.Join(tmpDir, "waymon-test.toml")
}

// Event helpers

// waitForCondition waits for a condition to become true with timeout.
func waitForCondition(t *testing.T, condition func() bool, timeout time.Duration, description string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Condition not met within %v: %s", timeout, description)
}

// assertEventually asserts that a condition becomes true within timeout.
func assertEventually(t *testing.T, condition func() bool, timeout time.Duration, msgAndArgs ...interface{}) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	msg := "condition not met"
	if len(msgAndArgs) > 0 {
		msg = fmt.Sprintf("%v", msgAndArgs[0])
	}
	t.Fatalf("assertEventually failed after %v: %s", timeout, msg)
}

// Concurrency helpers

// parallelTest runs a function in a goroutine and returns a channel for the result.
func parallelTest(fn func() error) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- fn()
	}()
	return ch
}
