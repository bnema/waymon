package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bnema/waymon/internal/ipc"
	"github.com/bnema/waymon/internal/server"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Message types for server UI
type RefreshClientListMsg struct{}

// ServerModel represents the refactored server UI model
type ServerModel struct {
	BaseModel // Embed base model functionality

	// Server info
	port       int
	serverName string
	version    string

	// Client management
	clientManager *server.ClientManager
	clients       []*server.ConnectedClient
	activeClient  *server.ConnectedClient
	localControl  bool

	// Server reference for proper shutdown
	serverInstance interface{ Stop() }

	// UI components
	viewport viewport.Model
	ready    bool

	// SSH auth approval
	pendingAuth *SSHAuthRequestMsg
	authChannel chan bool

	// UI state
	selectedClientIndex int

	// Program reference for sending messages
	program *tea.Program

	// Styles
	headerStyle     lipgloss.Style
	statusStyle     lipgloss.Style
	clientListStyle lipgloss.Style
	activeStyle     lipgloss.Style
	idleStyle       lipgloss.Style
	controlsStyle   lipgloss.Style
	logStyle        lipgloss.Style
}

// NewServerModel creates a new refactored server UI model
func NewServerModel(port int, serverName, version string) *ServerModel {
	return &ServerModel{
		port:                port,
		serverName:          serverName,
		version:             version,
		localControl:        true, // Start controlling local
		selectedClientIndex: -1,   // No client selected initially

		// Define styles
		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			Padding(0, 1).
			Width(80),

		statusStyle: lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("255")).
			Padding(0, 1),

		clientListStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(1).
			MarginTop(1),

		activeStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true),

		idleStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),

		controlsStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			MarginTop(1),

		logStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("247")),
	}
}

// SetClientManager sets the client manager for real-time updates
func (m *ServerModel) SetClientManager(cm *server.ClientManager) {
	m.clientManager = cm
	// Immediately refresh client list
	m.refreshClientList()
}

// SetServer sets the server instance for proper shutdown handling
func (m *ServerModel) SetServer(srv interface{ Stop() }) {
	m.serverInstance = srv
}

// Init initializes the model
func (m *ServerModel) Init() tea.Cmd {
	if m.base != nil {
		return tea.Batch(
			m.base.TickSpinner(),
			tea.EnterAltScreen,
		)
	}
	return tea.EnterAltScreen
}

// OnShutdown implements UIModel interface
func (m *ServerModel) OnShutdown() error {
	// Stop the server if we have a reference
	if m.serverInstance != nil {
		m.base.AddLogEntry("info", "Stopping server...")
		m.serverInstance.Stop()
	}

	// Disconnect all clients
	if m.clientManager != nil {
		m.base.AddLogEntry("info", "Disconnecting all clients...")
		// Client manager cleanup is handled by server.Stop()
	}

	m.base.AddLogEntry("info", "Server shutdown complete")
	return nil
}

