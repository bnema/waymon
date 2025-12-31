package server

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
)

const (
	// MinPaneWidth is the minimum width for a pane to be readable.
	MinPaneWidth = 35
	// MinTerminalWidth is the minimum supported terminal width.
	MinTerminalWidth = 40
	// MinTerminalHeight is the minimum supported terminal height.
	MinTerminalHeight = 12
	// LogPaneHeight is the fixed height for the log stream pane.
	LogPaneHeight = 10
)

// View renders the server view.
func (m Model) View() string {
	if m.quitting {
		return "Shutting down...\n"
	}

	// Check minimum terminal size
	if m.width < MinTerminalWidth || m.height < MinTerminalHeight {
		return m.renderTooSmall()
	}

	// 1. Render header and footer first (they have fixed content)
	header := m.header.View()
	footer := m.footer.View()

	// 2. Measure their heights using lipgloss
	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)

	// 3. Calculate available space for main content
	mainHeight := m.height - headerHeight - footerHeight
	mainWidth := m.width

	// 4. Render main content (responsive horizontal/vertical)
	var mainContent string
	switch {
	case !m.ready:
		mainContent = m.renderLoading(mainHeight, mainWidth)
	case m.err != nil:
		mainContent = m.renderError(mainHeight, mainWidth)
	default:
		mainContent = m.renderMainContent(mainHeight, mainWidth)
	}

	// 5. Assemble layout
	content := lipgloss.JoinVertical(lipgloss.Left, header, mainContent, footer)

	// 6. Handle toast overlay (top-right, absolute positioning)
	if m.toasts.HasToasts() {
		content = m.overlayToasts(content)
	}

	return content
}

// renderTooSmall renders a message when terminal is too small.
func (m Model) renderTooSmall() string {
	msg := styles.WarningStyle.Render("Terminal too small\nMinimum: 80x24")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
}

// renderMainContent renders the main content area with responsive layout.
// Layout: top section (clients + control) and bottom section (log stream).
func (m Model) renderMainContent(height, width int) string {
	// Reserve space for log pane at bottom
	logHeight := LogPaneHeight
	topHeight := height - logHeight

	if topHeight < 6 {
		topHeight = 6
		logHeight = height - topHeight
	}

	// Render top section (responsive horizontal/vertical)
	var topSection string
	if width >= MinPaneWidth*2+4 {
		topSection = m.renderHorizontalLayout(topHeight, width)
	} else {
		topSection = m.renderVerticalLayout(topHeight, width)
	}

	// Render log stream pane
	logPane := m.renderLogPane(logHeight, width)

	return lipgloss.JoinVertical(lipgloss.Left, topSection, logPane)
}

// renderHorizontalLayout renders two panes side by side.
func (m Model) renderHorizontalLayout(height, width int) string {
	leftWidth := width / 2
	rightWidth := width - leftWidth

	// Render pane contents
	leftContent := m.clientList.View()
	rightContent := m.renderControlPanel()

	// Create bordered panes with titles
	leftPane := m.renderPane(
		styles.IconUsers+" Connected Clients",
		leftContent,
		leftWidth,
		height,
	)
	rightPane := m.renderPane(
		styles.IconMouse+" Control Status",
		rightContent,
		rightWidth,
		height,
	)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

// renderVerticalLayout renders two panes stacked vertically (50/50).
func (m Model) renderVerticalLayout(height, width int) string {
	topHeight := height / 2
	bottomHeight := height - topHeight

	// Render pane contents
	topContent := m.clientList.View()
	bottomContent := m.renderControlPanel()

	// Create bordered panes with titles
	topPane := m.renderPane(
		styles.IconUsers+" Connected Clients",
		topContent,
		width,
		topHeight,
	)
	bottomPane := m.renderPane(
		styles.IconMouse+" Control Status",
		bottomContent,
		width,
		bottomHeight,
	)

	return lipgloss.JoinVertical(lipgloss.Left, topPane, bottomPane)
}

// renderPane renders a bordered pane with title.
func (m Model) renderPane(title, content string, width, height int) string {
	// Account for border (2) and padding (2)
	innerWidth := width - 4
	innerHeight := height - 4

	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	// Title style
	titleRendered := styles.TitleStyle.Render(title)

	// Content area height (subtract title height)
	titleHeight := lipgloss.Height(titleRendered)
	contentHeight := innerHeight - titleHeight - 1 // -1 for spacing

	if contentHeight < 1 {
		contentHeight = 1
	}

	// Style content to fit within bounds
	contentStyle := lipgloss.NewStyle().
		Width(innerWidth).
		Height(contentHeight)

	styledContent := contentStyle.Render(content)

	// Combine title and content
	inner := lipgloss.JoinVertical(lipgloss.Left, titleRendered, "", styledContent)

	// Apply pane style with border
	paneStyle := lipgloss.NewStyle().
		Width(width - 2).
		Height(height - 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Border).
		Padding(1)

	return paneStyle.Render(inner)
}

// renderLoading renders the loading state.
func (m Model) renderLoading(height, width int) string {
	content := styles.IconSpinner + " Starting server...\n\n" +
		styles.MutedStyle.Render("Waiting for connections")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

// renderError renders the error state.
func (m Model) renderError(height, width int) string {
	errorContent := styles.IconCross + " Error\n\n" +
		styles.ErrorStyle.Render(m.err.Error()) + "\n\n" +
		styles.MutedStyle.Render("Press 'esc' to dismiss")

	errorBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Error).
		Padding(1, 2).
		Width(minInt(width-10, 60))

	boxed := errorBox.Render(errorContent)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, boxed)
}

