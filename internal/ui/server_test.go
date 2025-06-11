package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestServerModel(t *testing.T) {
	t.Run("creates new server model", func(t *testing.T) {
		cfg := ServerConfig{
			Port: 52525,
			Name: "test-server",
			Monitors: []Monitor{
				{Name: "DP-1", Size: "1920x1080", Position: "0,0", Primary: true},
			},
		}
		
		model := NewServerModel(cfg)
		
		if model.port != 52525 {
			t.Errorf("Expected port 52525, got %d", model.port)
		}
		if model.name != "test-server" {
			t.Errorf("Expected name 'test-server', got %q", model.name)
		}
		if len(model.monitors.Monitors) != 1 {
			t.Errorf("Expected 1 monitor, got %d", len(model.monitors.Monitors))
		}
	})

	t.Run("renders initial view", func(t *testing.T) {
		cfg := ServerConfig{
			Port: 52525,
			Name: "test-server",
			Monitors: []Monitor{
				{Name: "DP-1", Size: "1920x1080", Position: "0,0", Primary: true},
			},
		}
		
		model := NewServerModel(cfg)
		model.width = 80
		model.height = 30
		
		view := model.View()
		
		// Check for key elements
		if !strings.Contains(view, "Waymon Server") {
			t.Error("Should contain 'Waymon Server'")
		}
		if !strings.Contains(view, "test-server") {
			t.Error("Should contain server name")
		}
		if !strings.Contains(view, "Listening on port 52525") {
			t.Error("Should show listening port")
		}
		if !strings.Contains(view, "Connected Clients") {
			t.Error("Should have Connected Clients section")
		}
	})

	t.Run("handles client connections", func(t *testing.T) {
		cfg := ServerConfig{Port: 52525, Name: "test-server"}
		model := NewServerModel(cfg)
		
		// Add a connection
		model.AddConnection("laptop", "192.168.1.100")
		
		if len(model.connections.Connections) != 1 {
			t.Error("Should have 1 connection")
		}
		
		conn := model.connections.Connections[0]
		if conn.Name != "laptop" {
			t.Errorf("Expected name 'laptop', got %q", conn.Name)
		}
		if !conn.Connected {
			t.Error("Connection should be marked as connected")
		}
		
		// Remove connection
		model.RemoveConnection("laptop")
		
		if len(model.connections.Connections) != 0 {
			t.Error("Should have 0 connections after removal")
		}
	})

	t.Run("handles messages", func(t *testing.T) {
		cfg := ServerConfig{Port: 52525, Name: "test-server"}
		model := NewServerModel(cfg)
		
		model.AddMessage(MessageSuccess, "Test success")
		model.AddMessage(MessageError, "Test error")
		
		if len(model.messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(model.messages))
		}
		
		if model.messages[0].Type != MessageSuccess {
			t.Error("First message should be success type")
		}
		if model.messages[1].Type != MessageError {
			t.Error("Second message should be error type")
		}
	})

	t.Run("handles keyboard input", func(t *testing.T) {
		cfg := ServerConfig{Port: 52525, Name: "test-server"}
		model := NewServerModel(cfg)
		
		// Test quit
		newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		serverModel := newModel.(*ServerModel)
		
		if !serverModel.quitting {
			t.Error("Should be quitting after 'q' key")
		}
		if cmd == nil {
			t.Error("Should return quit command")
		}
		
		// Test clear messages
		model.AddMessage(MessageInfo, "Test")
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
		
		if len(model.messages) != 0 {
			t.Error("Messages should be cleared after 'c' key")
		}
	})

	t.Run("handles client connected message", func(t *testing.T) {
		cfg := ServerConfig{Port: 52525, Name: "test-server"}
		model := NewServerModel(cfg)
		
		msg := ClientConnectedMsg{
			Name:    "laptop",
			Address: "192.168.1.100",
		}
		
		model.Update(msg)
		
		if len(model.connections.Connections) != 1 {
			t.Error("Should have added connection")
		}
		if len(model.messages) == 0 {
			t.Error("Should have added a message")
		}
	})

	t.Run("handles client disconnected message", func(t *testing.T) {
		cfg := ServerConfig{Port: 52525, Name: "test-server"}
		model := NewServerModel(cfg)
		
		// First add a connection
		model.AddConnection("laptop", "192.168.1.100")
		
		// Then disconnect
		msg := ClientDisconnectedMsg{Name: "laptop"}
		model.Update(msg)
		
		if len(model.connections.Connections) != 0 {
			t.Error("Should have removed connection")
		}
		if len(model.messages) == 0 {
			t.Error("Should have added a disconnect message")
		}
	})

	t.Run("handles window resize", func(t *testing.T) {
		cfg := ServerConfig{Port: 52525, Name: "test-server"}
		model := NewServerModel(cfg)
		
		model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
		
		if model.width != 100 {
			t.Errorf("Expected width 100, got %d", model.width)
		}
		if model.height != 40 {
			t.Errorf("Expected height 40, got %d", model.height)
		}
	})
}