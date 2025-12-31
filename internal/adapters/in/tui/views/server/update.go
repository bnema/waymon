package server

import (
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bnema/waymon/internal/adapters/in/tui/components"
	"github.com/bnema/waymon/internal/adapters/in/tui/messages"
	"github.com/bnema/waymon/internal/domain"
)

// Update handles incoming messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// Window resize
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.updateClientList()
		m = m.updateFooter()
		m = m.updateHeader()
		m = m.updateLogStream()

	// Keyboard input
	case tea.KeyMsg:
		cmd, clearErr := m.handleKeyPress(msg)
		if clearErr {
			m.err = nil
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	// Client events
	case messages.ClientConnectedMsg:
		// Check if client already exists (avoid duplicate toasts from refresh)
		exists := false
		for _, c := range m.clients {
			if c.ID == msg.Client.ID {
				exists = true
				break
			}
		}
		if !exists {
			m.clients = append(m.clients, msg.Client)
			m = m.updateClientList()
			m = m.updateFooter()
			m.toasts = m.toasts.AddSuccess("Client connected: " + msg.Client.Name)
			cmds = append(cmds, m.toasts.Init())
		}

	case messages.ClientDisconnectedMsg:
		// Check if client exists before showing toast (avoid duplicate toasts)
		exists := false
		for _, c := range m.clients {
			if c.ID == msg.ClientID {
				exists = true
				break
			}
		}
		if exists {
			m = m.removeClient(msg.ClientID)
			m = m.updateClientList()
			m = m.updateFooter()
			m.toasts = m.toasts.AddWarning("Client disconnected: " + msg.ClientID)
			cmds = append(cmds, m.toasts.Init())
		}

	case messages.ClientListUpdatedMsg:
		m.clients = msg.Clients
		m = m.updateClientList()
		m = m.updateFooter()
		m.ready = true

	case messages.ClientUpdatedMsg:
		m = m.updateClient(msg.Client)
		m = m.updateClientList()

	// Control events
	case messages.ControlSwitchedMsg:
		// Only update and show toast if state actually changed
		stateChanged := m.activeClientID != msg.ActiveClientID || m.controlLocal != msg.IsLocal
		m.activeClientID = msg.ActiveClientID
		m.controlLocal = msg.IsLocal
		m = m.updateClientList()
		m = m.updateFooter()
		m = m.updateHeader()
		// Only show toast for explicit control switches, not automatic ones on disconnect
		// (disconnect already shows its own toast)
		if stateChanged && !msg.IsLocal {
			// Only show toast when switching TO a client, not when returning to local
			// (returning to local on disconnect is handled by disconnect toast)
			m.toasts = m.toasts.AddMessage("Control switched to: " + msg.ActiveClientID)
			cmds = append(cmds, m.toasts.Init())
		}

	// Activity log
	case messages.ActivityMsg:
		m = m.addActivity(msg.Level, msg.Message)

	// Log stream messages
	case messages.LogMsg:
		level := components.LogLevelInfo
		switch msg.Level {
		case "DBG", "debug":
			level = components.LogLevelDebug
		case "INF", "info":
			level = components.LogLevelInfo
		case "WRN", "warn", "warning":
			level = components.LogLevelWarn
		case "ERR", "error":
			level = components.LogLevelError
		}
		m = m.AddLog(level, msg.Message, msg.Fields)

	// Log stream component updates
	case components.LogStreamMsg:
		var cmd tea.Cmd
		m.logStream, cmd = m.logStream.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	// Error handling
	case messages.ErrorMsg:
		// Don't show "client not found" errors as they're expected during disconnects
		if msg.Err != nil && !errors.Is(msg.Err, domain.ErrClientNotFound) {
			m.err = msg.Err
			m.toasts = m.toasts.AddError(msg.Err.Error())
			cmds = append(cmds, m.toasts.Init())
		}

	// Toast management
	case components.ToastDismissMsg:
		m.toasts = m.toasts.Update(msg)

	// Periodic tick
	case messages.TickMsg:
		// Refresh client list periodically
		cmds = append(cmds, m.refreshClients())
		cmds = append(cmds, tea.Tick(time.Second, func(_ time.Time) tea.Msg {
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
// Returns (command, shouldClearError).
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return tea.Quit, false

	case "j", "down":
		m.clientList = m.clientList.SelectNext()

	case "k", "up":
		m.clientList = m.clientList.SelectPrevious()

	case "enter":
		if client := m.clientList.SelectedClient(); client != nil {
			return m.switchToClient(client.ID), false
		}

	case "l":
		return m.switchToLocal(), false

	case "n", "tab":
		return m.switchToNext(), false

	case "p", "shift+tab":
		return m.switchToPrevious(), false

	case "ctrl+r":
		// Emergency release with cooldown - deliberate key combo
		return m.emergencyRelease(), false

	case "?":
		// Toggle help - could be handled by expanding help component

	case "esc":
		// Clear error if any, or release control if controlling a client
		if m.err != nil {
			return nil, true // Signal to clear error
		} else if !m.controlLocal {
			return m.switchToLocal(), false
		}
	}

	return nil, false
}

// switchToClient switches control to a specific client.
func (m Model) switchToClient(clientID string) tea.Cmd {
	return func() tea.Msg {
		err := m.useCase.SwitchToClient(m.ctx, clientID)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}
		return messages.ControlSwitchedMsg{
			ActiveClientID: clientID,
			IsLocal:        false,
		}
	}
}

// switchToLocal switches control back to local.
func (m Model) switchToLocal() tea.Cmd {
	return func() tea.Msg {
		err := m.useCase.SwitchToLocal(m.ctx)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}
		return messages.ControlSwitchedMsg{
			ActiveClientID: "",
			IsLocal:        true,
		}
	}
}

// switchToNext switches to the next client.
func (m Model) switchToNext() tea.Cmd {
	return func() tea.Msg {
		err := m.useCase.SwitchToNext(m.ctx)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}
		// The use case will send the actual update via callback
		return nil
	}
}

// switchToPrevious switches to the previous client.
func (m Model) switchToPrevious() tea.Cmd {
	return func() tea.Msg {
		err := m.useCase.SwitchToPrevious(m.ctx)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}
		// The use case will send the actual update via callback
		return nil
	}
}

// emergencyRelease releases control with cooldown to prevent immediate re-control.
// This is triggered by Ctrl+R and marks the emergency release timestamp,
// preventing clients from requesting control for a cooldown period.
func (m Model) emergencyRelease() tea.Cmd {
	return func() tea.Msg {
		// Mark emergency release first to engage cooldown
		m.useCase.MarkEmergencyRelease(m.ctx)

		// Then switch to local
		err := m.useCase.SwitchToLocal(m.ctx)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}
		return messages.ControlSwitchedMsg{
			ActiveClientID: "",
			IsLocal:        true,
		}
	}
}

// removeClient removes a client from the local list.
func (m Model) removeClient(clientID string) Model {
	newClients := make([]domain.Client, 0, len(m.clients))
	for _, c := range m.clients {
		if c.ID != clientID {
			newClients = append(newClients, c)
		}
	}
	m.clients = newClients

	// If the disconnected client was active, switch to local
	if m.activeClientID == clientID {
		m.activeClientID = ""
		m.controlLocal = true
	}
	return m
}

// updateClient updates a client in the local list.
func (m Model) updateClient(client domain.Client) Model {
	for i, c := range m.clients {
		if c.ID == client.ID {
			m.clients[i] = client
			break
		}
	}
	return m
}
