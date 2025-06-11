package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ClientModel is the Bubble Tea model for client UI
type ClientModel struct {
	statusBar      *StatusBar
	serverInfo     *InfoPanel
	captureStatus  *InfoPanel
	controls       *ControlsHelp
	edgeIndicator  *EdgeIndicator
	messages       []Message
	width          int
	height         int
	serverAddress  string
	serverName     string
	capturing      bool
	connected      bool
	quitting       bool
	currentEdge    string
}

// ClientConfig holds configuration for the client UI
type ClientConfig struct {
	ServerAddress string
	ServerName    string
	EdgeThreshold int
}

// NewClientModel creates a new client UI model
func NewClientModel(cfg ClientConfig) *ClientModel {
	statusBar := NewStatusBar("Waymon Client")
	statusBar.Status = fmt.Sprintf("Connecting to %s...", cfg.ServerAddress)
	statusBar.Connected = false
	statusBar.ShowSpinner = true

	serverInfo := &InfoPanel{
		Title: "Server Information",
		Content: []string{
			fmt.Sprintf("Name: %s", cfg.ServerName),
			fmt.Sprintf("Address: %s", cfg.ServerAddress),
			fmt.Sprintf("Edge threshold: %d pixels", cfg.EdgeThreshold),
		},
	}

	return &ClientModel{
		statusBar:     statusBar,
		serverInfo:    serverInfo,
		serverAddress: cfg.ServerAddress,
		serverName:    cfg.ServerName,
		connected:     false,
		captureStatus: &InfoPanel{
			Title:   "Capture Status",
			Content: []string{"Mouse capture inactive"},
		},
		controls: &ControlsHelp{
			Controls: []Control{
				{Key: "Space", Desc: "Toggle mouse capture"},
				{Key: "h", Desc: "Toggle edge hint"},
				{Key: "r", Desc: "Reconnect to server"},
				{Key: "q", Desc: "Quit"},
			},
		},
		edgeIndicator: NewEdgeIndicator(),
		messages:      []Message{},
	}
}

// Init implements tea.Model
func (m *ClientModel) Init() tea.Cmd {
	return m.statusBar.Init()
}

// Update implements tea.Model
func (m *ClientModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case " ", "space":
			m.ToggleCapture()
		case "h":
			m.edgeIndicator.Visible = !m.edgeIndicator.Visible
		case "r":
			if !m.connected {
				m.AddMessage(MessageInfo, "Attempting to reconnect...")
				// Trigger reconnection
			}
		case "c":
			m.messages = []Message{}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.statusBar.Width = msg.Width
		m.serverInfo.Width = msg.Width
		m.captureStatus.Width = msg.Width
		m.controls.Width = msg.Width
		m.edgeIndicator.Width = msg.Width
		m.edgeIndicator.Height = msg.Height

	case ServerDisconnectedMsg:
		m.connected = false
		m.capturing = false
		m.statusBar.Connected = false
		m.statusBar.Status = "Disconnected from server"
		m.UpdateCaptureStatus()
		m.AddMessage(MessageError, "Lost connection to server")

	case ServerReconnectedMsg:
		m.connected = true
		m.statusBar.Connected = true
		m.statusBar.Status = fmt.Sprintf("Connected to %s", m.serverAddress)
		m.AddMessage(MessageSuccess, "Reconnected to server")

	case EdgeDetectedMsg:
		m.currentEdge = msg.Edge
		m.edgeIndicator.SetEdge(msg.Edge)
		
	case MouseSwitchedMsg:
		if msg.ToServer {
			m.AddMessage(MessageInfo, fmt.Sprintf("Mouse switched to %s", m.serverName))
		} else {
			m.AddMessage(MessageInfo, "Mouse returned to client")
		}
	}

	// Update status bar
	statusBar, cmd := m.statusBar.Update(msg)
	m.statusBar = statusBar

	return m, cmd
}

