// Package components provides reusable TUI components.
package components

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
)

// Header displays the application title and status in a header bar.
type Header struct {
	title    string
	subtitle string
	icon     string
	status   string
	width    int
}

// NewHeader creates a new header with the given title.
func NewHeader(title string) Header {
	return Header{
		title: title,
		icon:  styles.IconServer,
		width: 80,
	}
}

// WithSubtitle sets the header subtitle.
func (h Header) WithSubtitle(subtitle string) Header {
	h.subtitle = subtitle
	return h
}

// WithIcon sets the header icon.
func (h Header) WithIcon(icon string) Header {
	h.icon = icon
	return h
}

// WithStatus sets the status text displayed on the right.
func (h Header) WithStatus(status string) Header {
	h.status = status
	return h
}

// WithWidth sets the header width.
func (h Header) WithWidth(width int) Header {
	h.width = width
	return h
}

// View renders the header.
func (h Header) View() string {
	// Title with icon
	titleText := h.title
	if h.icon != "" {
		titleText = h.icon + " " + titleText
	}
	title := styles.TitleStyle.Render(titleText)

	// Subtitle (if any)
	var subtitle string
	if h.subtitle != "" {
		subtitle = styles.SubtitleStyle.Render(h.subtitle)
	}

	// Left side: title and subtitle
	leftContent := title
	if subtitle != "" {
		leftContent = lipgloss.JoinVertical(lipgloss.Left, title, subtitle)
	}

	// Right side: status
	var rightContent string
	if h.status != "" {
		rightContent = styles.MutedStyle.Render(h.status)
	}

	// Calculate spacing
	leftWidth := lipgloss.Width(leftContent)
	rightWidth := lipgloss.Width(rightContent)
	spacerWidth := h.width - leftWidth - rightWidth - 4 // 4 for padding
	if spacerWidth < 1 {
		spacerWidth = 1
	}

	// Build the header row
	spacer := lipgloss.NewStyle().Width(spacerWidth).Render("")
	row := lipgloss.JoinHorizontal(lipgloss.Center, leftContent, spacer, rightContent)

	// Apply header style
	headerStyle := styles.HeaderStyle.Width(h.width)
	return headerStyle.Render(row)
}

// ServerHeader creates a header configured for server mode.
func ServerHeader(width int) Header {
	return NewHeader("Waymon Server").
		WithIcon(styles.IconServer).
		WithWidth(width)
}

// ClientHeader creates a header configured for client mode.
func ClientHeader(width int) Header {
	return NewHeader("Waymon Client").
		WithIcon(styles.IconClient).
		WithWidth(width)
}
