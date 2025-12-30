// Package components provides reusable TUI components.
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
)

// Binding represents a key binding with its description.
type Binding struct {
	Key  string
	Desc string
}

// Help displays keyboard shortcuts and help information.
type Help struct {
	bindings []Binding
	width    int
	compact  bool
}

// NewHelp creates a new help component.
func NewHelp() Help {
	return Help{
		bindings: make([]Binding, 0),
		width:    80,
		compact:  true,
	}
}

// WithBindings sets the key bindings to display.
func (h Help) WithBindings(bindings ...Binding) Help {
	h.bindings = bindings
	return h
}

// WithWidth sets the help component width.
func (h Help) WithWidth(width int) Help {
	h.width = width
	return h
}

// WithCompact sets whether to use compact display mode.
func (h Help) WithCompact(compact bool) Help {
	h.compact = compact
	return h
}

// AddBinding adds a key binding to the help.
func (h Help) AddBinding(key, desc string) Help {
	h.bindings = append(h.bindings, Binding{Key: key, Desc: desc})
	return h
}

// View renders the help component.
func (h Help) View() string {
	if len(h.bindings) == 0 {
		return ""
	}

	if h.compact {
		return h.renderCompact()
	}
	return h.renderFull()
}

// renderCompact renders bindings in a single line.
func (h Help) renderCompact() string {
	var parts []string
	for _, b := range h.bindings {
		key := styles.HelpKeyStyle.Render(b.Key)
		desc := styles.HelpDescStyle.Render(b.Desc)
		parts = append(parts, key+" "+desc)
	}

	separator := styles.HelpSepStyle.Render(" • ")
	return strings.Join(parts, separator)
}

// renderFull renders bindings in a vertical list.
func (h Help) renderFull() string {
	var lines []string
	for _, b := range h.bindings {
		key := styles.HelpKeyStyle.Width(12).Render(b.Key)
		desc := styles.HelpDescStyle.Render(b.Desc)
		lines = append(lines, key+desc)
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// Common key bindings for different modes.

// ServerBindings returns common bindings for server mode.
func ServerBindings() []Binding {
	return []Binding{
		{Key: "q", Desc: "quit"},
		{Key: "tab", Desc: "next client"},
		{Key: "shift+tab", Desc: "prev client"},
		{Key: "enter", Desc: "switch to client"},
		{Key: "esc", Desc: "release control"},
		{Key: "ctrl+r", Desc: "emergency release"},
		{Key: "?", Desc: "help"},
	}
}

// ClientBindings returns common bindings for client mode.
func ClientBindings() []Binding {
	return []Binding{
		{Key: "q", Desc: "quit"},
		{Key: "r", Desc: "reconnect"},
		{Key: "?", Desc: "help"},
	}
}

// NavigationBindings returns navigation bindings.
func NavigationBindings() []Binding {
	return []Binding{
		{Key: "↑/k", Desc: "up"},
		{Key: "↓/j", Desc: "down"},
		{Key: "enter", Desc: "select"},
		{Key: "esc", Desc: "back"},
	}
}

// FullHelp creates a full help view with title.
func FullHelp(title string, bindings []Binding, width int) string {
	titleLine := styles.TitleStyle.Render(styles.IconHelp + " " + title)

	help := NewHelp().
		WithBindings(bindings...).
		WithWidth(width).
		WithCompact(false)

	content := help.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		titleLine,
		"",
		content,
	)
}