// View implements tea.Model
func (m *ClientModel) View() string {
	if m.quitting {
		return MutedStyle.Render("Disconnecting from server...\n")
	}

	var sections []string

	// Header
	header := HeaderStyle.Render("Waymon Client")
	sections = append(sections, header)

	// Status bar
	sections = append(sections, m.statusBar.View())

	// Server info
	if m.height > 15 {
		sections = append(sections, m.serverInfo.View())
	}

	// Capture status with visual indicator
	sections = append(sections, m.captureStatus.View())

	// Edge indicator (if visible and capturing)
	if m.edgeIndicator.Visible && m.capturing && m.height > 20 {
		sections = append(sections, m.edgeIndicator.View())
	}

	// Messages (if any)
	if len(m.messages) > 0 && m.height > 25 {
		var msgSection strings.Builder
		msgSection.WriteString(SubheaderStyle.Render("Activity:"))
		msgSection.WriteString("\n\n")
		
		// Show last 3 messages
		start := 0
		if len(m.messages) > 3 {
			start = len(m.messages) - 3
		}
		
		for _, msg := range m.messages[start:] {
			msgSection.WriteString(msg.View())
			msgSection.WriteString("\n")
		}
		
		sections = append(sections, BoxStyle.Width(m.width).Render(msgSection.String()))
	}

	// Controls (at the bottom)
	sections = append(sections, m.controls.View())

	// Hint when capturing
	if m.capturing {
		hint := ItalicStyle.Copy().
			Foreground(ColorInfo).
			Render("Move mouse to screen edge to switch computers")
		sections = append(sections, Center(m.width, hint))
	}

	// Join sections with spacing
	return lipgloss.JoinVertical(lipgloss.Top, sections...)
}

// ToggleCapture toggles mouse capture mode
func (m *ClientModel) ToggleCapture() {
	if !m.connected {
		m.AddMessage(MessageWarning, "Cannot capture - not connected to server")
		return
	}
	
	m.capturing = !m.capturing
	m.UpdateCaptureStatus()
	
	if m.capturing {
		m.AddMessage(MessageSuccess, "Mouse capture enabled")
	} else {
		m.AddMessage(MessageInfo, "Mouse capture disabled")
	}
}

// UpdateCaptureStatus updates the capture status panel
func (m *ClientModel) UpdateCaptureStatus() {
	var content []string
	
	if !m.connected {
		content = append(content, ErrorStyle.Render("✗ Disconnected from server"))
	} else if m.capturing {
		content = append(content, SuccessStyle.Render("✓ Mouse capture active"))
		content = append(content, "")
		content = append(content, "Move mouse to screen edges to switch")
	} else {
		content = append(content, MutedStyle.Render("○ Mouse capture inactive"))
		content = append(content, "")
		content = append(content, "Press Space to enable")
	}
	
	m.captureStatus.Content = content
}

// AddMessage adds a message to the activity log
func (m *ClientModel) AddMessage(msgType MessageType, content string) {
	m.messages = append(m.messages, Message{
		Type:    msgType,
		Content: content,
	})
}

// EdgeIndicator shows which edge is active
type EdgeIndicator struct {
	Width   int
	Height  int
	Edge    string
	Visible bool
}

// NewEdgeIndicator creates a new edge indicator
func NewEdgeIndicator() *EdgeIndicator {
	return &EdgeIndicator{
		Visible: true,
	}
}

// SetEdge updates the active edge
func (e *EdgeIndicator) SetEdge(edge string) {
	e.Edge = edge
}

// View renders the edge indicator
func (e *EdgeIndicator) View() string {
	if !e.Visible || e.Edge == "" {
		return ""
	}
	
	var b strings.Builder
	b.WriteString(SubheaderStyle.Render("Edge Detection:"))
	b.WriteString("\n\n")
	
	// Visual representation of edges
	top := "───"
	bottom := "───"
	left := "│"
	right := "│"
	
	// Highlight active edge
	activeStyle := lipgloss.NewStyle().Foreground(ColorActive)
	switch e.Edge {
	case "top":
		top = activeStyle.Render("━━━")
	case "bottom":
		bottom = activeStyle.Render("━━━")
	case "left":
		left = activeStyle.Render("┃")
	case "right":
		right = activeStyle.Render("┃")
	}
	
	// Simple box representation
	b.WriteString(fmt.Sprintf("    %s\n", top))
	b.WriteString(fmt.Sprintf("  %s   %s\n", left, right))
	b.WriteString(fmt.Sprintf("    %s\n", bottom))
	
	if e.Edge != "" {
		b.WriteString("\n")
		b.WriteString(InfoStyle.Render(fmt.Sprintf("Active: %s edge", e.Edge)))
	}
	
	return BoxStyle.Width(e.Width).Render(b.String())
}

// Custom messages for client events
type ServerDisconnectedMsg struct{}
type ServerReconnectedMsg struct{}
type EdgeDetectedMsg struct{ Edge string }
type MouseSwitchedMsg struct{ ToServer bool }