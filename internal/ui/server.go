package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ServerModel is the Bubble Tea model for server UI
type ServerModel struct {
	statusBar   *StatusBar
	connections *ConnectionList
	monitors    *MonitorInfo
	controls    *ControlsHelp
	messages    []Message
	width       int
	height      int
	port        int
	name        string
	quitting    bool
}

// ServerConfig holds configuration for the server UI
type ServerConfig struct {
	Port     int
	Name     string
	Monitors []Monitor
}

// NewServerModel creates a new server UI model
func NewServerModel(cfg ServerConfig) *ServerModel {
	statusBar := NewStatusBar("Waymon Server")
	statusBar.Status = fmt.Sprintf("Listening on port %d", cfg.Port)
	statusBar.Connected = true

	return &ServerModel{
		statusBar: statusBar,
		port:      cfg.Port,
		name:      cfg.Name,
		connections: &ConnectionList{
			Title:       "Connected Clients",
			Connections: []Connection{},
		},
		monitors: &MonitorInfo{
			Monitors: cfg.Monitors,
		},
		controls: &ControlsHelp{
			Controls: []Control{
				{Key: "q", Desc: "Quit server"},
				{Key: "c", Desc: "Clear messages"},
				{Key: "r", Desc: "Refresh display"},
			},
		},
		messages: []Message{},
	}
}

// Init implements tea.Model
func (m *ServerModel) Init() tea.Cmd {
	return m.statusBar.Init()
}

// Update implements tea.Model
func (m *ServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "c":
			m.messages = []Message{}
		case "r":
			m.AddMessage(MessageInfo, "Display refreshed")
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.statusBar.Width = msg.Width
		m.connections.Width = msg.Width
		m.monitors.Width = msg.Width
		m.controls.Width = msg.Width

	case ClientConnectedMsg:
		// Extract name from address (e.g., "192.168.1.100:12345" -> "192.168.1.100")
		name := msg.ClientAddr
		if idx := strings.LastIndex(name, ":"); idx != -1 {
			name = name[:idx]
		}
		m.AddConnection(name, msg.ClientAddr)
		m.AddMessage(MessageSuccess, fmt.Sprintf("Client connected from %s", msg.ClientAddr))

	case ClientDisconnectedMsg:
		// Extract name from address
		name := msg.ClientAddr
		if idx := strings.LastIndex(name, ":"); idx != -1 {
			name = name[:idx]
		}
		m.RemoveConnection(name)
		m.AddMessage(MessageInfo, fmt.Sprintf("Client disconnected from %s", msg.ClientAddr))
	}

	// Update status bar
	statusBar, cmd := m.statusBar.Update(msg)
	m.statusBar = statusBar

	return m, cmd
}

// View implements tea.Model
func (m *ServerModel) View() string {
	if m.quitting {
		return MutedStyle.Render("Shutting down server...\n")
	}

	var sections []string

	// Header
	header := HeaderStyle.Render(fmt.Sprintf("Waymon Server - %s", m.name))
	sections = append(sections, header)

	// Status bar
	sections = append(sections, m.statusBar.View())

	// Monitor info (only show on larger screens)
	if m.height > 20 {
		sections = append(sections, m.monitors.View())
	}

	// Connections
	sections = append(sections, m.connections.View())

	// Messages (if any)
	if len(m.messages) > 0 && m.height > 25 {
		var msgSection strings.Builder
		msgSection.WriteString(SubheaderStyle.Render("Recent Activity:"))
		msgSection.WriteString("\n\n")

		// Show last 5 messages
		start := 0
		if len(m.messages) > 5 {
			start = len(m.messages) - 5
		}

		for _, msg := range m.messages[start:] {
			msgSection.WriteString(msg.View())
			msgSection.WriteString("\n")
		}

		sections = append(sections, BoxStyle.Width(m.width).Render(msgSection.String()))
	}

	// Controls (at the bottom)
	sections = append(sections, m.controls.View())

	// Join sections with spacing
	return lipgloss.JoinVertical(lipgloss.Top, sections...)
}

// AddConnection adds a client connection
func (m *ServerModel) AddConnection(name, address string) {
	conn := Connection{
		Name:      name,
		Address:   address,
		Connected: true,
		Active:    false,
	}

	m.connections.Connections = append(m.connections.Connections, conn)
}

// RemoveConnection removes a client connection
func (m *ServerModel) RemoveConnection(name string) {
	var filtered []Connection
	for _, conn := range m.connections.Connections {
		if conn.Name != name {
			filtered = append(filtered, conn)
		}
	}
	m.connections.Connections = filtered
}

// AddMessage adds a message to the activity log
func (m *ServerModel) AddMessage(msgType MessageType, content string) {
	m.messages = append(m.messages, Message{
		Type:    msgType,
		Content: content,
	})
}

