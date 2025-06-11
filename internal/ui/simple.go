package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Simple models for a cleaner UI

// SimpleClientModel is a simplified client UI
type SimpleClientModel struct {
	serverAddress string
	serverName    string
	connected     bool
	capturing     bool
	width         int
	height        int
	err           error
}

// NewSimpleClientModel creates a new simple client model
func NewSimpleClientModel(serverAddress, serverName string) *SimpleClientModel {
	return &SimpleClientModel{
		serverAddress: serverAddress,
		serverName:    serverName,
		connected:     false,
		capturing:     false,
	}
}

func (m *SimpleClientModel) Init() tea.Cmd {
	return nil
}

func (m *SimpleClientModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case " ":
			if m.connected {
				m.capturing = !m.capturing
			}
		case "r":
			// TODO: Reconnect
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m *SimpleClientModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Title
	title := TitleStyle.Render("Waymon Client")
	
	// Connection status
	var status string
	if m.connected {
		status = fmt.Sprintf("✓ Connected to %s", m.serverAddress)
		status = SuccessStyle.Render(status)
	} else {
		status = fmt.Sprintf("✗ Disconnected from server")
		status = ErrorStyle.Render(status)
	}
	
	// Capture status
	var captureStatus string
	if !m.connected {
		captureStatus = MutedStyle.Render("Connect to server to enable capture")
	} else if m.capturing {
		captureStatus = SuccessStyle.Render("✓ Mouse capture active")
	} else {
		captureStatus = MutedStyle.Render("○ Mouse capture inactive")
	}
	
	// Controls
	controls := []string{
		"[Space] Toggle capture",
		"[r] Reconnect",
		"[q] Quit",
	}
	controlsText := MutedStyle.Render(strings.Join(controls, "  •  "))
	
	// Build the view
	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		status,
		"",
		captureStatus,
		"",
		"",
		controlsText,
	)
	
	// Center everything
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

// SimpleServerModel is a simplified server UI
type SimpleServerModel struct {
	port        int
	name        string
	listening   bool
	clients     []string
	width       int
	height      int
}

// NewSimpleServerModel creates a new simple server model
func NewSimpleServerModel(port int, name string) *SimpleServerModel {
	return &SimpleServerModel{
		port:      port,
		name:      name,
		listening: true,
		clients:   []string{},
	}
}

func (m *SimpleServerModel) Init() tea.Cmd {
	return nil
}

func (m *SimpleServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m *SimpleServerModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Title
	title := TitleStyle.Render("Waymon Server")
	
	// Server info
	info := fmt.Sprintf("Listening on port %d", m.port)
	if m.name != "" {
		info = fmt.Sprintf("%s (%s)", info, m.name)
	}
	info = InfoStyle.Render(info)
	
	// Client list
	var clientsText string
	if len(m.clients) == 0 {
		clientsText = MutedStyle.Render("No clients connected")
	} else {
		clientsList := []string{"Connected clients:"}
		for _, client := range m.clients {
			clientsList = append(clientsList, fmt.Sprintf("  • %s", client))
		}
		clientsText = strings.Join(clientsList, "\n")
	}
	
	// Controls
	controls := MutedStyle.Render("[q] Quit")
	
	// Build the view
	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		info,
		"",
		clientsText,
		"",
		"",
		controls,
	)
	
	// Center everything
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}