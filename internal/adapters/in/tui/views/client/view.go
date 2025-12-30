package client

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
)

// View renders the client view.
func (m Model) View() string {
	if m.quitting {
		return "Disconnecting...\n"
	}

	// Build layout
	var sections []string

	// Header
	sections = append(sections, m.header.View())

	// Main content area
	mainContent := m.renderMainContent()
	sections = append(sections, mainContent)

	// Status bar at bottom
	sections = append(sections, m.statusBar.View())

	// Help line
	helpLine := m.help.View()
	if helpLine != "" {
		sections = append(sections, helpLine)
	}

	// Join all sections vertically
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Add toasts overlay if any
	if m.toasts.HasToasts() {
		toastView := m.toasts.View()
		toastStyle := lipgloss.NewStyle().
			MarginLeft(m.width - lipgloss.Width(toastView) - 2)
		toastOverlay := toastStyle.Render(toastView)

		content = lipgloss.JoinVertical(lipgloss.Left,
			toastOverlay,
			content,
		)
	}

	return content
}

// renderMainContent renders the main content area.
func (m Model) renderMainContent() string {
	if !m.ready {
		return m.renderLoading()
	}

	if m.err != nil {
		return m.renderError()
	}

	if !m.connected {
		return m.renderDisconnected()
	}

	// Connected state
	return m.renderConnected()
}

// renderLoading renders the loading state.
func (m Model) renderLoading() string {
	style := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height-10).
		Align(lipgloss.Center, lipgloss.Center)

	return style.Render(
		styles.IconSpinner + " Initializing client...\n\n" +
			styles.MutedStyle.Render("Connecting to server"),
	)
}

// renderError renders the error state.
func (m Model) renderError() string {
	errorBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Error).
		Padding(1, 2).
		Width(m.width - 10)

	errorContent := styles.IconCross + " Error\n\n" +
		styles.ErrorStyle.Render(m.err.Error()) + "\n\n" +
		styles.MutedStyle.Render("Press 'esc' to dismiss, 'r' to retry")

	return errorBox.Render(errorContent)
}

// renderDisconnected renders the disconnected state.
func (m Model) renderDisconnected() string {
	var lines []string

	// Disconnection notice
	lines = append(lines, styles.TitleStyle.Render(styles.IconDisconnected+" Not Connected"))
	lines = append(lines, "")

	if m.connectError != nil {
		lines = append(lines, styles.ErrorStyle.Render("Last error: "+m.connectError.Error()))
		lines = append(lines, "")
	}

	lines = append(lines, styles.MutedStyle.Render("Press 'r' to reconnect"))

	style := lipgloss.NewStyle().
		Padding(2, 2).
		Width(m.width - 4)

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderConnected renders the connected state.
func (m Model) renderConnected() string {
	var lines []string

	// Connection status
	lines = append(lines, styles.TitleStyle.Render(styles.IconConnected+" Connected"))
	lines = append(lines, "")

	// Server info
	if m.serverName != "" {
		lines = append(lines, styles.LabelStyle.Render("Server: ")+m.serverName)
	}
	lines = append(lines, "")

	// Control status
	lines = append(lines, styles.LabelStyle.Render("Control Status"))
	if m.controlStatus.BeingControlled {
		lines = append(lines, styles.SuccessStyle.Render(
			styles.IconMouse+" Being controlled by "+m.controlStatus.ControllerName,
		))
	} else {
		lines = append(lines, styles.MutedStyle.Render(
			styles.IconCircleEmpty+" Idle - waiting for control",
		))
	}
	lines = append(lines, "")

	// Monitor display
	if len(m.monitors) > 0 {
		lines = append(lines, m.monitorDisplay.View())
	}

	// Layout
	leftPane := lipgloss.JoinVertical(lipgloss.Left, lines...)

	style := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width - 4)

	return style.Render(leftPane)
}
