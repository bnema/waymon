package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InlineClientModel represents the inline UI model for the client
type InlineClientModel struct {
	connected      bool
	waitingApproval bool
	capturing      bool
	serverAddr     string
	lastUpdate     time.Time
	spinner        spinner.Model
	message        string
	messageType    string // "info", "error", "success"
	messageExpiry  time.Time
}

// NewInlineClientModel creates a new inline client UI model
func NewInlineClientModel(serverAddr string) *InlineClientModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	
	return &InlineClientModel{
		serverAddr: serverAddr,
		spinner:    s,
		lastUpdate: time.Now(),
	}
}

// Init initializes the inline client model
func (m *InlineClientModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles messages for the inline client model
func (m *InlineClientModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle messages
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case " ":
			m.capturing = !m.capturing
			if m.capturing {
				m.SetMessage("success", "Mouse capture enabled")
			} else {
				m.SetMessage("info", "Mouse capture disabled")
			}
		case "r":
			m.SetMessage("info", "Reconnecting...")
			// Trigger reconnection logic here
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		// Handle resize if needed

	case spinner.TickMsg:
		if !m.connected || m.waitingApproval {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case ConnectedMsg:
		m.connected = true
		m.waitingApproval = false
		m.SetMessage("success", "Connected to server")

	case DisconnectedMsg:
		m.connected = false
		m.waitingApproval = false
		m.capturing = false
		m.SetMessage("error", "Disconnected from server")
		
	case WaitingApprovalMsg:
		m.waitingApproval = true
		m.SetMessage("info", "Waiting for server approval...")

	case CaptureStartMsg:
		m.capturing = true
		m.SetMessage("info", "Mouse capture started")

	case CaptureStopMsg:
		m.capturing = false
		m.SetMessage("info", "Mouse capture stopped")
	}

	// Clear expired messages
	if !m.messageExpiry.IsZero() && time.Now().After(m.messageExpiry) {
		m.message = ""
		m.messageType = ""
		m.messageExpiry = time.Time{}
	}

	m.lastUpdate = time.Now()
	return m, tea.Batch(cmds...)
}

// View renders the inline client UI
func (m *InlineClientModel) View() string {
	var parts []string

	// App name
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	parts = append(parts, nameStyle.Render("WAYMON"))

	// Connection status
	if m.connected {
		connStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		parts = append(parts, connStyle.Render("● Connected"))
	} else if m.waitingApproval {
		connStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		parts = append(parts, connStyle.Render(m.spinner.View()+" Waiting for approval"))
	} else {
		connStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		parts = append(parts, connStyle.Render(m.spinner.View()+" Connecting"))
	}

	// Server address
	addrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
	parts = append(parts, addrStyle.Render(m.serverAddr))

	// Capture status
	if m.capturing {
		capStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
		parts = append(parts, capStyle.Render("▶ CAPTURING"))
	} else {
		capStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		parts = append(parts, capStyle.Render("■ Idle"))
	}

	// Message (if any)
	if m.message != "" {
		var msgStyle lipgloss.Style
		switch m.messageType {
		case "error":
			msgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		case "success":
			msgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		default:
			msgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
		}
		parts = append(parts, msgStyle.Render(m.message))
	}

	// Controls hint
	controlsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	controls := "[space] capture • [r] reconnect • [q] quit"
	parts = append(parts, controlsStyle.Render(controls))

	// Join with separators
	separator := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render(" │ ")
	return strings.Join(parts, separator)
}

// SetMessage sets a temporary message
func (m *InlineClientModel) SetMessage(msgType, message string) {
	m.message = message
	m.messageType = msgType
	m.messageExpiry = time.Now().Add(3 * time.Second)
}

// InlineServerModel represents the inline UI model for the server
type InlineServerModel struct {
	port          int
	serverName    string
	clientCount   int
	lastUpdate    time.Time
	spinner       spinner.Model
	message       string
	messageType   string
	messageExpiry time.Time
	
	// SSH auth approval
	pendingAuth   *SSHAuthRequestMsg
	authChannel   chan bool // Send approval decision back
}

// NewInlineServerModel creates a new inline server UI model
func NewInlineServerModel(port int, serverName string) *InlineServerModel {
	s := spinner.New()
	s.Spinner = spinner.Globe
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	
	return &InlineServerModel{
		port:       port,
		serverName: serverName,
		spinner:    s,
		lastUpdate: time.Now(),
	}
}

// Init initializes the inline server model
func (m *InlineServerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles messages for the inline server model
func (m *InlineServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle auth approval if pending
		if m.pendingAuth != nil {
			switch msg.String() {
			case "y", "Y":
				if m.authChannel != nil {
					select {
					case m.authChannel <- true:
					default:
					}
				}
				m.SetMessage("success", fmt.Sprintf("Approved connection from %s", m.pendingAuth.ClientAddr))
				m.pendingAuth = nil
				m.authChannel = nil
			case "n", "N":
				if m.authChannel != nil {
					select {
					case m.authChannel <- false:
					default:
					}
				}
				m.SetMessage("info", fmt.Sprintf("Denied connection from %s", m.pendingAuth.ClientAddr))
				m.pendingAuth = nil
				m.authChannel = nil
			}
			return m, nil
		}
		
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case SSHAuthRequestMsg:
		m.pendingAuth = &msg
		m.authChannel = msg.ResponseChan
		// Don't set a message, we'll show the prompt in the View

	case ClientConnectedMsg:
		m.clientCount++
		m.SetMessage("success", fmt.Sprintf("Client connected (total: %d)", m.clientCount))

	case ClientDisconnectedMsg:
		if m.clientCount > 0 {
			m.clientCount--
		}
		m.SetMessage("info", fmt.Sprintf("Client disconnected (remaining: %d)", m.clientCount))
	}

	// Clear expired messages only if no auth pending
	if m.pendingAuth == nil && !m.messageExpiry.IsZero() && time.Now().After(m.messageExpiry) {
		m.message = ""
		m.messageType = ""
		m.messageExpiry = time.Time{}
	}

	m.lastUpdate = time.Now()
	return m, tea.Batch(cmds...)
}

// View renders the inline server UI
func (m *InlineServerModel) View() string {
	// If there's a pending auth request, show that instead
	if m.pendingAuth != nil {
		warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
		addrStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
		promptStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
		
		// Build multi-line auth prompt with proper line clearing
		lines := []string{
			"\r\033[K" + warnStyle.Render("⚠️  NEW CONNECTION:") + " " + addrStyle.Render(m.pendingAuth.ClientAddr),
			"\r\033[K" + keyStyle.Render("SSH Key:") + " " + keyStyle.Render(m.pendingAuth.Fingerprint),
			"\r\033[K" + promptStyle.Render("Allow this connection? [Y/n]") + " ",
		}
		
		return strings.Join(lines, "\n")
	}

	var parts []string

	// App name
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	parts = append(parts, nameStyle.Render("WAYMON SERVER"))

	// Listening status
	listenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	parts = append(parts, listenStyle.Render(m.spinner.View()+" Listening"))

	// Port
	portStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
	parts = append(parts, portStyle.Render(fmt.Sprintf(":%d", m.port)))

	// Client count
	var clientStyle lipgloss.Style
	if m.clientCount > 0 {
		clientStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		parts = append(parts, clientStyle.Render(fmt.Sprintf("%d client%s", m.clientCount, pluralize(m.clientCount))))
	} else {
		clientStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		parts = append(parts, clientStyle.Render("No clients"))
	}

	// Message (if any)
	if m.message != "" {
		var msgStyle lipgloss.Style
		switch m.messageType {
		case "error":
			msgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		case "success":
			msgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		default:
			msgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
		}
		parts = append(parts, msgStyle.Render(m.message))
	}

	// Controls hint
	controlsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	parts = append(parts, controlsStyle.Render("[q] quit"))

	// Join with separators
	separator := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render(" │ ")
	return strings.Join(parts, separator)
}

// SetMessage sets a temporary message
func (m *InlineServerModel) SetMessage(msgType, message string) {
	m.message = message
	m.messageType = msgType
	m.messageExpiry = time.Now().Add(3 * time.Second)
}

// GetAuthChannel returns the current auth channel for approval responses
func (m *InlineServerModel) GetAuthChannel() chan bool {
	return m.authChannel
}

// Helper functions
func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}


// Message types for reactive updates
type (
	ConnectedMsg          struct{}
	DisconnectedMsg       struct{}
	WaitingApprovalMsg    struct{}
	CaptureStartMsg       struct{}
	CaptureStopMsg        struct{}
	ClientConnectedMsg    struct{ ClientAddr string }
	ClientDisconnectedMsg struct{ ClientAddr string }
	SSHAuthRequestMsg     struct{ 
		ClientAddr   string 
		PublicKey    string
		Fingerprint  string
		ResponseChan chan bool
	}
	SSHAuthApprovedMsg    struct{ Fingerprint string }
	SSHAuthDeniedMsg      struct{ Fingerprint string }
)