// Update handles messages
func (m *ServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case tea.WindowSizeMsg:
		width, height := msg.Width, msg.Height
		m.base.UpdateWindowSize(width, height)

		// Initialize viewport if not ready
		if !m.ready {
			// Reserve space for header (3 lines), client list (variable), controls (3 lines)
			headerHeight := 3
			clientListHeight := 8
			controlsHeight := 3
			verticalMargins := headerHeight + clientListHeight + controlsHeight

			m.viewport = viewport.New(width, height-verticalMargins)
			m.viewport.YPosition = headerHeight + clientListHeight + controlsHeight
			m.ready = true

			// Show initial content
			m.updateViewport()
		} else {
			availableHeight := height - 14 // Adjust based on static elements
			if availableHeight < 5 {
				availableHeight = 5
			}
			m.viewport.Width = width
			m.viewport.Height = availableHeight
		}

	case tea.KeyMsg:
		// ALWAYS handle quit first
		switch msg.String() {
		case "q", "ctrl+c":
			return m, m.base.InitiateShutdown()
		}

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
				m.base.AddLogEntry("info", fmt.Sprintf("Approved connection from %s", m.pendingAuth.ClientAddr))
				m.pendingAuth = nil
				m.authChannel = nil
			case "n", "N":
				if m.authChannel != nil {
					select {
					case m.authChannel <- false:
					default:
					}
				}
				m.base.AddLogEntry("info", fmt.Sprintf("Denied connection from %s", m.pendingAuth.ClientAddr))
				m.pendingAuth = nil
				m.authChannel = nil
			}
			m.updateViewport()
			return m, nil
		}

		// Normal key handling
		switch msg.String() {
		case "0", "esc":
			// Switch to local control via IPC
			cmd := m.sendReleaseCommand()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		case "tab":
			// Switch to next client
			if m.clientManager != nil {
				if err := m.clientManager.SwitchToNextClient(); err != nil {
					m.base.AddLogEntry("error", fmt.Sprintf("Failed to switch to next client: %v", err))
				} else {
					m.localControl = false
					m.base.AddLogEntry("info", "Switched to next client")
					m.refreshClientList()
				}
			}

		case "1", "2", "3", "4", "5":
			// Switch to specific client by slot number via IPC
			slot, _ := strconv.Atoi(msg.String())
			cmd := m.sendConnectCommand(int32(slot))
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		case "r", "R":
			// Manual emergency release via IPC
			if !m.localControl {
				cmd := m.sendReleaseCommand()
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}

		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		}

	case spinner.TickMsg:
		if m.base != nil {
			cmds = append(cmds, m.base.UpdateSpinner(msg))
		}

	case ClientConnectedMsg:
		m.base.AddLogEntry("info", fmt.Sprintf("Client connected: %s", msg.ClientAddr))
		m.refreshClientList()

	case ClientDisconnectedMsg:
		m.base.AddLogEntry("info", fmt.Sprintf("Client disconnected: %s", msg.ClientAddr))
		
		// If the disconnected client was active, switch to local
		// We need to match by address since that's all we have
		if m.activeClient != nil && m.activeClient.Address == msg.ClientAddr {
			m.localControl = true
			m.activeClient = nil
			m.selectedClientIndex = -1
			m.base.AddLogEntry("info", "Active client disconnected - switched to local control")
		}
		
		m.refreshClientList()

	case SSHAuthRequestMsg:
		m.pendingAuth = &msg
		m.authChannel = msg.ResponseChan
		m.base.AddLogEntry("warn", fmt.Sprintf("SSH auth request from %s", msg.ClientAddr))

	case LogMsg:
		if m.base != nil {
			m.base.AddLogEntry(msg.Entry.Level, msg.Entry.Message)
		}
		m.updateViewport()

	case RefreshClientListMsg:
		m.refreshClientList()

	case SetClientManagerMsg:
		m.clientManager = msg.ClientManager.(*server.ClientManager)
		m.refreshClientList()

	case SetServerMsg:
		m.serverInstance = msg.Server
	}

	// Update viewport
	var viewportCmd tea.Cmd
	m.viewport, viewportCmd = m.viewport.Update(msg)
	cmds = append(cmds, viewportCmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m *ServerModel) View() string {
	if !m.ready || m.base == nil {
		return "Initializing..."
	}

	var output strings.Builder

	// 1. Status bar
	output.WriteString(m.renderServerStatusBar())
	output.WriteString("\n\n")

	// 2. SSH auth prompt if pending
	if m.pendingAuth != nil {
		output.WriteString(m.renderAuthPrompt())
		output.WriteString("\n")
	}

	// 3. Client list
	output.WriteString(m.renderClientList())
	output.WriteString("\n")

	// 4. Controls
	output.WriteString(m.renderControls())
	output.WriteString("\n\n")

	// 5. Logs viewport
	output.WriteString(m.viewport.View())

	return output.String()
}

// Helper methods

