package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bnema/waymon/internal/server"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// refreshClientListMsg is an internal message to trigger a client list refresh
type refreshClientListMsg struct{}

// serverShutdownMsg is an internal message to trigger proper server shutdown
type serverShutdownMsg struct{}

// ServerModel represents the redesigned server UI model where server is the controller
type ServerModel struct {
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
	spinner  spinner.Model
	ready    bool

	// Window dimensions
	windowWidth  int
	windowHeight int

	// SSH auth approval
	pendingAuth *SSHAuthRequestMsg
	authChannel chan bool

	// Log buffer
	logBuffer   []LogEntry
	maxLogLines int

	// UI state
	selectedClientIndex int
	lastUpdate          time.Time

	// Styles
	headerStyle     lipgloss.Style
	statusStyle     lipgloss.Style
	clientListStyle lipgloss.Style
	activeStyle     lipgloss.Style
	idleStyle       lipgloss.Style
	controlsStyle   lipgloss.Style
	logStyle        lipgloss.Style
}

// NewServerModel creates a new redesigned server UI model
func NewServerModel(port int, serverName, version string) *ServerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &ServerModel{
		port:                port,
		serverName:          serverName,
		version:             version,
		spinner:             s,
		localControl:        true, // Start controlling local
		selectedClientIndex: -1,   // No client selected initially
		logBuffer:           make([]LogEntry, 0),
		maxLogLines:         100,
		lastUpdate:          time.Now(),

		// Define styles for the redesigned UI
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
}

// SetServer sets the server instance for proper shutdown handling
func (m *ServerModel) SetServer(srv interface{ Stop() }) {
	m.serverInstance = srv
}

// Init initializes the redesigned model
func (m *ServerModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.EnterAltScreen,
	)
}

