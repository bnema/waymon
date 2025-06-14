package network

import (
	"testing"
	"time"

	"github.com/bnema/waymon/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSHWhitelistFunctionality(t *testing.T) {
	// Initialize config for testing
	require.NoError(t, config.Init())

	// Create server
	server := NewSSHServer(0, "", "")

	// Track auth requests
	authRequests := make(chan struct {
		addr        string
		fingerprint string
		approved    bool
	}, 1)

	server.OnAuthRequest = func(addr, publicKey, fingerprint string) bool {
		// Simulate approval based on fingerprint
		approved := fingerprint == "SHA256:test-approved-key"
		authRequests <- struct {
			addr        string
			fingerprint string
			approved    bool
		}{addr, fingerprint, approved}
		return approved
	}

	// Test the auth request handler
	t.Run("Auth request handler", func(t *testing.T) {
		// Test approved key
		approved := server.OnAuthRequest("192.168.1.100:12345", "mock-key", "SHA256:test-approved-key")
		assert.True(t, approved)

		// Check request was tracked
		select {
		case req := <-authRequests:
			assert.Equal(t, "192.168.1.100:12345", req.addr)
			assert.Equal(t, "SHA256:test-approved-key", req.fingerprint)
			assert.True(t, req.approved)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Expected auth request but none received")
		}

		// Test denied key
		denied := server.OnAuthRequest("192.168.1.101:12345", "mock-key", "SHA256:test-denied-key")
		assert.False(t, denied)

		// Check request was tracked
		select {
		case req := <-authRequests:
			assert.Equal(t, "192.168.1.101:12345", req.addr)
			assert.Equal(t, "SHA256:test-denied-key", req.fingerprint)
			assert.False(t, req.approved)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Expected auth request but none received")
		}
	})
}

func TestWhitelistConfigManagement(t *testing.T) {
	// Initialize config
	require.NoError(t, config.Init())

	// Clear any existing whitelist
	cfg := config.Get()
	cfg.Server.SSHWhitelist = []string{}
	cfg.Server.SSHWhitelistOnly = true

	t.Run("Add to whitelist", func(t *testing.T) {
		testFingerprint := "SHA256:test-add-key"

		// Add key to whitelist
		err := config.AddSSHKeyToWhitelist(testFingerprint)
		assert.NoError(t, err)

		// Check it's whitelisted
		assert.True(t, config.IsSSHKeyWhitelisted(testFingerprint))

		// Try adding again - should fail
		err = config.AddSSHKeyToWhitelist(testFingerprint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already whitelisted")
	})

	t.Run("Remove from whitelist", func(t *testing.T) {
		testFingerprint := "SHA256:test-remove-key"

		// Add key first
		err := config.AddSSHKeyToWhitelist(testFingerprint)
		assert.NoError(t, err)
		assert.True(t, config.IsSSHKeyWhitelisted(testFingerprint))

		// Remove from whitelist
		err = config.RemoveSSHKeyFromWhitelist(testFingerprint)
		assert.NoError(t, err)

		// Check it's no longer whitelisted
		assert.False(t, config.IsSSHKeyWhitelisted(testFingerprint))

		// Try removing again - should fail
		err = config.RemoveSSHKeyFromWhitelist(testFingerprint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Multiple keys", func(t *testing.T) {
		// Clear whitelist
		cfg := config.Get()
		cfg.Server.SSHWhitelist = []string{}

		keys := []string{
			"SHA256:key-1",
			"SHA256:key-2",
			"SHA256:key-3",
		}

		// Add multiple keys
		for _, key := range keys {
			err := config.AddSSHKeyToWhitelist(key)
			assert.NoError(t, err)
		}

		// Check all are whitelisted
		for _, key := range keys {
			assert.True(t, config.IsSSHKeyWhitelisted(key))
		}

		// Check a non-whitelisted key
		assert.False(t, config.IsSSHKeyWhitelisted("SHA256:unknown-key"))
	})
}

func TestWhitelistModeToggle(t *testing.T) {
	// Initialize config
	require.NoError(t, config.Init())

	cfg := config.Get()

	t.Run("Whitelist-only mode enabled", func(t *testing.T) {
		cfg.Server.SSHWhitelistOnly = true
		assert.True(t, cfg.Server.SSHWhitelistOnly)
	})

	t.Run("Whitelist-only mode disabled", func(t *testing.T) {
		cfg.Server.SSHWhitelistOnly = false
		assert.False(t, cfg.Server.SSHWhitelistOnly)
	})
}
