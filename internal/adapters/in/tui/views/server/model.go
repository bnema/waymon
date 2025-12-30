// Package server provides the server TUI view.
package server

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bnema/waymon/internal/adapters/in/tui/components"
	"github.com/bnema/waymon/internal/adapters/in/tui/messages"
	"github.com/bnema/waymon/internal/boundaries/in"
	"github.com/bnema/waymon/internal/domain"
)

// Model is the Bubble Tea model for the server view.
type Model struct {
	// Context for cancellation
	ctx context.Context

	// Use case interface (NOT concrete implementation)
	useCase in.ServerUseCase

	// UI state
	clients        []domain.Client
	activeClientID string
	controlLocal   bool
	err            error
	ready          bool
	quitting       bool

	// Window dimensions
	width  int
	height int

	// Activity log
	activityLog []activityEntry
	maxLogSize  int

	// Components
	clientList components.ClientList
	header     components.Header
	statusBar  components.StatusBar
	help       components.Help
	toasts     components.ToastManager
}

// activityEntry is a log entry for activity display.
type activityEntry struct {
	level   string
	message string
	time    time.Time
}

// New creates a new server view model.
func New(ctx context.Context, useCase in.ServerUseCase) Model {
	return Model{
		ctx:          ctx,
		useCase:      useCase,
		clients:      make([]domain.Client, 0),
		controlLocal: true,
		activityLog:  make([]activityEntry, 0),
		maxLogSize:   50,
		clientList:   components.NewClientList(),
		header:       components.ServerHeader(80),
		statusBar:    components.NewStatusBar(),
		help:         components.NewHelp().WithBindings(components.ServerBindings()...),
		toasts:       components.NewToastManager(),
		width:        80,
		height:       24,
	}
}

// Init initializes the model and returns initial commands.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.subscribeToEvents(),
		m.refreshClients(),
		tea.Tick(time.Second, func(_ time.Time) tea.Msg {
			return messages.TickMsg{}
		}),
	)
}

// subscribeToEvents sets up callbacks from the use case.
func (m Model) subscribeToEvents() tea.Cmd {
	return func() tea.Msg {
		// Set up activity callback
		m.useCase.SetOnActivity(func(_, _ string) {
			// This callback is called from the use case
			// We need to send it as a tea.Msg
			// Note: In a real implementation, you'd use a channel here
		})
		return nil
	}
}

// refreshClients fetches the current client list.
func (m Model) refreshClients() tea.Cmd {
	return func() tea.Msg {
		clients := m.useCase.GetConnectedClients(m.ctx)
		return messages.ClientListUpdatedMsg{Clients: clients}
	}
}

// GetClients returns the current list of connected clients.
func (m Model) GetClients() []domain.Client {
	return m.clients
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

// addActivity adds an activity log entry.
func (m Model) addActivity(level, message string) Model {
	entry := activityEntry{
		level:   level,
		message: message,
		time:    time.Now(),
	}
	m.activityLog = append(m.activityLog, entry)

	// Trim old entries
	if len(m.activityLog) > m.maxLogSize {
		m.activityLog = m.activityLog[len(m.activityLog)-m.maxLogSize:]
	}

	return m
}

// updateClientList updates the client list component state.
func (m Model) updateClientList() Model {
	m.clientList = m.clientList.
		WithClients(m.clients).
		WithActiveID(m.activeClientID).
		WithWidth(m.width - 4)
	return m
}

// updateStatusBar updates the status bar content.
func (m Model) updateStatusBar() Model {
	var controlValue string
	if m.controlLocal {
		controlValue = "Local"
	} else {
		controlValue = m.activeClientID
	}

	m.statusBar = m.statusBar.
		WithWidth(m.width).
		WithItems(
			components.StatusItem{Label: "Control", Value: controlValue},
			components.ClientsItem(len(m.clients), ""),
		)
	return m
}

// updateHeader updates the header with current status.
func (m Model) updateHeader() Model {
	var status string
	switch {
	case m.controlLocal:
		status = "Controlling: Local"
	case m.activeClientID != "":
		status = fmt.Sprintf("Controlling: %s", m.activeClientID)
	default:
		status = "Waiting for clients..."
	}
	m.header = m.header.WithWidth(m.width).WithStatus(status)
	return m
}
