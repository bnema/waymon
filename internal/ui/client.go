package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bnema/waymon/internal/client"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Message types for client UI
type ControlStatusMsg struct {
	Status client.ControlStatus
}

// ClientModel represents the refactored UI model for the client
type ClientModel struct {
	BaseModel // Embed base model functionality

	serverAddr    string
	inputReceiver *client.InputReceiver
	version       string

	// Connection state
	connected       bool
	reconnecting    bool
	waitingApproval bool
	controlStatus   client.ControlStatus

	// Message display
	message       string
	messageType   string
	messageExpiry time.Time
}

// NewClientModel creates a new refactored client UI model
func NewClientModel(serverAddr string, inputReceiver *client.InputReceiver, version string) *ClientModel {
	return &ClientModel{
		serverAddr:    serverAddr,
		inputReceiver: inputReceiver,
		version:       version,
	}
}

// Init initializes the client model
func (m *ClientModel) Init() tea.Cmd {
	if m.base != nil {
		return tea.Batch(
			m.base.TickSpinner(),
			tea.EnterAltScreen,
		)
	}
	return tea.EnterAltScreen
}

// OnShutdown implements UIModel interface
func (m *ClientModel) OnShutdown() error {
	// Disconnect input receiver
	if m.inputReceiver != nil {
		m.base.AddLogEntry("info", "Disconnecting input receiver...")
		if err := m.inputReceiver.Disconnect(); err != nil {
			m.base.AddLogEntry("error", fmt.Sprintf("Failed to disconnect input receiver: %v", err))
			return err
		}
	}

	m.base.AddLogEntry("info", "Client shutdown complete")
	return nil
}

// SetProgram sets the tea.Program for sending updates
func (m *ClientModel) SetProgram(p *tea.Program) {
	if m.inputReceiver != nil {
		// Set up control status callback
		m.inputReceiver.OnStatusChange(func(status client.ControlStatus) {
			p.Send(ControlStatusMsg{Status: status})
		})

		// Set up connection callbacks
		m.inputReceiver.SetOnConnected(func() {
			p.Send(ConnectedMsg{})
		})

		m.inputReceiver.SetOnDisconnected(func() {
			p.Send(DisconnectedMsg{})
		})

		m.inputReceiver.SetOnReconnectStatus(func(status string) {
			p.Send(ReconnectingMsg{Status: status})
		})
	}
}

// Update handles messages for the client model
func (m *ClientModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// First handle base updates (including shutdown)
	model, cmd := m.BaseModel.Update(msg)
	if cmd != nil {
		// Check if this is a quit command
		if _, ok := cmd().(tea.QuitMsg); ok {
			return model, cmd
		}
		cmds = append(cmds, cmd)
	}

	// Don't process other messages if shutting down
	if m.base != nil && m.base.IsShuttingDown() {
		return m, tea.Batch(cmds...)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			if m.controlStatus.BeingControlled {
				// When being controlled, 'r' requests release
				// Use a command to avoid blocking the Update method
				return m, m.requestControlRelease()
			} else {
				// When not being controlled, 'r' reconnects
				m.SetMessage("info", "Reconnecting...")
				// TODO: Implement reconnection logic
			}

		case "p":
			// Alternative key for pause/release control
			if m.controlStatus.BeingControlled {
				// Use a command to avoid blocking the Update method
				return m, m.requestControlRelease()
			}

		case "q":
			// Quit triggers shutdown
			return m, m.base.InitiateShutdown()
		}

	case spinner.TickMsg:
		if !m.connected || m.waitingApproval {
			if m.base != nil {
				cmds = append(cmds, m.base.UpdateSpinner(msg))
			}
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

	case LogMsg:
		if m.base != nil {
			m.base.AddLogEntry(msg.Entry.Level, msg.Entry.Message)
		}

	case ControlStatusMsg:
		m.controlStatus = msg.Status
		// Log the status change
		if msg.Status.BeingControlled {
			m.SetMessage("info", fmt.Sprintf("Now being controlled by %s", msg.Status.ControllerName))
		} else {
			m.SetMessage("success", "Control released")
		}
	}

	// Clear expired messages
	if !m.messageExpiry.IsZero() && time.Now().After(m.messageExpiry) {
		m.message = ""
		m.messageType = ""
		m.messageExpiry = time.Time{}
	}

	return m, tea.Batch(cmds...)
}