// Update handles messages for the redesigned server UI
func (m *ServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height

		// Initialize viewport if not ready
		if !m.ready {
			// Reserve space for header (3 lines), client list (variable), controls (3 lines)
			headerHeight := 3
			clientListHeight := 8
			controlsHeight := 3
			verticalMargins := headerHeight + clientListHeight + controlsHeight

			m.viewport = viewport.New(msg.Width, msg.Height-verticalMargins)
			m.viewport.YPosition = headerHeight + clientListHeight + controlsHeight
			m.ready = true

			// Show initial content
			m.updateViewport()
		} else {
			availableHeight := msg.Height - 14 // Adjust based on static elements
			if availableHeight < 5 {
				availableHeight = 5
			}
			m.viewport.Width = msg.Width
			m.viewport.Height = availableHeight
		}

	case tea.KeyMsg:
		// ALWAYS handle quit first - should work regardless of state
		switch msg.String() {
		case "q", "ctrl+c":
			// Send shutdown message instead of direct tea.Quit to ensure proper cleanup
			return m, func() tea.Msg { return serverShutdownMsg{} }
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
				m.AddLogEntry(LogEntry{
					Timestamp: time.Now(),
					Level:     "INFO",
					Message:   fmt.Sprintf("Approved connection from %s", m.pendingAuth.ClientAddr),
				})
				m.pendingAuth = nil
				m.authChannel = nil
			case "n", "N":
				if m.authChannel != nil {
					select {
					case m.authChannel <- false:
					default:
					}
				}
				m.AddLogEntry(LogEntry{
					Timestamp: time.Now(),
					Level:     "INFO",
					Message:   fmt.Sprintf("Denied connection from %s", m.pendingAuth.ClientAddr),
				})
				m.pendingAuth = nil
				m.authChannel = nil
			}
			m.updateViewport()
			return m, nil
		}

		// Normal key handling for redesigned server UI
		switch msg.String() {

		case "0", "esc":
			// Switch to local control
			if m.clientManager != nil {
				// Do the switch in a goroutine to prevent UI blocking
				go func() {
					if err := m.clientManager.SwitchToLocal(); err != nil {
						m.AddLogEntry(LogEntry{
							Timestamp: time.Now(),
							Level:     "ERROR",
							Message:   fmt.Sprintf("Failed to switch to local: %v", err),
						})
					}
				}()

				// Immediately update our local state for responsive UI
				m.localControl = true
				m.selectedClientIndex = -1
				m.activeClient = nil

				m.AddLogEntry(LogEntry{
					Timestamp: time.Now(),
					Level:     "INFO",
					Message:   "Released client control - now controlling local system",
				})

				// Force immediate UI update
				m.updateViewport()

				// Schedule a delayed refresh
				cmd := tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
					return refreshClientListMsg{}
				})
				cmds = append(cmds, cmd)
			}

		case "tab":
			// Switch to next client
			if m.clientManager != nil {
				if err := m.clientManager.SwitchToNextClient(); err != nil {
					m.AddLogEntry(LogEntry{
						Timestamp: time.Now(),
						Level:     "ERROR",
						Message:   fmt.Sprintf("Failed to switch to next client: %v", err),
					})
				} else {
					m.localControl = false
					m.AddLogEntry(LogEntry{
						Timestamp: time.Now(),
						Level:     "INFO",
						Message:   "Switched to next client",
					})
					// Refresh client list to update status indicators
					m.refreshClientList()
				}
			}

		case "1", "2", "3", "4", "5":
			// Only handle number keys for switching when controlling local
			// This prevents keyboard events forwarded to clients from triggering switches
			if m.localControl {
				// Switch to specific client by number
				clientNum, _ := strconv.Atoi(msg.String())
				if m.clientManager != nil && clientNum <= len(m.clients) && clientNum > 0 {
					client := m.clients[clientNum-1]

					// Do the switch in a goroutine to prevent UI blocking
					go func() {
						if err := m.clientManager.SwitchToClient(client.ID); err != nil {
							m.AddLogEntry(LogEntry{
								Timestamp: time.Now(),
								Level:     "ERROR",
								Message:   fmt.Sprintf("Failed to switch to client %s: %v", client.Name, err),
							})
						}
					}()

					// Immediately update our local state for responsive UI
					m.localControl = false
					m.selectedClientIndex = clientNum - 1
					m.activeClient = client

					// Log the switch attempt
					m.AddLogEntry(LogEntry{
						Timestamp: time.Now(),
						Level:     "INFO",
						Message:   fmt.Sprintf("Switching to control %s (%s)...", client.Name, client.Address),
					})

					// Force immediate UI update
					m.updateViewport()

					// Schedule a refresh to ensure state is synced
					cmd := tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
						return refreshClientListMsg{}
					})
					cmds = append(cmds, cmd)
				}
			}

		case "r", "R":
			// Manual emergency release
			if m.clientManager != nil && !m.localControl {
				go func() {
					if err := m.clientManager.SwitchToLocal(); err != nil {
						m.AddLogEntry(LogEntry{
							Timestamp: time.Now(),
							Level:     "ERROR",
							Message:   fmt.Sprintf("Failed to release control: %v", err),
						})
					} else {
						m.AddLogEntry(LogEntry{
							Timestamp: time.Now(),
							Level:     "INFO",
							Message:   "Manual emergency release - control returned to local",
						})
					}
				}()
				
				// Update UI state immediately
				m.localControl = true
				m.selectedClientIndex = -1
				m.activeClient = nil
				m.updateViewport()
			}
			
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case ClientConnectedMsg:
		m.AddLogEntry(LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   fmt.Sprintf("Client connected from %s", msg.ClientAddr),
		})
		m.refreshClientList()

	case ClientDisconnectedMsg:
		m.AddLogEntry(LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   fmt.Sprintf("Client disconnected from %s", msg.ClientAddr),
		})
		m.refreshClientList()

	case SSHAuthRequestMsg:
		m.pendingAuth = &msg
		m.authChannel = msg.ResponseChan
		m.AddLogEntry(LogEntry{
			Timestamp: time.Now(),
			Level:     "WARN",
			Message:   fmt.Sprintf("SSH auth request from %s (fingerprint: %s)", msg.ClientAddr, msg.Fingerprint),
		})

	case LogMsg:
		m.AddLogEntry(msg.Entry)

	case SetClientManagerMsg:
		if cm, ok := msg.ClientManager.(*server.ClientManager); ok {
			m.SetClientManager(cm)
			m.AddLogEntry(LogEntry{
				Timestamp: time.Now(),
				Level:     "INFO",
				Message:   "Client manager connected to UI",
			})
		}

	case SetServerMsg:
		m.SetServer(msg.Server)
		m.AddLogEntry(LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   "Server instance connected to UI",
		})

	case refreshClientListMsg:
		// Handle the delayed refresh
		m.refreshClientList()

	case serverShutdownMsg:
		// Handle proper server shutdown
		m.AddLogEntry(LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   "Server shutting down...",
		})

		// Perform proper shutdown sequence
		if m.serverInstance != nil {
			go func() {
				m.serverInstance.Stop()
			}()
		}

		// Now quit the UI
		return m, tea.Quit
	}

	// Update viewport content
	m.updateViewport()

	return m, tea.Batch(cmds...)
}

// View renders the redesigned server UI
func (m *ServerModel) View() string {
	if !m.ready {
		return "\n\n   " + m.spinner.View() + " Initializing server..."
	}

	var output strings.Builder

	// 1. Header
	output.WriteString(m.renderHeader())
	output.WriteString("\n")

	// 2. Client List
	output.WriteString(m.renderClientList())
	output.WriteString("\n")

	// 3. Controls
	output.WriteString(m.renderControls())
	output.WriteString("\n")

	// 4. Auth prompt if pending
	if m.pendingAuth != nil {
		output.WriteString(m.renderAuthPrompt())
		output.WriteString("\n")
	}

	// 5. Logs viewport
	output.WriteString(m.viewport.View())

	return output.String()
}

// renderHeader renders the header showing server status
func (m *ServerModel) renderHeader() string {
	statusText := fmt.Sprintf("Active on port %d", m.port)
	title := FormatAppHeader("SERVER MODE", statusText)
	headerLine2 := "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

	return title + "\n" + headerLine2
}

