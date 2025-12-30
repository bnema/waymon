// Package status provides the status command TUI view.
// This is a simple one-shot view that displays current status and exits.
package status

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bnema/waymon/internal/adapters/in/tui/components"
	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
	"github.com/bnema/waymon/internal/boundaries/in"
)

// Model is the Bubble Tea model for the status view.
type Model struct {
	// Data from IPC response
	active        bool
	connected     bool
	serverHost    string
	currentIndex  int32
	totalCount    int32
	computerNames []string

	// UI state
	err    error
	width  int
	height int
}

// NewFromIPC creates a status model from IPC response data.
func NewFromIPC(response *in.StatusResponse) Model {
	m := Model{
		width:  80,
		height: 24,
	}

	if response != nil {
		m.active = response.Active
		m.connected = response.Connected
		m.serverHost = response.ServerHost
		m.currentIndex = response.CurrentIndex
		m.totalCount = response.TotalCount
		m.computerNames = response.ComputerNames
	}

	return m
}

// NewEmpty creates an empty status model.
func NewEmpty() Model {
	return Model{
		width:  80,
		height: 24,
	}
}

// SetActive sets whether mouse sharing is active.
func (m Model) SetActive(active bool) Model {
	m.active = active
	return m
}

// SetConnected sets whether connected.
func (m Model) SetConnected(connected bool) Model {
	m.connected = connected
	return m
}

// SetServerHost sets the server host.
func (m Model) SetServerHost(host string) Model {
	m.serverHost = host
	return m
}

// SetCurrentIndex sets the current client index.
func (m Model) SetCurrentIndex(index int32) Model {
	m.currentIndex = index
	return m
}

// SetTotalCount sets the total client count.
func (m Model) SetTotalCount(count int32) Model {
	m.totalCount = count
	return m
}

// SetComputerNames sets the computer names.
func (m Model) SetComputerNames(names []string) Model {
	m.computerNames = names
	return m
}

// SetError sets an error state.
func (m Model) SetError(err error) Model {
	m.err = err
	return m
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	// One-shot view - no ongoing commands needed
	return nil
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc", "enter":
			return m, tea.Quit
		}
	}

	return m, nil
}

// View renders the status view.
func (m Model) View() string {
	if m.err != nil {
		return m.renderError()
	}

	var lines []string

	// Header
	header := components.NewHeader("Waymon Status").
		WithIcon(styles.IconInfo).
		WithWidth(m.width)
	lines = append(lines, header.View())
	lines = append(lines, "")

	// Connection status
	lines = append(lines, m.renderConnectionStatus())
	lines = append(lines, "")

	// Control status
	lines = append(lines, m.renderControlStatus())
	lines = append(lines, "")

	// Computer list
	if len(m.computerNames) > 0 {
		lines = append(lines, m.renderComputerList())
		lines = append(lines, "")
	}

	// Footer
	lines = append(lines, styles.MutedStyle.Render("Press any key to exit"))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderError renders error state.
func (m Model) renderError() string {
	errorBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Error).
		Padding(1, 2)

	content := styles.IconCross + " Failed to get status\n\n" +
		styles.ErrorStyle.Render(m.err.Error())

	return errorBox.Render(content)
}

// renderConnectionStatus renders the connection status section.
func (m Model) renderConnectionStatus() string {
	var lines []string

	lines = append(lines, styles.LabelStyle.Render("Connection Status"))

	if m.connected {
		status := styles.SuccessStyle.Render(styles.IconConnected + " Connected")
		if m.serverHost != "" {
			status += styles.MutedStyle.Render(" to " + m.serverHost)
		}
		lines = append(lines, "  "+status)
	} else {
		lines = append(lines, "  "+styles.ErrorStyle.Render(styles.IconDisconnected+" Not connected"))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderControlStatus renders the control status section.
func (m Model) renderControlStatus() string {
	var lines []string

	lines = append(lines, styles.LabelStyle.Render("Mouse Sharing"))

	if m.active {
		lines = append(lines, "  "+styles.SuccessStyle.Render(styles.IconMouse+" Active"))
		if m.currentIndex >= 0 && int(m.currentIndex) < len(m.computerNames) {
			lines = append(lines, "  "+styles.InfoStyle.Render(
				styles.IconArrowRight+" Controlling: "+m.computerNames[m.currentIndex],
			))
		}
	} else {
		lines = append(lines, "  "+styles.MutedStyle.Render(styles.IconCircleEmpty+" Inactive"))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderComputerList renders the available computers.
func (m Model) renderComputerList() string {
	var lines []string

	lines = append(lines, styles.LabelStyle.Render(fmt.Sprintf("Available Computers (%d)", m.totalCount)))

	for i, name := range m.computerNames {
		var statusIcon string
		var statusStyle lipgloss.Style

		if int32(i) == m.currentIndex && m.active {
			statusIcon = styles.IconMouse
			statusStyle = styles.SuccessStyleBold
		} else {
			statusIcon = styles.IconClient
			statusStyle = styles.MutedStyle
		}

		line := fmt.Sprintf("  %d. %s", i+1, statusStyle.Render(statusIcon+" "+name))
		lines = append(lines, line)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// RenderStatic renders the status as a static string (for non-TUI output).
func (m Model) RenderStatic() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %s\n", m.err.Error())
	}

	var output string

	// Connection status
	if m.connected {
		output += fmt.Sprintf("Connected: Yes (%s)\n", m.serverHost)
	} else {
		output += "Connected: No\n"
	}

	// Control status
	if m.active {
		output += "Mouse sharing: Active\n"
		if m.currentIndex >= 0 && int(m.currentIndex) < len(m.computerNames) {
			output += fmt.Sprintf("Controlling: %s\n", m.computerNames[m.currentIndex])
		}
	} else {
		output += "Mouse sharing: Inactive\n"
	}

	// Computers
	output += fmt.Sprintf("Computers: %d\n", m.totalCount)
	for i, name := range m.computerNames {
		active := ""
		if int32(i) == m.currentIndex && m.active {
			active = " [active]"
		}
		output += fmt.Sprintf("  %d. %s%s\n", i+1, name, active)
	}

	return output
}