// View renders the client UI
func (m *ClientModel) View() string {
	if m.base == nil {
		return "Initializing..."
	}

	var output strings.Builder
	_, height := m.base.GetWindowSize()

	// Calculate available space for logs
	statusBarHeight := 1
	waitingPromptHeight := 0
	if m.waitingApproval {
		waitingPromptHeight = 4
	}
	controlStatusHeight := 3 // Control status section

	availableHeight := height - statusBarHeight - waitingPromptHeight - controlStatusHeight - 1
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

// renderClientStatusBar renders the status bar
func (m *ClientModel) renderClientStatusBar() string {
	// Connection status
	var statusText string
	switch {
	case m.base.IsShuttingDown():
		statusText = "Shutting down..."
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
	case m.base.IsShuttingDown():
		// Shutting down
		shutdownStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
		output.WriteString("  ")
		output.WriteString(shutdownStyle.Render("⏳ SHUTTING DOWN..."))
		output.WriteString("\n")
		output.WriteString("  ")
		output.WriteString(m.base.GetSpinner())
		output.WriteString(" Cleaning up resources...")

	case m.controlStatus.BeingControlled:
		// Being controlled
		controlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
		output.WriteString("  ")
		output.WriteString(controlStyle.Render(fmt.Sprintf("▶ BEING CONTROLLED BY %s", m.controlStatus.ControllerName)))
		output.WriteString("\n")

		// Show controls for when being controlled
		controlsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		controls := "  Controls: [r] Release control • [p] Pause (alt) • [q] Quit"
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

// renderClientLogs renders the recent log entries
func (m *ClientModel) renderClientLogs(maxLines int) string {
	logs := m.base.GetLogs()

	if len(logs) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		return dimStyle.Render("No logs yet...")
	}

	var logLines []string

	// Show the most recent logs that fit in the available space
	startIdx := 0
	if len(logs) > maxLines {
		startIdx = len(logs) - maxLines
	}

	for i := startIdx; i < len(logs); i++ {
		logLine := m.base.FormatLogEntry(logs[i])
		logLines = append(logLines, logLine)
	}

	return strings.Join(logLines, "\n")
}

// SetMessage sets a temporary message
func (m *ClientModel) SetMessage(msgType, message string) {
	m.message = message
	m.messageType = msgType
	m.messageExpiry = time.Now().Add(3 * time.Second)

	// Also add to logs
	if m.base != nil {
		m.base.AddLogEntry(msgType, message)
	}
}

// requestControlRelease returns a command that requests control release
func (m *ClientModel) requestControlRelease() tea.Cmd {
	// Show user that action is being processed
	m.SetMessage("info", "Requesting control release...")

	// Return a command that performs the async operation
	return func() tea.Msg {
		if m.inputReceiver == nil {
			return LogMsg{Entry: LogEntry{Level: "error", Message: "Input receiver not available"}}
		}

		// Request release - this happens asynchronously
		if err := m.inputReceiver.RequestControlRelease(); err != nil {
			return LogMsg{Entry: LogEntry{Level: "error", Message: fmt.Sprintf("Failed to request release: %v", err)}}
		}

		return LogMsg{Entry: LogEntry{Level: "info", Message: "Control release requested successfully"}}
	}
}

// RunClientUI runs the client UI with proper lifecycle management
func RunClientUI(ctx context.Context, serverAddr string, inputReceiver *client.InputReceiver, version string) error {
	// Create the model
	model := NewClientModel(serverAddr, inputReceiver, version)

	// Create program runner with configuration
	config := ProgramConfig{
		ShutdownConfig: ShutdownConfig{
			GracePeriod: 5 * time.Second,
			ForcePeriod: 2 * time.Second,
		},
		Debug: false,
	}

	runner := NewProgramRunner(config)

	// Run the UI (blocking)
	// The runner will call SetProgram on the model automatically
	return runner.Run(ctx, model)
}
