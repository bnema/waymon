package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/bnema/waymon/internal/client"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ClientModel represents the UI model for the client
type ClientModel struct {
	serverAddr    string
	inputReceiver *client.InputReceiver
	lastUpdate    time.Time
	spinner       spinner.Model
	message       string
	messageType   string
	messageExpiry time.Time
	version       string

	// Connection state
	connected       bool
	reconnecting    bool
	waitingApproval bool
	controlStatus   client.ControlStatus

	// Log display
	logBuffer    []LogEntry
	maxLogLines  int
	windowHeight int
	windowWidth  int
}

// NewClientModel creates a new client UI model
func NewClientModel(serverAddr string, inputReceiver *client.InputReceiver, version string) *ClientModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	model := &ClientModel{
		serverAddr:    serverAddr,
		inputReceiver: inputReceiver,
		spinner:       s,
		lastUpdate:    time.Now(),
		logBuffer:     make([]LogEntry, 0),
		maxLogLines:   50,
		windowHeight:  24,
		windowWidth:   80,
		version:       version,
	}

	// Set up status change callback
	if inputReceiver != nil {
		inputReceiver.OnStatusChange(func(status client.ControlStatus) {
			model.controlStatus = status
		})
	}

	return model
}

// Init initializes the client model
func (m *ClientModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.EnterAltScreen,
	)
}

// Update handles messages for the client model
func (m *ClientModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			// Request reconnection
			m.SetMessage("info", "Reconnecting...")
			// TODO: Implement reconnection logic

		case "p":
			// Request pause/release control
			if m.inputReceiver != nil && m.controlStatus.BeingControlled {
				if err := m.inputReceiver.RequestControlRelease(); err != nil {
					m.SetMessage("error", fmt.Sprintf("Failed to request release: %v", err))
				} else {
					m.SetMessage("info", "Requested control release")
				}
			}

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
		m.reconnecting = false
		m.waitingApproval = false
		m.SetMessage("success", "Connected to server")

	case DisconnectedMsg:
		m.connected = false
		m.reconnecting = false
		m.waitingApproval = false
		m.controlStatus = client.ControlStatus{}
		m.SetMessage("error", "Disconnected from server")

	case ReconnectingMsg:
		m.connected = false
		m.reconnecting = true
		m.waitingApproval = false
		m.controlStatus = client.ControlStatus{}
		m.SetMessage("info", msg.Status)

	case WaitingApprovalMsg:
		m.waitingApproval = true
		m.SetMessage("info", "Waiting for server approval...")

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

// View renders the client UI
func (m *ClientModel) View() string {
	var output strings.Builder

	// Calculate available space for logs
	statusBarHeight := 1
	waitingPromptHeight := 0
	if m.waitingApproval {
		waitingPromptHeight = 4
	}
	controlStatusHeight := 3 // Control status section

	availableHeight := m.windowHeight - statusBarHeight - waitingPromptHeight - controlStatusHeight - 1
	if availableHeight < 1 {
		availableHeight = 10
	}

	// 1. Render status bar
	output.WriteString(m.renderClientStatusBar())
	output.WriteString("\n")

	// 2. If waiting for approval, show it
	if m.waitingApproval {
		output.WriteString(m.renderWaitingPrompt())
		output.WriteString("\n")
	}

	// 3. Render control status
	output.WriteString(m.renderControlStatus())
	output.WriteString("\n")

	// 4. Render recent logs
	output.WriteString(m.renderClientLogs(availableHeight))

	return output.String()
}

// renderClientStatusBar renders the status bar for the redesigned client
func (m *ClientModel) renderClientStatusBar() string {
	// Connection status
	var statusText string
	switch {
	case m.connected:
		statusText = fmt.Sprintf("Connected to %s", m.serverAddr)
	case m.reconnecting:
		statusText = fmt.Sprintf("Reconnecting to %s", m.serverAddr)
	default:
		statusText = fmt.Sprintf("Disconnected from %s", m.serverAddr)
	}

	return FormatAppHeader("CLIENT MODE", statusText)
}

// renderControlStatus renders the control status section
func (m *ClientModel) renderControlStatus() string {
	var output strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	output.WriteString(headerStyle.Render("Control Status:"))
	output.WriteString("\n")

	switch {
	case m.controlStatus.BeingControlled:
		// Being controlled
		controlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
		output.WriteString("  ")
		output.WriteString(controlStyle.Render(fmt.Sprintf("▶ BEING CONTROLLED BY %s", m.controlStatus.ControllerName)))
		output.WriteString("\n")

		// Show controls for when being controlled
		controlsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		controls := "  Controls: [p] Request pause • [r] Reconnect • [q] Quit"
		output.WriteString(controlsStyle.Render(controls))
	case m.connected:
		// Connected but idle
		idleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		output.WriteString("  ")
		output.WriteString(idleStyle.Render("■ Idle - Waiting for server control"))
		output.WriteString("\n")

		// Show controls for idle state
		controlsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		controls := "  Controls: [r] Reconnect • [q] Quit"
		output.WriteString(controlsStyle.Render(controls))
	default:
		// Disconnected
		disconnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		output.WriteString("  ")
		output.WriteString(disconnStyle.Render("✗ Disconnected"))
		output.WriteString("\n")

		// Show controls for disconnected state
		controlsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		controls := "  Controls: [r] Reconnect • [q] Quit"
		output.WriteString(controlsStyle.Render(controls))
	}

	return output.String()
}

// renderWaitingPrompt renders the waiting for approval prompt
func (m *ClientModel) renderWaitingPrompt() string {
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
func (m *ClientModel) renderClientLogs(maxLines int) string {
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
func (m *ClientModel) formatClientLogEntry(entry LogEntry) string {
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

// AddLogEntry adds a new log entry to the client buffer
func (m *ClientModel) AddLogEntry(entry LogEntry) {
	m.logBuffer = append(m.logBuffer, entry)

	// Keep only the last maxLogLines entries
	if len(m.logBuffer) > m.maxLogLines {
		m.logBuffer = m.logBuffer[len(m.logBuffer)-m.maxLogLines:]
	}
}

// SetMessage sets a temporary message
func (m *ClientModel) SetMessage(msgType, message string) {
	m.message = message
	m.messageType = msgType
	m.messageExpiry = time.Now().Add(3 * time.Second)
}
