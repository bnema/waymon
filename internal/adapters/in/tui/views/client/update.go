package client

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bnema/waymon/internal/adapters/in/tui/components"
	"github.com/bnema/waymon/internal/adapters/in/tui/messages"
)

// Update handles incoming messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// Window resize
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.updateStatusBar()
		m = m.updateHeader()
		m = m.updateMonitorDisplay()

	// Keyboard input
	case tea.KeyMsg:
		cmd := m.handleKeyPress(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	// Connection events
	case messages.ConnectionStateMsg:
		m.connected = msg.Connected
		if msg.Error != nil {
			m.connectError = msg.Error
			m.toasts = m.toasts.AddError("Connection failed: " + msg.Error.Error())
		} else if msg.Connected {
			m.toasts = m.toasts.AddSuccess("Connected to server")
			m.connectError = nil
		} else {
			m.toasts = m.toasts.AddWarning("Disconnected from server")
		}
		m = m.updateStatusBar()
		m = m.updateHeader()
		m.ready = true
		cmds = append(cmds, m.toasts.Init())

	// Control status events
	case messages.ControlStatusChangedMsg:
		m.controlStatus = msg.Status
		m = m.updateStatusBar()
		if msg.Status.BeingControlled {
			m.toasts = m.toasts.AddMessage("Control acquired by " + msg.Status.ControllerName)
		} else {
			m.toasts = m.toasts.AddMessage("Control released")
		}
		cmds = append(cmds, m.toasts.Init())

	// Server info
	case messages.ServerInfoReceivedMsg:
		m.serverName = msg.Info.Name
		m = m.updateStatusBar()
		m = m.updateHeader()

	// Monitor detection
	case messages.MonitorsDetectedMsg:
		m.monitors = msg.Monitors
		m = m.updateMonitorDisplay()

	// Error handling
	case messages.ErrorMsg:
		m.err = msg.Err
		m.toasts = m.toasts.AddError(msg.Err.Error())
		cmds = append(cmds, m.toasts.Init())

	// Toast management
	case components.ToastDismissMsg:
		m.toasts = m.toasts.Update(msg)

	// Periodic tick
	case messages.TickMsg:
		// Check connection status periodically
		cmds = append(cmds, m.checkConnection())
		cmds = append(cmds, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return messages.TickMsg{}
		}))

	// Quit message
	case messages.QuitMsg:
		m.quitting = true
		return m, tea.Quit
	}

	return m, tea.Batch(cmds...)
}

// handleKeyPress handles keyboard input.
func (m Model) handleKeyPress(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return tea.Sequence(
			m.disconnect(),
			tea.Quit,
		)

	case "r":
		// Reconnect
		if !m.connected {
			return m.connect()
		}

	case "d":
		// Disconnect
		if m.connected {
			return m.disconnect()
		}

	case "?":
		// Toggle help

	case "esc":
		// Clear error
		m.err = nil
		m.connectError = nil
	}

	return nil
}

// connect initiates connection to server.
func (m Model) connect() tea.Cmd {
	return func() tea.Msg {
		err := m.useCase.Connect(m.ctx)
		if err != nil {
			return messages.ConnectionStateMsg{
				Connected: false,
				Error:     err,
			}
		}
		return messages.ConnectionStateMsg{
			Connected: true,
			Error:     nil,
		}
	}
}

// disconnect closes connection to server.
func (m Model) disconnect() tea.Cmd {
	return func() tea.Msg {
		err := m.useCase.Disconnect(m.ctx)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}
		return messages.ConnectionStateMsg{
			Connected: false,
			Error:     nil,
		}
	}
}