// renderClientList renders the connected clients list with current control status
func (m *ServerModel) renderClientList() string {
	var output strings.Builder

	output.WriteString("Connected Clients:\n")

	if len(m.clients) == 0 {
		output.WriteString("  No clients connected")
	} else {
		for i, client := range m.clients {
			prefix := fmt.Sprintf("  [%d] ", i+1)

			// Add monitor count if available
			monitorInfo := ""
			if len(client.Monitors) > 0 {
				monitorInfo = fmt.Sprintf(" [%dm]", len(client.Monitors))
			}

			clientInfo := fmt.Sprintf("%s (%s)%s", client.Name, client.Address, monitorInfo)

			// Show status - check if this client is being controlled
			var status string
			if !m.localControl && m.activeClient != nil && client.ID == m.activeClient.ID {
				status = m.activeStyle.Render(" - CONTROLLING ←")
			} else {
				status = m.idleStyle.Render(" - Idle")
			}

			output.WriteString(prefix + clientInfo + status + "\n")
		}
	}

	// Show local control status - this should be at the bottom
	output.WriteString("\n")
	if m.localControl {
		localStatus := m.activeStyle.Render("  → [LOCAL] Controlling local system ←")
		output.WriteString(localStatus)
	} else if m.activeClient != nil {
		// When controlling a client, show which one
		localStatus := m.idleStyle.Render("  [LOCAL] Local system - Idle")
		output.WriteString(localStatus)
		output.WriteString("\n")
		controllingMsg := m.activeStyle.Render(fmt.Sprintf("  ▶ NOW CONTROLLING: %s (%s)", m.activeClient.Name, m.activeClient.Address))
		output.WriteString(controllingMsg)
	} else {
		localStatus := m.idleStyle.Render("  [LOCAL] Local system - Idle")
		output.WriteString(localStatus)
	}

	return output.String()
}

// renderControls renders the control instructions
func (m *ServerModel) renderControls() string {
	var controlsText string

	if m.localControl {
		// When controlling local, emphasize client switching
		controlsText = "Controls:\n" +
			"  [1-5] Switch to client • [Tab] Next client\n" +
			"  [g] Top of logs • [G] Bottom of logs • [q] Quit"
	} else {
		// When controlling a client, emphasize escape/release
		escapeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
		controlsText = "Controls:\n" +
			"  [1-5] Switch to client • " + escapeStyle.Render("[Esc/0] RELEASE CONTROL") + " • [Tab] Next client\n" +
			"  [g] Top of logs • [G] Bottom of logs • [q] Quit"
	}

	return m.controlsStyle.Render(controlsText)
}

// renderAuthPrompt renders the SSH authentication prompt
func (m *ServerModel) renderAuthPrompt() string {
	if m.pendingAuth == nil {
		return ""
	}

	promptStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214")).
		Background(lipgloss.Color("234")).
		Padding(1).
		Border(lipgloss.RoundedBorder())

	prompt := fmt.Sprintf("SSH Auth Request from %s\n", m.pendingAuth.ClientAddr) +
		fmt.Sprintf("Fingerprint: %s\n", m.pendingAuth.Fingerprint) +
		"Approve connection? [y/n]"

	return promptStyle.Render(prompt)
}

// updateViewport updates the viewport content with current logs
func (m *ServerModel) updateViewport() {
	m.viewport.SetContent(m.renderLogs())
	m.viewport.GotoBottom()
}

// renderLogs renders the log entries
func (m *ServerModel) renderLogs() string {
	if len(m.logBuffer) == 0 {
		return m.logStyle.Render("No logs yet...")
	}

	var logLines []string
	for _, entry := range m.logBuffer {
		logLine := m.formatLogEntry(entry)
		logLines = append(logLines, logLine)
	}

	return strings.Join(logLines, "\n")
}

// formatLogEntry formats a single log entry with colors
func (m *ServerModel) formatLogEntry(entry LogEntry) string {
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

// AddLogEntry adds a new log entry to the buffer
func (m *ServerModel) AddLogEntry(entry LogEntry) {
	m.logBuffer = append(m.logBuffer, entry)

	// Keep only the last maxLogLines entries
	if len(m.logBuffer) > m.maxLogLines {
		m.logBuffer = m.logBuffer[len(m.logBuffer)-m.maxLogLines:]
	}

	m.lastUpdate = time.Now()
}

// refreshClientList updates the client list from the client manager
func (m *ServerModel) refreshClientList() {
	if m.clientManager != nil {
		// Add timeout protection to prevent hanging
		done := make(chan bool, 1)
		go func() {
			m.clients = m.clientManager.GetConnectedClients()
			m.activeClient = m.clientManager.GetActiveClient()
			m.localControl = m.clientManager.IsControllingLocal()
			done <- true
		}()

		select {
		case <-done:
			// Success
		case <-time.After(100 * time.Millisecond):
			// Timeout - don't hang the UI
			m.AddLogEntry(LogEntry{
				Timestamp: time.Now(),
				Level:     "WARN",
				Message:   "Client manager refresh timed out",
			})
		}
	}
}