func (m *ServerModel) renderServerStatusBar() string {
	// Connection info
	status := fmt.Sprintf("Listening on port %d", m.port)
	if m.base.IsShuttingDown() {
		status = "Shutting down..."
	}

	// Current control status
	var controlStatus string
	if m.localControl {
		controlStatus = "Controlling: LOCAL"
	} else if m.activeClient != nil {
		controlStatus = fmt.Sprintf("Controlling: %s", m.activeClient.Name)
	} else {
		controlStatus = "Controlling: NONE"
	}

	return FormatAppHeader("SERVER MODE", fmt.Sprintf("%s | %s", status, controlStatus))
}

func (m *ServerModel) renderAuthPrompt() string {
	promptStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214")).
		Background(lipgloss.Color("235")).
		Padding(1, 2)

	return promptStyle.Render(fmt.Sprintf(
		"⚠️  SSH Authentication Request from %s\nFingerprint: %s\n[Y]es to approve, [N]o to deny",
		m.pendingAuth.ClientAddr,
		m.pendingAuth.Fingerprint,
	))
}

func (m *ServerModel) renderClientList() string {
	var content strings.Builder

	// Header
	content.WriteString(lipgloss.NewStyle().Bold(true).Render("Connected Clients:"))
	content.WriteString("\n")

	if len(m.clients) == 0 {
		content.WriteString(m.idleStyle.Render("  No clients connected"))
	} else {
		for i, client := range m.clients {
			// Number for selection
			numStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
			content.WriteString(numStyle.Render(fmt.Sprintf("  [%d] ", i+1)))

			// Client info with status
			var clientLine string
			if m.activeClient != nil && client.ID == m.activeClient.ID {
				clientLine = m.activeStyle.Render(fmt.Sprintf("▶ %s (%s) - ACTIVE", client.Name, client.Address))
			} else {
				clientLine = m.idleStyle.Render(fmt.Sprintf("  %s (%s)", client.Name, client.Address))
			}
			content.WriteString(clientLine)
			content.WriteString("\n")
		}
	}

	// Show local control option
	content.WriteString("\n")
	if m.localControl {
		content.WriteString(m.activeStyle.Render("  [0/ESC] ▶ Local Control - ACTIVE"))
	} else {
		content.WriteString(m.idleStyle.Render("  [0/ESC]   Local Control"))
	}

	return m.clientListStyle.Render(content.String())
}

func (m *ServerModel) renderControls() string {
	controls := []string{
		"[1-5] Switch to client",
		"[Tab] Next client",
		"[0/ESC] Local control",
		"[R] Emergency release",
		"[g/G] Top/Bottom",
		"[q] Quit",
	}

	if m.base.IsShuttingDown() {
		return m.controlsStyle.Render("Shutting down... " + m.base.GetSpinner())
	}

	return m.controlsStyle.Render("Controls: " + strings.Join(controls, " • "))
}

func (m *ServerModel) refreshClientList() {
	if m.clientManager != nil {
		m.clients = m.clientManager.GetConnectedClients()
		
		// Update active client reference based on client manager state
		// The ClientManager tracks the active client internally
		// We'll need to check which client is active by other means
		m.activeClient = m.clientManager.GetActiveClient()
		if m.activeClient != nil {
			m.localControl = false
		} else {
			m.localControl = true
		}
		
		// If no client is active, we must be in local control
		if m.activeClient == nil {
			m.localControl = true
		}
		
		m.updateViewport()
	}
}

func (m *ServerModel) updateViewport() {
	logs := m.base.GetLogs()
	var content strings.Builder

	for _, entry := range logs {
		content.WriteString(m.base.FormatLogEntry(entry))
		content.WriteString("\n")
	}

	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
}

// AddLogEntry adds a log entry (for compatibility)
func (m *ServerModel) AddLogEntry(entry LogEntry) {
	// Send as a message to trigger UI update
	if m.program != nil {
		m.program.Send(LogMsg{Entry: entry})
	} else if m.base != nil {
		// Fallback if program not set
		m.base.AddLogEntry(entry.Level, entry.Message)
		m.updateViewport()
	}
}

