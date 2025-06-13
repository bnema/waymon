package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInlineClientModel(t *testing.T) {
	t.Run("creates new inline client model", func(t *testing.T) {
		model := NewInlineClientModel("192.168.1.100:52525")

		if model.serverAddr != "192.168.1.100:52525" {
			t.Errorf("Expected server address '192.168.1.100:52525', got %q", model.serverAddr)
		}
		if model.connected {
			t.Error("Should not be connected initially")
		}
		if model.capturing {
			t.Error("Should not be capturing initially")
		}
	})

	t.Run("renders disconnected view", func(t *testing.T) {
		model := NewInlineClientModel("192.168.1.100:52525")
		view := model.View()

		// Check for key elements
		if !strings.Contains(view, "WAYMON") {
			t.Error("Should contain 'WAYMON'")
		}
		if !strings.Contains(view, "Connecting") {
			t.Error("Should show connecting status")
		}
		if !strings.Contains(view, "192.168.1.100:52525") {
			t.Error("Should show server address")
		}
		if !strings.Contains(view, "■ Idle") {
			t.Error("Should show idle status when not capturing")
		}
	})

	t.Run("handles connection message", func(t *testing.T) {
		model := NewInlineClientModel("192.168.1.100:52525")
		
		updatedModel, _ := model.Update(ConnectedMsg{})
		updated := updatedModel.(*InlineClientModel)

		if !updated.connected {
			t.Error("Should be connected after ConnectedMsg")
		}
		if updated.message != "Connected to server" {
			t.Errorf("Expected success message, got %q", updated.message)
		}
	})

	t.Run("handles disconnection message", func(t *testing.T) {
		model := NewInlineClientModel("192.168.1.100:52525")
		model.connected = true
		model.capturing = true
		
		updatedModel, _ := model.Update(DisconnectedMsg{})
		updated := updatedModel.(*InlineClientModel)

		if updated.connected {
			t.Error("Should not be connected after DisconnectedMsg")
		}
		if updated.capturing {
			t.Error("Should stop capturing after disconnect")
		}
		if updated.message != "Disconnected from server" {
			t.Errorf("Expected error message, got %q", updated.message)
		}
	})

	t.Run("handles waiting approval message", func(t *testing.T) {
		model := NewInlineClientModel("192.168.1.100:52525")
		
		updatedModel, _ := model.Update(WaitingApprovalMsg{})
		updated := updatedModel.(*InlineClientModel)

		if !updated.waitingApproval {
			t.Error("Should be waiting for approval after WaitingApprovalMsg")
		}
		if updated.message != "Waiting for server approval..." {
			t.Errorf("Expected waiting message, got %q", updated.message)
		}
	})

	t.Run("renders waiting for approval view", func(t *testing.T) {
		model := NewInlineClientModel("192.168.1.100:52525")
		model.waitingApproval = true
		
		view := model.View()

		if !strings.Contains(view, "Waiting for approval") {
			t.Error("Should show waiting for approval status")
		}
		// Should show spinner while waiting
		if strings.Contains(view, "● Connected") {
			t.Error("Should not show connected status when waiting for approval")
		}
	})

	t.Run("toggles capture with space key", func(t *testing.T) {
		model := NewInlineClientModel("192.168.1.100:52525")
		model.connected = true
		
		// Toggle on
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeySpace})
		updated := updatedModel.(*InlineClientModel)

		if !updated.capturing {
			t.Error("Should be capturing after space key")
		}

		// Toggle off
		updatedModel2, _ := updated.Update(tea.KeyMsg{Type: tea.KeySpace})
		updated2 := updatedModel2.(*InlineClientModel)

		if updated2.capturing {
			t.Error("Should not be capturing after second space key")
		}
	})

	t.Run("renders connected and capturing view", func(t *testing.T) {
		model := NewInlineClientModel("192.168.1.100:52525")
		model.connected = true
		model.capturing = true
		
		view := model.View()

		if !strings.Contains(view, "● Connected") {
			t.Error("Should show connected status")
		}
		if !strings.Contains(view, "▶ CAPTURING") {
			t.Error("Should show capturing status")
		}
	})

	t.Run("clears expired messages", func(t *testing.T) {
		model := NewInlineClientModel("192.168.1.100:52525")
		model.SetMessage("info", "Test message")
		model.messageExpiry = time.Now().Add(-1 * time.Second) // Already expired
		
		updatedModel, _ := model.Update(tea.WindowSizeMsg{})
		updated := updatedModel.(*InlineClientModel)

		if updated.message != "" {
			t.Error("Should have cleared expired message")
		}
	})

	t.Run("quits on q key", func(t *testing.T) {
		model := NewInlineClientModel("192.168.1.100:52525")
		
		_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		
		if cmd == nil {
			t.Error("Should return quit command")
		}
	})
}

