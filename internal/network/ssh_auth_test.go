package network

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bnema/waymon/internal/config"
	"github.com/spf13/viper"
	gossh "golang.org/x/crypto/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSSHAuthHandlerRaceCondition tests that auth handler is available when connections arrive
func TestSSHAuthHandlerRaceCondition(t *testing.T) {
	// Create test SSH keys
	clientKey, err := GenerateTestSSHKey()
	require.NoError(t, err)
	
	fingerprint := gossh.FingerprintSHA256(clientKey.PublicKey())
	
	// Reset viper and config
	viper.Reset()
	// Set test-specific values before Init to ensure they take precedence
	viper.Set("server.ssh_whitelist_only", true)
	viper.Set("server.ssh_whitelist", []string{})
	config.Init()
	
	// Verify config is set correctly
	cfg := config.Get()
	require.True(t, cfg.Server.SSHWhitelistOnly, "SSHWhitelistOnly should be true")
	require.Empty(t, cfg.Server.SSHWhitelist, "SSHWhitelist should be empty")
	
	// Create test host key
	hostKeyPath := GenerateTestHostKey(t)
	
	// Create server with whitelist-only mode enabled
	server := NewSSHServer(52530, hostKeyPath, "") // Use specific port for testing
	authHandlerCalled := false
	
	// Simulate the UI not being ready immediately
	time.AfterFunc(200*time.Millisecond, func() {
		server.OnAuthRequest = func(addr, publicKey, fp string) bool {
			authHandlerCalled = true
			assert.Equal(t, fingerprint, fp)
			return true
		}
	})
	
	// Start server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop()
	
	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)
	
	// Get actual port after server starts
	port := server.Port()
	require.NotZero(t, port, "server port should be assigned")
	
	// Try to connect immediately (before auth handler is set)
	conn1, err := gossh.Dial("tcp", fmt.Sprintf("localhost:%d", port), &gossh.ClientConfig{
		User: "test",
		Auth: []gossh.AuthMethod{
			gossh.PublicKeys(clientKey),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:        2 * time.Second,
	})
	
	// Debug: log the current config state
	currentCfg := config.Get()
	t.Logf("Config state: SSHWhitelistOnly=%v, SSHWhitelist=%v", currentCfg.Server.SSHWhitelistOnly, currentCfg.Server.SSHWhitelist)
	t.Logf("Generated key fingerprint: %s", fingerprint)
	t.Logf("Connection result: err=%v, authHandlerCalled=%v", err, authHandlerCalled)
	
	if conn1 != nil {
		conn1.Close()
	}
	
	// Should fail because auth handler wasn't ready
	require.Error(t, err, "Connection should fail when no auth handler is set")
	assert.Contains(t, err.Error(), "unable to authenticate")
	assert.False(t, authHandlerCalled, "auth handler should not have been called")
	
	// Wait for auth handler to be set
	time.Sleep(250 * time.Millisecond)
	
	// Try again after auth handler is ready
	conn, err := gossh.Dial("tcp", fmt.Sprintf("localhost:%d", port), &gossh.ClientConfig{
		User: "test",
		Auth: []gossh.AuthMethod{
			gossh.PublicKeys(clientKey),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:        2 * time.Second,
	})
	
	// Should succeed now
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()
	
	assert.True(t, authHandlerCalled, "auth handler should have been called")
}

// TestSSHAuthApprovalFlow tests the complete auth approval flow
func TestSSHAuthApprovalFlow(t *testing.T) {
	tests := []struct {
		name          string
		approve       bool
		expectConnect bool
	}{
		{
			name:          "approved connection",
			approve:       true,
			expectConnect: true,
		},
		{
			name:          "denied connection",
			approve:       false,
			expectConnect: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test SSH keys
			clientKey, err := GenerateTestSSHKey()
			require.NoError(t, err)
			
			// Reset viper and config
			viper.Reset()
			config.Init()
			viper.Set("server.ssh_whitelist_only", true)
			viper.Set("server.ssh_whitelist", []string{})
			
			// Create test host key
			hostKeyPath := GenerateTestHostKey(t)
			
			// Create server
			server := NewSSHServer(52531, hostKeyPath, "")
			
			// Set up auth handler
			server.OnAuthRequest = func(addr, publicKey, fingerprint string) bool {
				return tt.approve
			}
			
			// Start server
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			err = server.Start(ctx)
			require.NoError(t, err)
			defer server.Stop()
			
			// Wait for server to start
			time.Sleep(200 * time.Millisecond)
			
			// Get actual port after server starts
			port := server.Port()
			require.NotZero(t, port, "server port should be assigned")
			
			// Try to connect
			conn, err := gossh.Dial("tcp", fmt.Sprintf("localhost:%d", port), &gossh.ClientConfig{
				User: "test",
				Auth: []gossh.AuthMethod{
					gossh.PublicKeys(clientKey),
				},
				HostKeyCallback: gossh.InsecureIgnoreHostKey(),
				Timeout:        2 * time.Second,
			})
			
			if tt.expectConnect {
				require.NoError(t, err)
				require.NotNil(t, conn)
				conn.Close()
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unable to authenticate")
			}
		})
	}
}

