package server

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
)

// View renders the server view.
func (m Model) View() string {
	if m.quitting {
		return "Shutting down...\n"
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
		// Position toasts at the top right
		toastStyle := lipgloss.NewStyle().
			MarginLeft(m.width - lipgloss.Width(toastView) - 2)
		toastOverlay := toastStyle.Render(toastView)

		// Overlay toasts on top of content
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

	// Main layout: client list on left, details on right
	leftPane := m.clientList.View()

	// Right pane: control status and activity
	rightPane := m.renderControlPanel()

	// Calculate widths
	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth - 4 // Account for margin

	// Apply widths
	leftStyle := lipgloss.NewStyle().
		Width(leftWidth).
		Padding(1, 1)
	rightStyle := lipgloss.NewStyle().
		Width(rightWidth).
		Padding(1, 1)

	left := leftStyle.Render(leftPane)
	right := rightStyle.Render(rightPane)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// renderLoading renders the loading state.
func (m Model) renderLoading() string {
	style := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height-10).
		Align(lipgloss.Center, lipgloss.Center)

	return style.Render(
		styles.IconSpinner + " Starting server...\n\n" +
			styles.MutedStyle.Render("Waiting for connections"),
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
		styles.MutedStyle.Render("Press 'esc' to dismiss")

	return errorBox.Render(errorContent)
}

// renderControlPanel renders the control status panel.
func (m Model) renderControlPanel() string {
	var lines []string

	// Title
	lines = append(lines, styles.TitleStyle.Render(styles.IconMouse+" Control Status"))
	lines = append(lines, "")

	// Current control state
	switch {
	case m.controlLocal:
		lines = append(lines, styles.SuccessStyle.Render(styles.IconCheck+" Controlling: Local"))
	case m.activeClientID != "":
		lines = append(lines, styles.InfoStyle.Render(styles.IconArrowRight+" Controlling: "+m.activeClientID))
	default:
		lines = append(lines, styles.MutedStyle.Render(styles.IconCircleEmpty+" No active control"))
	}

	lines = append(lines, "")

	// Client count
	clientCount := len(m.clients)
	switch clientCount {
	case 0:
		lines = append(lines, styles.MutedStyle.Render("No clients connected"))
	case 1:
		lines = append(lines, styles.InfoStyle.Render("1 client connected"))
	default:
		lines = append(lines, styles.InfoStyle.Render(formatInt(clientCount)+" clients connected"))
	}

	lines = append(lines, "")

	// Recent activity (last 5 entries)
	if len(m.activityLog) > 0 {
		lines = append(lines, styles.LabelStyle.Render("Recent Activity"))
		lines = append(lines, "")

		startIdx := len(m.activityLog) - 5
		if startIdx < 0 {
			startIdx = 0
		}

		for _, entry := range m.activityLog[startIdx:] {
			var style lipgloss.Style
			switch entry.level {
			case "error":
				style = styles.ErrorStyle
			case "warn":
				style = styles.WarningStyle
			case "success":
				style = styles.SuccessStyle
			default:
				style = styles.MutedStyle
			}
			timeStr := entry.time.Format("15:04:05")
			lines = append(lines, styles.MutedStyle.Render(timeStr)+" "+style.Render(entry.message))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// formatInt converts an int to string for display.
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + formatInt(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
