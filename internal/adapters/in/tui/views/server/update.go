package server

import (
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

	// Keyboard input
	case tea.KeyMsg:
		cmd := m.handleKeyPress(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	// Client events
	case messages.ClientConnectedMsg:
		m.clients = append(m.clients, msg.Client)
		m = m.updateClientList()
		m = m.updateFooter()
		m.toasts = m.toasts.AddSuccess("Client connected: " + msg.Client.Name)
		cmds = append(cmds, m.toasts.Init())

	case messages.ClientDisconnectedMsg:
		m = m.removeClient(msg.ClientID)
		m = m.updateClientList()
		m = m.updateFooter()
		m.toasts = m.toasts.AddWarning("Client disconnected: " + msg.ClientID)
		cmds = append(cmds, m.toasts.Init())

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
		m.activeClientID = msg.ActiveClientID
		m.controlLocal = msg.IsLocal
		m = m.updateClientList()
		m = m.updateFooter()
		m = m.updateHeader()
		if msg.IsLocal {
			m.toasts = m.toasts.AddMessage("Control switched to local")
		} else {
			m.toasts = m.toasts.AddMessage("Control switched to: " + msg.ActiveClientID)
		}
		cmds = append(cmds, m.toasts.Init())

	// Activity log
	case messages.ActivityMsg:
		m = m.addActivity(msg.Level, msg.Message)

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
func (m Model) handleKeyPress(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return tea.Quit

	case "j", "down":
		m.clientList = m.clientList.SelectNext()

	case "k", "up":
		m.clientList = m.clientList.SelectPrevious()

	case "enter":
		if client := m.clientList.SelectedClient(); client != nil {
			return m.switchToClient(client.ID)
		}

	case "l":
		return m.switchToLocal()

	case "n", "tab":
		return m.switchToNext()

	case "p", "shift+tab":
		return m.switchToPrevious()

	case "ctrl+r":
		// Emergency release with cooldown - deliberate key combo
		return m.emergencyRelease()

	case "?":
		// Toggle help - could be handled by expanding help component

	case "esc":
		// Clear error if any, or release control if controlling a client
		if m.err != nil {
			m.err = nil
		} else if !m.controlLocal {
			return m.switchToLocal()
		}
	}

	return nil
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
