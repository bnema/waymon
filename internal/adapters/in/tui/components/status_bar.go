// Package components provides reusable TUI components.
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
)

// StatusItem represents a single item in the status bar.
type StatusItem struct {
	Label string
	Value string
	Icon  string
}

// StatusBar displays status information at the bottom of the screen.
type StatusBar struct {
	items []StatusItem
	width int
}

// NewStatusBar creates a new empty status bar.
func NewStatusBar() StatusBar {
	return StatusBar{
		items: make([]StatusItem, 0),
		width: 80,
	}
}

// WithWidth sets the status bar width.
func (s StatusBar) WithWidth(width int) StatusBar {
	s.width = width
	return s
}

// WithItems sets the status bar items.
func (s StatusBar) WithItems(items ...StatusItem) StatusBar {
	s.items = items
	return s
}

// AddItem adds an item to the status bar.
func (s StatusBar) AddItem(item StatusItem) StatusBar {
	s.items = append(s.items, item)
	return s
}

// View renders the status bar.
func (s StatusBar) View() string {
	if len(s.items) == 0 {
		return ""
	}

	var parts []string
	for _, item := range s.items {
		part := s.renderItem(item)
		parts = append(parts, part)
	}

	separator := styles.HelpSepStyle.Render(" | ")
	content := strings.Join(parts, separator)

	barStyle := styles.StatusBarStyle.Width(s.width)
	return barStyle.Render(content)
}

// renderItem renders a single status item.
func (s StatusBar) renderItem(item StatusItem) string {
	var parts []string

	if item.Icon != "" {
		parts = append(parts, item.Icon)
	}

	if item.Label != "" {
		label := styles.LabelStyle.Render(item.Label + ":")
		parts = append(parts, label)
	}

	if item.Value != "" {
		value := styles.ValueStyle.Render(item.Value)
		parts = append(parts, value)
	}

	return strings.Join(parts, " ")
}

// Convenience functions for creating common status items.

// ClientsItem creates a status item showing client count.
func ClientsItem(count int, icon string) StatusItem {
	if icon == "" {
		icon = styles.IconUsers
	}
	return StatusItem{
		Icon:  icon,
		Label: "Clients",
		Value: formatCount(count),
	}
}

// ActiveItem creates a status item showing the active client.
func ActiveItem(name string) StatusItem {
	return StatusItem{
		Icon:  styles.IconMouse,
		Label: "Active",
		Value: name,
	}
}

// ModeItem creates a status item showing the current mode.
func ModeItem(mode string) StatusItem {
	return StatusItem{
		Icon:  styles.IconGear,
		Label: "Mode",
		Value: mode,
	}
}

// PortItem creates a status item showing the port number.
func PortItem(port int) StatusItem {
	return StatusItem{
		Icon:  styles.IconNetwork,
		Label: "Port",
		Value: formatPort(port),
	}
}

// UptimeItem creates a status item showing uptime.
func UptimeItem(uptime string) StatusItem {
	return StatusItem{
		Icon:  styles.IconClock,
		Label: "Uptime",
		Value: uptime,
	}
}

// formatCount formats a count as a string.
func formatCount(count int) string {
	return strings.TrimSpace(lipgloss.NewStyle().Render(formatInt(count)))
}

// formatPort formats a port number as a string.
func formatPort(port int) string {
	return strings.TrimSpace(lipgloss.NewStyle().Render(formatInt(port)))
}

// formatInt converts an int to string.
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