func TestInlineServerModel(t *testing.T) {
	t.Run("creates new inline server model", func(t *testing.T) {
		model := NewInlineServerModel(52525, "test-server")

		if model.port != 52525 {
			t.Errorf("Expected port 52525, got %d", model.port)
		}
		if model.serverName != "test-server" {
			t.Errorf("Expected server name 'test-server', got %q", model.serverName)
		}
		if model.clientCount != 0 {
			t.Error("Should have no clients initially")
		}
	})

	t.Run("renders server view", func(t *testing.T) {
		model := NewInlineServerModel(52525, "test-server")
		view := model.View()

		// Check for key elements
		if !strings.Contains(view, "WAYMON SERVER") {
			t.Error("Should contain 'WAYMON SERVER'")
		}
		if !strings.Contains(view, "Listening") {
			t.Error("Should show listening status")
		}
		if !strings.Contains(view, ":52525") {
			t.Error("Should show port")
		}
		if !strings.Contains(view, "No clients") {
			t.Error("Should show no clients initially")
		}
	})

	t.Run("handles client connected message", func(t *testing.T) {
		model := NewInlineServerModel(52525, "test-server")
		
		updatedModel, _ := model.Update(ClientConnectedMsg{ClientAddr: "192.168.1.100:12345"})
		updated := updatedModel.(*InlineServerModel)

		if updated.clientCount != 1 {
			t.Errorf("Expected 1 client, got %d", updated.clientCount)
		}
		if !strings.Contains(updated.message, "Client connected") {
			t.Errorf("Expected connection message, got %q", updated.message)
		}
	})

	t.Run("handles client disconnected message", func(t *testing.T) {
		model := NewInlineServerModel(52525, "test-server")
		model.clientCount = 2
		
		updatedModel, _ := model.Update(ClientDisconnectedMsg{ClientAddr: "192.168.1.100:12345"})
		updated := updatedModel.(*InlineServerModel)

		if updated.clientCount != 1 {
			t.Errorf("Expected 1 client remaining, got %d", updated.clientCount)
		}
		if !strings.Contains(updated.message, "Client disconnected") {
			t.Errorf("Expected disconnection message, got %q", updated.message)
		}
	})

	t.Run("renders view with multiple clients", func(t *testing.T) {
		model := NewInlineServerModel(52525, "test-server")
		model.clientCount = 3
		
		view := model.View()

		if !strings.Contains(view, "3 clients") {
			t.Error("Should show '3 clients'")
		}
	})

	t.Run("renders view with single client", func(t *testing.T) {
		model := NewInlineServerModel(52525, "test-server")
		model.clientCount = 1
		
		view := model.View()

		if !strings.Contains(view, "1 client") {
			t.Error("Should show '1 client' (singular)")
		}
		if strings.Contains(view, "1 clients") {
			t.Error("Should not show '1 clients' (plural)")
		}
	})

	t.Run("handles temporary messages", func(t *testing.T) {
		model := NewInlineServerModel(52525, "test-server")
		model.SetMessage("error", "Test error")
		
		view := model.View()

		if !strings.Contains(view, "Test error") {
			t.Error("Should display error message")
		}
		if model.messageExpiry.IsZero() {
			t.Error("Should set message expiry")
		}
	})

	t.Run("quits on q key", func(t *testing.T) {
		model := NewInlineServerModel(52525, "test-server")
		
		_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		
		if cmd == nil {
			t.Error("Should return quit command")
		}
	})

	t.Run("handles SSH auth request", func(t *testing.T) {
		model := NewInlineServerModel(52525, "test-server")
		responseChan := make(chan bool, 1)
		
		updatedModel, _ := model.Update(SSHAuthRequestMsg{
			ClientAddr:   "192.168.1.100:12345",
			PublicKey:    "ssh-ed25519 AAAAC...",
			Fingerprint:  "SHA256:abc123",
			ResponseChan: responseChan,
		})
		updated := updatedModel.(*InlineServerModel)

		if updated.pendingAuth == nil {
			t.Error("Should have pending auth after SSHAuthRequestMsg")
		}
		if updated.authChannel != responseChan {
			t.Error("Should store response channel")
		}
	})

	t.Run("renders auth prompt view", func(t *testing.T) {
		model := NewInlineServerModel(52525, "test-server")
		model.pendingAuth = &SSHAuthRequestMsg{
			ClientAddr:  "192.168.1.100:12345",
			Fingerprint: "SHA256:abc123",
		}
		
		view := model.View()

		if !strings.Contains(view, "⚠️  NEW CONNECTION:") {
			t.Error("Should show new connection warning")
		}
		if !strings.Contains(view, "192.168.1.100:12345") {
			t.Error("Should show client address")
		}
		if !strings.Contains(view, "SHA256:abc123") {
			t.Error("Should show SSH key fingerprint")
		}
		if !strings.Contains(view, "Allow this connection? [Y/n]") {
			t.Error("Should show approval prompt")
		}
	})

	t.Run("approves auth with Y key", func(t *testing.T) {
		model := NewInlineServerModel(52525, "test-server")
		responseChan := make(chan bool, 1)
		model.pendingAuth = &SSHAuthRequestMsg{
			ClientAddr:   "192.168.1.100:12345",
			Fingerprint:  "SHA256:abc123",
			ResponseChan: responseChan,
		}
		model.authChannel = responseChan
		
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
		updated := updatedModel.(*InlineServerModel)

		// Check approval was sent
		select {
		case approved := <-responseChan:
			if !approved {
				t.Error("Should have sent approval")
			}
		default:
			t.Error("Should have sent approval to channel")
		}

		if updated.pendingAuth != nil {
			t.Error("Should clear pending auth after approval")
		}
		if !strings.Contains(updated.message, "Approved connection") {
			t.Errorf("Expected approval message, got %q", updated.message)
		}
	})

	t.Run("denies auth with N key", func(t *testing.T) {
		model := NewInlineServerModel(52525, "test-server")
		responseChan := make(chan bool, 1)
		model.pendingAuth = &SSHAuthRequestMsg{
			ClientAddr:   "192.168.1.100:12345",
			Fingerprint:  "SHA256:abc123",
			ResponseChan: responseChan,
		}
		model.authChannel = responseChan
		
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		updated := updatedModel.(*InlineServerModel)

		// Check denial was sent
		select {
		case approved := <-responseChan:
			if approved {
				t.Error("Should have sent denial")
			}
		default:
			t.Error("Should have sent denial to channel")
		}

		if updated.pendingAuth != nil {
			t.Error("Should clear pending auth after denial")
		}
		if !strings.Contains(updated.message, "Denied connection") {
			t.Errorf("Expected denial message, got %q", updated.message)
		}
	})
}