package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FullscreenServerModel represents the full-screen TUI model for the server
type FullscreenServerModel struct {
	// Server info
	port        int
	serverName  string
	clientCount int

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
	logs    []string
	maxLogs int

	// Styles
	statusStyle lipgloss.Style
	logStyle    lipgloss.Style
	headerStyle lipgloss.Style
}

// NewFullscreenServerModel creates a new server UI model
func NewFullscreenServerModel(port int, serverName string) *FullscreenServerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &FullscreenServerModel{
		port:       port,
		serverName: serverName,
		spinner:    s,
		logs:       make([]string, 0),
		maxLogs:    1000, // Keep more logs in memory

		// Define styles
		statusStyle: lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("255")).
			Padding(0, 1),
		logStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("247")),
		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			Padding(0, 1),
	}
}

// Init initializes the model
func (m *FullscreenServerModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.EnterAltScreen,
	)
}

// Update handles messages
func (m *FullscreenServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height

		// Initialize viewport if not ready
		if !m.ready {
			// Reserve space for header (3 lines) and status bar (1 line)
			headerHeight := 3
			footerHeight := 1
			verticalMargins := headerHeight + footerHeight

			m.viewport = viewport.New(msg.Width, msg.Height-verticalMargins)
			m.viewport.YPosition = headerHeight
			m.ready = true

			// Show initial content
			m.viewport.SetContent(m.renderLogs())
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 4
		}

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
				m.addLog("INFO", fmt.Sprintf("Approved connection from %s", m.pendingAuth.ClientAddr))
				m.pendingAuth = nil
				m.authChannel = nil
			case "n", "N":
				if m.authChannel != nil {
					select {
					case m.authChannel <- false:
					default:
					}
				}
				m.addLog("INFO", fmt.Sprintf("Denied connection from %s", m.pendingAuth.ClientAddr))
				m.pendingAuth = nil
				m.authChannel = nil
			}
			m.viewport.SetContent(m.renderLogs())
			m.viewport.GotoBottom()
			return m, nil
		}

		// Normal key handling
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case SSHAuthRequestMsg:
		m.pendingAuth = &msg
		m.authChannel = msg.ResponseChan
		m.viewport.SetContent(m.renderLogs())
		m.viewport.GotoBottom()

	case ClientConnectedMsg:
		m.clientCount++
		m.addLog("INFO", fmt.Sprintf("Client connected from %s (total: %d)", msg.ClientAddr, m.clientCount))
		m.viewport.SetContent(m.renderLogs())
		m.viewport.GotoBottom()

	case ClientDisconnectedMsg:
		if m.clientCount > 0 {
			m.clientCount--
		}
		m.addLog("INFO", fmt.Sprintf("Client disconnected: %s (remaining: %d)", msg.ClientAddr, m.clientCount))
		m.viewport.SetContent(m.renderLogs())
		m.viewport.GotoBottom()

	case LogMsg:
		m.addLogEntry(msg.Entry)
		m.viewport.SetContent(m.renderLogs())
		// Auto-scroll to bottom for new logs
		m.viewport.GotoBottom()
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m *FullscreenServerModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	var b strings.Builder

	// Header
	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n")

	// Viewport (logs)
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Status bar
	statusBar := m.renderStatusBar()
	b.WriteString(statusBar)

	return b.String()
}

// renderHeader renders the header section
func (m *FullscreenServerModel) renderHeader() string {
	// Title bar
	title := m.headerStyle.Width(m.windowWidth).Render(
		fmt.Sprintf("🖱️  WAYMON SERVER - %s", m.serverName),
	)

	// Status line
	var statusParts []string

	// Server status
	statusParts = append(statusParts, fmt.Sprintf("%s Listening on port %d", m.spinner.View(), m.port))

	// Client count
	if m.clientCount > 0 {
		statusParts = append(statusParts, fmt.Sprintf("👥 %d client%s", m.clientCount, pluralize(m.clientCount)))
	} else {
		statusParts = append(statusParts, "👥 No clients")
	}

	statusLine := m.statusStyle.Width(m.windowWidth).Render(strings.Join(statusParts, " │ "))

	// Auth prompt if needed
	if m.pendingAuth != nil {
		authPrompt := m.renderAuthPrompt()
		return fmt.Sprintf("%s\n%s\n%s", title, statusLine, authPrompt)
	}

	return fmt.Sprintf("%s\n%s", title, statusLine)
}

// renderAuthPrompt renders the SSH auth approval prompt
func (m *FullscreenServerModel) renderAuthPrompt() string {
	warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	promptStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))

	prompt := fmt.Sprintf("%s New connection from %s (fingerprint: %s) %s",
		warnStyle.Render("⚠️"),
		m.pendingAuth.ClientAddr,
		m.pendingAuth.Fingerprint[:16]+"...",
		promptStyle.Render("[Y/n]"),
	)

	return lipgloss.NewStyle().
		Background(lipgloss.Color("52")).
		Foreground(lipgloss.Color("255")).
		Width(m.windowWidth).
		Padding(0, 1).
		Render(prompt)
}

// renderStatusBar renders the bottom status bar
func (m *FullscreenServerModel) renderStatusBar() string {
	var parts []string

	// Scroll position
	scrollInfo := fmt.Sprintf("%d/%d", m.viewport.YOffset+m.viewport.Height, len(m.logs))
	parts = append(parts, scrollInfo)

	// Controls
	controls := "[q] quit │ [g/G] top/bottom │ [↑/↓] scroll"
	parts = append(parts, controls)

	status := strings.Join(parts, " │ ")

	return lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("240")).
		Width(m.windowWidth).
		Padding(0, 1).
		Render(status)
}

// renderLogs renders all logs as a single string
func (m *FullscreenServerModel) renderLogs() string {
	if len(m.logs) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true).
			Render("  Waiting for events...")
	}

	return strings.Join(m.logs, "\n")
}

// addLog adds a raw log message
func (m *FullscreenServerModel) addLog(level, message string) {
	timestamp := time.Now().Format("15:04:05")
	entry := m.formatLogEntry(timestamp, level, message)
	m.logs = append(m.logs, entry)

	// Trim old logs if needed
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}
}

// addLogEntry adds a log entry
func (m *FullscreenServerModel) addLogEntry(entry LogEntry) {
	formatted := m.formatLogEntry(
		entry.Timestamp.Format("15:04:05"),
		entry.Level,
		entry.Message,
	)
	m.logs = append(m.logs, formatted)

	// Trim old logs if needed
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}
}

// formatLogEntry formats a single log entry with colors
func (m *FullscreenServerModel) formatLogEntry(timestamp, level, message string) string {
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	var levelStyle lipgloss.Style
	switch strings.ToUpper(level) {
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

	return fmt.Sprintf("  %s %s %s",
		timeStyle.Render(timestamp),
		levelStyle.Render(fmt.Sprintf("%-5s", strings.ToUpper(level))),
		msgStyle.Render(message))
}