// SetProgram sets the tea.Program reference for sending updates
func (m *ServerModel) SetProgram(p *tea.Program) {
	m.program = p
}

// GetProgram returns the tea.Program reference
func (m *ServerModel) GetProgram() *tea.Program {
	return m.program
}

// sendReleaseCommand sends an IPC release command and updates UI
func (m *ServerModel) sendReleaseCommand() tea.Cmd {
	return func() tea.Msg {
		// Create IPC client
		client, err := ipc.NewClient()
		if err != nil {
			return LogMsg{
				Entry: LogEntry{
					Level:   "error",
					Message: fmt.Sprintf("Failed to create IPC client: %v", err),
				},
			}
		}
		defer client.Close()

		// Send release command
		if err := client.SendRelease(); err != nil {
			return LogMsg{
				Entry: LogEntry{
					Level:   "error",
					Message: fmt.Sprintf("Failed to send release command: %v", err),
				},
			}
		}

		// Update UI state immediately
		m.localControl = true
		m.selectedClientIndex = -1
		m.activeClient = nil

		// Schedule a refresh
		go func() {
			time.Sleep(200 * time.Millisecond)
			if m.program != nil {
				m.program.Send(RefreshClientListMsg{})
			}
		}()

		return LogMsg{
			Entry: LogEntry{
				Level:   "info",
				Message: "Released client control - now controlling local system",
			},
		}
	}
}

// sendConnectCommand sends an IPC connect command and updates UI
func (m *ServerModel) sendConnectCommand(slot int32) tea.Cmd {
	// Validate slot and get client info
	if slot < 1 || slot > 5 || int(slot) > len(m.clients) {
		return nil
	}

	client := m.clients[slot-1]
	
	return func() tea.Msg {
		// Create IPC client
		ipcClient, err := ipc.NewClient()
		if err != nil {
			return LogMsg{
				Entry: LogEntry{
					Level:   "error",
					Message: fmt.Sprintf("Failed to create IPC client: %v", err),
				},
			}
		}
		defer ipcClient.Close()

		// Send connect command
		if err := ipcClient.SendConnect(slot); err != nil {
			return LogMsg{
				Entry: LogEntry{
					Level:   "error",
					Message: fmt.Sprintf("Failed to send connect command: %v", err),
				},
			}
		}

		// Update UI state immediately
		m.localControl = false
		m.selectedClientIndex = int(slot - 1)
		m.activeClient = client

		// Schedule a refresh
		go func() {
			time.Sleep(200 * time.Millisecond)
			if m.program != nil {
				m.program.Send(RefreshClientListMsg{})
			}
		}()

		return LogMsg{
			Entry: LogEntry{
				Level:   "info",
				Message: fmt.Sprintf("Switching to control %s (%s)...", client.Name, client.Address),
			},
		}
	}
}

// RunServerUI runs the server UI with proper lifecycle management
// It returns the model and a channel that will be closed when the UI exits
func RunServerUI(ctx context.Context, port int, serverName, version string) (*ServerModel, <-chan struct{}, error) {
	// Create the model
	model := NewServerModel(port, serverName, version)

	// Create program runner with configuration
	config := ProgramConfig{
		ShutdownConfig: ShutdownConfig{
			GracePeriod: 30 * time.Second, // Longer for server to handle clients
			ForcePeriod: 5 * time.Second,
		},
		Debug: false,
	}

	runner := NewProgramRunner(config)

	// Run the UI in a goroutine
	go func() {
		if err := runner.Run(ctx, model); err != nil {
			if model.base != nil {
				model.base.AddLogEntry("error", fmt.Sprintf("UI error: %v", err))
			}
		}
	}()

	// Return the model and done channel from the runner
	return model, runner.Done(), nil
}