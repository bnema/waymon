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
	
	// Log display
	logBuffer     []LogEntry
	maxLogLines   int
	windowHeight  int
	windowWidth   int
}

// NewInlineClientModel creates a new inline client UI model
func NewInlineClientModel(serverAddr string) *InlineClientModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	
	return &InlineClientModel{
		serverAddr:   serverAddr,
		spinner:      s,
		lastUpdate:   time.Now(),
		logBuffer:    make([]LogEntry, 0),
		maxLogLines:  50, // Keep last 50 log entries
		windowHeight: 24, // Default terminal height
		windowWidth:  80, // Default terminal width
	}
}

// AddLogEntry adds a new log entry to the client buffer
func (m *InlineClientModel) AddLogEntry(entry LogEntry) {
	m.logBuffer = append(m.logBuffer, entry)
	
	// Keep only the last maxLogLines entries
	if len(m.logBuffer) > m.maxLogLines {
		m.logBuffer = m.logBuffer[len(m.logBuffer)-m.maxLogLines:]
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
	
	case tea.WindowSizeMsg:
		m.windowHeight = msg.Height
		m.windowWidth = msg.Width
	
	case LogMsg:
		m.AddLogEntry(msg.Entry)
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

// View renders the inline client UI with status bar + logs
func (m *InlineClientModel) View() string {
	var output strings.Builder
	
	// Calculate available space for logs
	statusBarHeight := 1
	waitingPromptHeight := 0
	if m.waitingApproval {
		waitingPromptHeight = 4 // Waiting prompt takes 4 lines
	}
	
	availableHeight := m.windowHeight - statusBarHeight - waitingPromptHeight - 1 // -1 for padding
	if availableHeight < 1 {
		availableHeight = 10 // Minimum height
	}
	
	// 1. Render status bar
	statusBar := m.renderClientStatusBar()
	output.WriteString(statusBar)
	output.WriteString("\n")
	
	// 2. If waiting for approval, show it
	if m.waitingApproval {
		waitingPrompt := m.renderWaitingPrompt()
		output.WriteString(waitingPrompt)
		output.WriteString("\n")
	}
	
	// 3. Render recent logs
	logView := m.renderClientLogs(availableHeight)
	output.WriteString(logView)
	
	return output.String()
}

// renderClientStatusBar renders the client status bar line
func (m *InlineClientModel) renderClientStatusBar() string {
	var parts []string

	// App name
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	parts = append(parts, nameStyle.Render("WAYMON"))

	// Connection status
	if m.connected {
		connStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		parts = append(parts, connStyle.Render("● Connected"))
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

	// Controls hint
	controlsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	controls := "[space] capture • [r] reconnect • [q] quit"
	parts = append(parts, controlsStyle.Render(controls))

	// Join with separators
	separator := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render(" │ ")
	return strings.Join(parts, separator)
}

// renderWaitingPrompt renders the waiting for approval prompt
func (m *InlineClientModel) renderWaitingPrompt() string {
	waitingStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	serverStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
	
	var prompt strings.Builder
	prompt.WriteString(waitingStyle.Render("⏳ WAITING FOR SERVER APPROVAL: "))
	prompt.WriteString(serverStyle.Render(m.serverAddr))
	prompt.WriteString("\n")
	prompt.WriteString(infoStyle.Render("The server administrator needs to approve your SSH key..."))
	prompt.WriteString("\n")
	prompt.WriteString(infoStyle.Render("Press [q] to quit or [r] to reconnect"))
	
	return prompt.String()
}

// renderClientLogs renders the recent log entries for client
func (m *InlineClientModel) renderClientLogs(maxLines int) string {
	if len(m.logBuffer) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		return dimStyle.Render("No logs yet...")
	}
	
	var logLines []string
	
	// Show the most recent logs that fit in the available space
	startIdx := 0
	if len(m.logBuffer) > maxLines {
		startIdx = len(m.logBuffer) - maxLines
	}
	
	for i := startIdx; i < len(m.logBuffer); i++ {
		entry := m.logBuffer[i]
		logLine := m.formatClientLogEntry(entry)
		logLines = append(logLines, logLine)
	}
	
	return strings.Join(logLines, "\n")
}

// formatClientLogEntry formats a single log entry with colors for client
func (m *InlineClientModel) formatClientLogEntry(entry LogEntry) string {
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	
	var levelStyle lipgloss.Style
	switch strings.ToUpper(entry.Level) {
	case "ERROR":
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	case "WARN", "WARNING":
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	case "INFO":
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	case "DEBUG":
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	default:
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
	}
	
	msgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	
	return fmt.Sprintf("%s %s %s",
		timeStyle.Render(entry.Timestamp.Format("15:04:05")),
		levelStyle.Render(fmt.Sprintf("%-5s", strings.ToUpper(entry.Level))),
		msgStyle.Render(entry.Message))
}

// SetMessage sets a temporary message
func (m *InlineClientModel) SetMessage(msgType, message string) {
	m.message = message
	m.messageType = msgType
	m.messageExpiry = time.Now().Add(3 * time.Second)
}

// LogEntry represents a single log entry with timestamp and content
type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
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
	
	// Log display
	logBuffer     []LogEntry
	maxLogLines   int
	windowHeight  int
	windowWidth   int
}

// NewInlineServerModel creates a new inline server UI model
func NewInlineServerModel(port int, serverName string) *InlineServerModel {
	s := spinner.New()
	s.Spinner = spinner.Globe
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	
	return &InlineServerModel{
		port:         port,
		serverName:   serverName,
		spinner:      s,
		lastUpdate:   time.Now(),
		logBuffer:    make([]LogEntry, 0),
		maxLogLines:  50, // Keep last 50 log entries
		windowHeight: 24, // Default terminal height
		windowWidth:  80, // Default terminal width
	}
}

// AddLogEntry adds a new log entry to the buffer
func (m *InlineServerModel) AddLogEntry(entry LogEntry) {
	m.logBuffer = append(m.logBuffer, entry)
	
	// Keep only the last maxLogLines entries
	if len(m.logBuffer) > m.maxLogLines {
		m.logBuffer = m.logBuffer[len(m.logBuffer)-m.maxLogLines:]
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
	
	case tea.WindowSizeMsg:
		m.windowHeight = msg.Height
		m.windowWidth = msg.Width
	
	case LogMsg:
		m.AddLogEntry(msg.Entry)
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

// View renders the inline server UI with status bar + logs
func (m *InlineServerModel) View() string {
	var output strings.Builder
	
	// Calculate available space for logs (leave room for status bar and auth prompt)
	statusBarHeight := 1
	authPromptHeight := 0
	if m.pendingAuth != nil {
		authPromptHeight = 4 // Auth prompt takes 4 lines
	}
	
	availableHeight := m.windowHeight - statusBarHeight - authPromptHeight - 1 // -1 for padding
	if availableHeight < 1 {
		availableHeight = 10 // Minimum height
	}
	
	// 1. Render status bar
	statusBar := m.renderStatusBar()
	output.WriteString(statusBar)
	output.WriteString("\n")
	
	// 2. If there's a pending auth request, show it
	if m.pendingAuth != nil {
		authPrompt := m.renderAuthPrompt()
		output.WriteString(authPrompt)
		output.WriteString("\n")
	}
	
	// 3. Render recent logs
	logView := m.renderLogs(availableHeight)
	output.WriteString(logView)
	
	return output.String()
}

// renderStatusBar renders the status bar line
func (m *InlineServerModel) renderStatusBar() string {
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

	// Controls hint
	controlsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	controls := "[q] quit"
	parts = append(parts, controlsStyle.Render(controls))

	// Join with separators
	separator := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render(" │ ")
	return strings.Join(parts, separator)
}

// renderAuthPrompt renders the SSH auth approval prompt
func (m *InlineServerModel) renderAuthPrompt() string {
	warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	addrStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
	promptStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	
	var prompt strings.Builder
	prompt.WriteString(warnStyle.Render("⚠️  NEW CONNECTION: "))
	prompt.WriteString(addrStyle.Render(m.pendingAuth.ClientAddr))
	prompt.WriteString("\n")
	prompt.WriteString(keyStyle.Render("SSH Key: "))
	prompt.WriteString(keyStyle.Render(m.pendingAuth.Fingerprint))
	prompt.WriteString("\n")
	prompt.WriteString(promptStyle.Render("Allow this connection? [Y/n]"))
	
	return prompt.String()
}

// renderLogs renders the recent log entries
func (m *InlineServerModel) renderLogs(maxLines int) string {
	if len(m.logBuffer) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		return dimStyle.Render("No logs yet...")
	}
	
	var logLines []string
	
	// Show the most recent logs that fit in the available space
	startIdx := 0
	if len(m.logBuffer) > maxLines {
		startIdx = len(m.logBuffer) - maxLines
	}
	
	for i := startIdx; i < len(m.logBuffer); i++ {
		entry := m.logBuffer[i]
		logLine := m.formatLogEntry(entry)
		logLines = append(logLines, logLine)
	}
	
	return strings.Join(logLines, "\n")
}

// formatLogEntry formats a single log entry with colors
func (m *InlineServerModel) formatLogEntry(entry LogEntry) string {
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	
	var levelStyle lipgloss.Style
	switch strings.ToUpper(entry.Level) {
	case "ERROR":
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	case "WARN", "WARNING":
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	case "INFO":
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	case "DEBUG":
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	default:
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
	}
	
	msgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	
	return fmt.Sprintf("%s %s %s",
		timeStyle.Render(entry.Timestamp.Format("15:04:05")),
		levelStyle.Render(fmt.Sprintf("%-5s", strings.ToUpper(entry.Level))),
		msgStyle.Render(entry.Message))
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
	LogMsg                struct{ Entry LogEntry }
)