// renderLogPane renders the log stream pane at the bottom.
func (m Model) renderLogPane(height, width int) string {
	// Title
	title := styles.TitleStyle.Render(styles.IconInfo + " Logs")

	// Get log stream content
	logContent := m.logStream.View()
	if logContent == "" {
		logContent = styles.MutedStyle.Render("No logs yet...")
	}

	// Calculate inner dimensions
	innerWidth := width - 4
	innerHeight := height - 4

	if innerHeight < 1 {
		innerHeight = 1
	}

	// Title height
	titleHeight := lipgloss.Height(title)
	contentHeight := innerHeight - titleHeight - 1

	if contentHeight < 1 {
		contentHeight = 1
	}

	// Style content
	contentStyle := lipgloss.NewStyle().
		Width(innerWidth).
		Height(contentHeight)

	styledContent := contentStyle.Render(logContent)

	// Combine title and content
	inner := lipgloss.JoinVertical(lipgloss.Left, title, "", styledContent)

	// Apply pane style
	paneStyle := lipgloss.NewStyle().
		Width(width - 2).
		Height(height - 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Border).
		Padding(1)

	return paneStyle.Render(inner)
}

// renderControlPanel renders the control status panel content.
func (m Model) renderControlPanel() string {
	var lines []string

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

// overlayToasts overlays toast notifications at the top-right of the content.
func (m Model) overlayToasts(base string) string {
	toastView := m.toasts.View()
	if toastView == "" {
		return base
	}

	toastWidth := lipgloss.Width(toastView)
	toastHeight := lipgloss.Height(toastView)

	// Position: top-right with margin
	startX := m.width - toastWidth - 2
	startY := 1

	if startX < 0 {
		startX = 0
	}

	// Split base into lines
	baseLines := strings.Split(base, "\n")
	toastLines := strings.Split(toastView, "\n")

	// Overlay toast lines onto base
	for i := 0; i < toastHeight && i < len(toastLines); i++ {
		lineIdx := startY + i
		if lineIdx >= 0 && lineIdx < len(baseLines) {
			baseLines[lineIdx] = overlayStringAt(baseLines[lineIdx], toastLines[i], startX)
		}
	}

	return strings.Join(baseLines, "\n")
}

// overlayStringAt places overlay string on top of base string at position x.
func overlayStringAt(base, overlay string, x int) string {
	baseRunes := []rune(base)
	overlayRunes := []rune(overlay)

	// Ensure base is long enough
	for len(baseRunes) < x+len(overlayRunes) {
		baseRunes = append(baseRunes, ' ')
	}

	// Copy overlay runes at position x
	for i, r := range overlayRunes {
		if x+i < len(baseRunes) {
			baseRunes[x+i] = r
		}
	}

	return string(baseRunes)
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

// minInt returns the minimum of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