// TestSSHAuthWithWhitelist tests authentication with whitelisted keys
func TestSSHAuthWithWhitelist(t *testing.T) {
	// Create test SSH keys
	clientKey, err := GenerateTestSSHKey()
	require.NoError(t, err)
	
	fingerprint := gossh.FingerprintSHA256(clientKey.PublicKey())
	
	// Reset viper and config
	viper.Reset()
	config.Init()
	
	// Set up config with whitelisted key
	cfg := config.Get()
	cfg.Server.SSHWhitelistOnly = true
	cfg.Server.SSHWhitelist = []string{fingerprint}
	config.Set(cfg)
	
	// Create test host key
	hostKeyPath := GenerateTestHostKey(t)
	
	// Create server
	server := NewSSHServer(52532, hostKeyPath, "")
	authHandlerCalled := false
	
	server.OnAuthRequest = func(addr, publicKey, fp string) bool {
		authHandlerCalled = true
		return false // Should not matter because key is whitelisted
	}
	
	// Start server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop()
	
	// Wait for server to start
	time.Sleep(200 * time.Millisecond)
	
	// Get actual port after server starts
	port := server.Port()
	require.NotZero(t, port, "server port should be assigned")
	
	// Try to connect with whitelisted key
	conn, err := gossh.Dial("tcp", fmt.Sprintf("localhost:%d", port), &gossh.ClientConfig{
		User: "test",
		Auth: []gossh.AuthMethod{
			gossh.PublicKeys(clientKey),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:        2 * time.Second,
	})
	
	// Should succeed without calling auth handler
	require.NoError(t, err)
	require.NotNil(t, conn)
	conn.Close()
	
	assert.False(t, authHandlerCalled, "auth handler should not be called for whitelisted keys")
}

// TestSSHAuthWhitelistOnlyDisabled tests authentication when whitelist-only mode is disabled
func TestSSHAuthWhitelistOnlyDisabled(t *testing.T) {
	// Create test SSH keys
	clientKey, err := GenerateTestSSHKey()
	require.NoError(t, err)
	
	// Reset viper and config
	viper.Reset()
	config.Init()
	
	// Set up config with whitelist disabled
	cfg := config.Get()
	cfg.Server.SSHWhitelistOnly = false
	cfg.Server.SSHWhitelist = []string{}
	config.Set(cfg)
	
	// Create test host key
	hostKeyPath := GenerateTestHostKey(t)
	
	// Create server
	server := NewSSHServer(52533, hostKeyPath, "")
	authHandlerCalled := false
	
	server.OnAuthRequest = func(addr, publicKey, fingerprint string) bool {
		authHandlerCalled = true
		return false // Should not matter because whitelist-only is disabled
	}
	
	// Start server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop()
	
	// Wait for server to start
	time.Sleep(200 * time.Millisecond)
	
	// Get actual port after server starts
	port := server.Port()
	require.NotZero(t, port, "server port should be assigned")
	
	// Try to connect
	conn, err := gossh.Dial("tcp", fmt.Sprintf("localhost:%d", port), &gossh.ClientConfig{
		User: "test",
		Auth: []gossh.AuthMethod{
			gossh.PublicKeys(clientKey),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:        2 * time.Second,
	})
	
	// Should succeed without calling auth handler
	require.NoError(t, err)
	require.NotNil(t, conn)
	conn.Close()
	
	assert.False(t, authHandlerCalled, "auth handler should not be called when whitelist-only is disabled")
}

// TestSSHAuthTimeout tests that auth requests timeout after 30 seconds
func TestSSHAuthTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}
	
	// Create test SSH keys
	clientKey, err := GenerateTestSSHKey()
	require.NoError(t, err)
	
	// Reset viper and config
	viper.Reset()
	config.Init()
	viper.Set("server.ssh_whitelist_only", true)
	viper.Set("server.ssh_whitelist", []string{})
	
	// Create test host key
	hostKeyPath := GenerateTestHostKey(t)
	
	// Create server
	server := NewSSHServer(52534, hostKeyPath, "")
	
	// Channel to signal when auth handler is called
	authHandlerCalled := make(chan struct{}, 1)
	
	// Set up auth handler that takes too long
	server.OnAuthRequest = func(addr, publicKey, fingerprint string) bool {
		select {
		case authHandlerCalled <- struct{}{}:
		default:
		}
		// Simulate UI never responding (longer than timeout)
		time.Sleep(35 * time.Second)
		return true
	}
	
	// Start server
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop()
	
	// Wait for server to start
	time.Sleep(200 * time.Millisecond)
	
	// Get actual port after server starts
	port := server.Port()
	require.NotZero(t, port, "server port should be assigned")
	
	// Try to connect
	start := time.Now()
	_, err = gossh.Dial("tcp", fmt.Sprintf("localhost:%d", port), &gossh.ClientConfig{
		User: "test",
		Auth: []gossh.AuthMethod{
			gossh.PublicKeys(clientKey),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:        35 * time.Second,
	})
	elapsed := time.Since(start)
	
	// Wait for auth handler to be called
	select {
	case <-authHandlerCalled:
		// Good, handler was called
	case <-time.After(5 * time.Second):
		t.Fatal("auth handler was not called")
	}
	
	// Should fail after timeout
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to authenticate")
	
	// Should have taken around 30 seconds (the auth timeout)
	assert.GreaterOrEqual(t, elapsed, 28*time.Second)
	assert.LessOrEqual(t, elapsed, 32*time.Second)
}

