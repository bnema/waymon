package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestClientModel(t *testing.T) {
	t.Run("creates new client model", func(t *testing.T) {
		cfg := ClientConfig{
			ServerAddress: "192.168.1.50:52525",
			ServerName:    "desktop",
			EdgeThreshold: 5,
		}

		model := NewClientModel(cfg)

		if model.serverAddress != "192.168.1.50:52525" {
			t.Errorf("Expected server address '192.168.1.50:52525', got %q", model.serverAddress)
		}
		if model.serverName != "desktop" {
			t.Errorf("Expected server name 'desktop', got %q", model.serverName)
		}
		if model.connected {
			t.Error("Should not be connected initially")
		}
		if model.capturing {
			t.Error("Should not be capturing initially")
		}
	})

	t.Run("renders initial view", func(t *testing.T) {
		cfg := ClientConfig{
			ServerAddress: "192.168.1.50:52525",
			ServerName:    "desktop",
			EdgeThreshold: 5,
		}

		model := NewClientModel(cfg)
		model.width = 80
		model.height = 30

		view := model.View()

		// Check for key elements
		if !strings.Contains(view, "Waymon Client") {
			t.Error("Should contain 'Waymon Client'")
		}
		if !strings.Contains(view, "192.168.1.50:52525") {
			t.Error("Should show server address")
		}
		if !strings.Contains(view, "Mouse capture inactive") {
			t.Error("Should show capture status")
		}
		if !strings.Contains(view, "Controls:") {
			t.Error("Should show controls")
		}
	})

	t.Run("toggles mouse capture", func(t *testing.T) {
		cfg := ClientConfig{ServerAddress: "192.168.1.50:52525"}
		model := NewClientModel(cfg)

		// Connect first so we can toggle capture
		model.connected = true

		// Toggle on
		model.ToggleCapture()

		if !model.capturing {
			t.Error("Should be capturing after toggle")
		}
		if len(model.messages) == 0 {
			t.Error("Should have added a message")
		}

		// Toggle off
		model.ToggleCapture()

		if model.capturing {
			t.Error("Should not be capturing after second toggle")
		}
	})

	t.Run("prevents capture when disconnected", func(t *testing.T) {
		cfg := ClientConfig{ServerAddress: "192.168.1.50:52525"}
		model := NewClientModel(cfg)

		// Disconnect first
		model.connected = false

		// Try to toggle capture
		model.ToggleCapture()

		if model.capturing {
			t.Error("Should not allow capture when disconnected")
		}

		// Check for warning message
		found := false
		for _, msg := range model.messages {
			if msg.Type == MessageWarning {
				found = true
				break
			}
		}
		if !found {
			t.Error("Should have added a warning message")
		}
	})

	t.Run("handles keyboard input", func(t *testing.T) {
		cfg := ClientConfig{ServerAddress: "192.168.1.50:52525"}
		model := NewClientModel(cfg)

		// Test quit
		newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		clientModel := newModel.(*ClientModel)

		if !clientModel.quitting {
			t.Error("Should be quitting after 'q' key")
		}
		if cmd == nil {
			t.Error("Should return quit command")
		}

		// Test space (toggle capture) - should not work when disconnected
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

		if model.capturing {
			t.Error("Should not toggle capture when not connected")
		}

		// Test edge hint toggle
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})

		if model.edgeIndicator.Visible {
			t.Error("Should toggle edge indicator visibility")
		}
	})

	t.Run("handles server disconnect", func(t *testing.T) {
		cfg := ClientConfig{ServerAddress: "192.168.1.50:52525"}
		model := NewClientModel(cfg)
		model.connected = true // Need to be connected first
		model.capturing = true

		model.Update(ServerDisconnectedMsg{})

		if model.connected {
			t.Error("Should be disconnected")
		}
		if model.capturing {
			t.Error("Should stop capturing on disconnect")
		}
		if model.statusBar.Connected {
			t.Error("Status bar should show disconnected")
		}

		// Check for error message
		found := false
		for _, msg := range model.messages {
			if msg.Type == MessageError {
				found = true
				break
			}
		}
		if !found {
			t.Error("Should have added an error message")
		}
	})

	t.Run("handles server reconnect", func(t *testing.T) {
		cfg := ClientConfig{ServerAddress: "192.168.1.50:52525"}
		model := NewClientModel(cfg)
		model.connected = false

		model.Update(ServerReconnectedMsg{})

		if !model.connected {
			t.Error("Should be connected")
		}
		if !model.statusBar.Connected {
			t.Error("Status bar should show connected")
		}

		// Check for success message
		found := false
		for _, msg := range model.messages {
			if msg.Type == MessageSuccess {
				found = true
				break
			}
		}
		if !found {
			t.Error("Should have added a success message")
		}
	})

	t.Run("handles edge detection", func(t *testing.T) {
		cfg := ClientConfig{ServerAddress: "192.168.1.50:52525"}
		model := NewClientModel(cfg)

		model.Update(EdgeDetectedMsg{Edge: "left"})

		if model.currentEdge != "left" {
			t.Errorf("Expected current edge 'left', got %q", model.currentEdge)
		}
		if model.edgeIndicator.Edge != "left" {
			t.Error("Edge indicator should be updated")
		}
	})

	t.Run("handles mouse switch", func(t *testing.T) {
		cfg := ClientConfig{
			ServerAddress: "192.168.1.50:52525",
			ServerName:    "desktop",
		}
		model := NewClientModel(cfg)

		// Switch to server
		model.Update(MouseSwitchedMsg{ToServer: true})

		found := false
		for _, msg := range model.messages {
			if strings.Contains(msg.Content, "desktop") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Should have message about switching to server")
		}

		// Switch back to client
		model.messages = []Message{} // Clear messages
		model.Update(MouseSwitchedMsg{ToServer: false})

		found = false
		for _, msg := range model.messages {
			if strings.Contains(msg.Content, "returned") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Should have message about returning to client")
		}
	})

	t.Run("updates capture status correctly", func(t *testing.T) {
		cfg := ClientConfig{ServerAddress: "192.168.1.50:52525"}
		model := NewClientModel(cfg)

		// Test disconnected state
		model.connected = false
		model.UpdateCaptureStatus()

		if !strings.Contains(model.captureStatus.Content[0], "Disconnected") {
			t.Error("Should show disconnected status")
		}

		// Test connected but not capturing
		model.connected = true
		model.capturing = false
		model.UpdateCaptureStatus()

		if !strings.Contains(model.captureStatus.Content[0], "inactive") {
			t.Error("Should show inactive status")
		}

		// Test capturing
		model.capturing = true
		model.UpdateCaptureStatus()

		if !strings.Contains(model.captureStatus.Content[0], "active") {
			t.Error("Should show active status")
		}
	})
}

func TestEdgeIndicator(t *testing.T) {
	t.Run("creates new edge indicator", func(t *testing.T) {
		ei := NewEdgeIndicator()

		if !ei.Visible {
			t.Error("Should be visible by default")
		}
		if ei.Edge != "" {
			t.Error("Should have no edge initially")
		}
	})

	t.Run("sets edge", func(t *testing.T) {
		ei := NewEdgeIndicator()
		ei.SetEdge("top")

		if ei.Edge != "top" {
			t.Errorf("Expected edge 'top', got %q", ei.Edge)
		}
	})

	t.Run("renders edge indicator", func(t *testing.T) {
		ei := NewEdgeIndicator()
		ei.Width = 40
		ei.Height = 20
		ei.SetEdge("left")

		view := ei.View()

		if !strings.Contains(view, "Edge Detection") {
			t.Error("Should contain 'Edge Detection'")
		}
		if !strings.Contains(view, "left") {
			t.Error("Should show active edge")
		}
	})

	t.Run("returns empty when not visible", func(t *testing.T) {
		ei := NewEdgeIndicator()
		ei.Visible = false
		ei.SetEdge("top")

		view := ei.View()

		if view != "" {
			t.Error("Should return empty string when not visible")
		}
	})
}
