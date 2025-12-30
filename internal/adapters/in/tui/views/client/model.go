// Package client provides the client TUI view.
package client

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bnema/waymon/internal/adapters/in/tui/components"
	"github.com/bnema/waymon/internal/adapters/in/tui/messages"
	"github.com/bnema/waymon/internal/boundaries/in"
	"github.com/bnema/waymon/internal/domain"
)

// Model is the Bubble Tea model for the client view.
type Model struct {
	// Context for cancellation
	ctx context.Context

	// Use case interface (NOT concrete implementation)
	useCase in.ClientUseCase

	// Connection state
	connected    bool
	serverName   string
	connectError error

	// Control state
	controlStatus domain.ControlStatus

	// Monitor information
	monitors []domain.Monitor

	// UI state
	err      error
	ready    bool
	quitting bool

	// Window dimensions
	width  int
	height int

	// Components
	header         components.Header
	statusBar      components.StatusBar
	help           components.Help
	toasts         components.ToastManager
	monitorDisplay components.MonitorDisplay
}

// New creates a new client view model.
func New(ctx context.Context, useCase in.ClientUseCase) Model {
	return Model{
		ctx:            ctx,
		useCase:        useCase,
		monitors:       make([]domain.Monitor, 0),
		header:         components.ClientHeader(80),
		statusBar:      components.NewStatusBar(),
		help:           components.NewHelp().WithBindings(components.ClientBindings()...),
		toasts:         components.NewToastManager(),
		monitorDisplay: components.NewMonitorDisplay(),
		width:          80,
		height:         24,
	}
}

// Init initializes the model and returns initial commands.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.subscribeToEvents(),
		m.checkConnection(),
		tea.Tick(time.Second, func(_ time.Time) tea.Msg {
			return messages.TickMsg{}
		}),
	)
}

// subscribeToEvents sets up callbacks from the use case.
func (m Model) subscribeToEvents() tea.Cmd {
	return func() tea.Msg {
		// Set up control change callback
		m.useCase.SetOnControlChanged(func(_ domain.ControlStatus) {
			// Note: In a real implementation, send via channel
		})

		// Set up connection state callback
		m.useCase.SetOnConnectionStateChanged(func(_ bool, _ string) {
			// Note: In a real implementation, send via channel
		})

		return nil
	}
}

// checkConnection checks the current connection status.
func (m Model) checkConnection() tea.Cmd {
	return func() tea.Msg {
		connected := m.useCase.IsConnected()
		return messages.ConnectionStateMsg{
			Connected: connected,
			Error:     nil,
		}
	}
}

// GetControlStatus returns the current control status.
func (m Model) GetControlStatus() domain.ControlStatus {
	return m.controlStatus
}

// IsConnected returns whether the client is connected.
func (m Model) IsConnected() bool {
	return m.connected
}

// IsReady returns whether the model is ready for interaction.
func (m Model) IsReady() bool {
	return m.ready
}

// SetReady sets the ready state.
func (m Model) SetReady(ready bool) Model {
	m.ready = ready
	return m
}

// SetError sets an error state.
func (m Model) SetError(err error) Model {
	m.err = err
	return m
}

// GetError returns the current error, if any.
func (m Model) GetError() error {
	return m.err
}

// AddToast adds a toast notification.
func (m Model) AddToast(toast components.Toast) Model {
	m.toasts = m.toasts.Add(toast)
	return m
}

// updateStatusBar updates the status bar content.
func (m Model) updateStatusBar() Model {
	var connectionItem components.StatusItem
	if m.connected {
		connectionItem = components.StatusItem{
			Label: "Server",
			Value: m.serverName,
		}
	} else {
		connectionItem = components.StatusItem{
			Label: "Status",
			Value: "Disconnected",
		}
	}

	var controlItem components.StatusItem
	if m.controlStatus.BeingControlled {
		controlItem = components.StatusItem{
			Label: "Control",
			Value: "Controlled by " + m.controlStatus.ControllerName,
		}
	} else {
		controlItem = components.StatusItem{
			Label: "Control",
			Value: "Idle",
		}
	}

	m.statusBar = m.statusBar.
		WithWidth(m.width).
		WithItems(connectionItem, controlItem)

	return m
}

// updateHeader updates the header with current status.
func (m Model) updateHeader() Model {
	var status string
	if m.connected {
		status = "Connected to " + m.serverName
	} else {
		status = "Disconnected"
	}
	m.header = m.header.WithWidth(m.width).WithStatus(status)
	return m
}

// updateMonitorDisplay updates the monitor display.
func (m Model) updateMonitorDisplay() Model {
	m.monitorDisplay = m.monitorDisplay.
		WithMonitors(m.monitors).
		WithSize(m.width-4, m.height/3)
	return m
}
