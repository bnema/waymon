// Package components provides reusable TUI components.
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
)

// Footer displays status information and help bindings at the bottom of the screen.
type Footer struct {
	statusItems []StatusItem
	bindings    []Binding
	width       int
}

// NewFooter creates a new footer component.
func NewFooter() Footer {
	return Footer{
		statusItems: make([]StatusItem, 0),
		bindings:    make([]Binding, 0),
		width:       80,
	}
}

// WithWidth sets the footer width.
func (f Footer) WithWidth(width int) Footer {
	f.width = width
	return f
}

// WithStatusItems sets the status items to display.
func (f Footer) WithStatusItems(items ...StatusItem) Footer {
	f.statusItems = items
	return f
}

// WithBindings sets the help bindings to display.
func (f Footer) WithBindings(bindings ...Binding) Footer {
	f.bindings = bindings
	return f
}

// View renders the footer.
func (f Footer) View() string {
	// Render status line
	statusLine := f.renderStatusLine()

	// Render help line
	helpLine := f.renderHelpLine()

	// Join with proper styling
	content := lipgloss.JoinVertical(lipgloss.Left, statusLine, helpLine)

	// Apply footer style with border-top
	footerStyle := lipgloss.NewStyle().
		Width(f.width).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(styles.Border).
		PaddingLeft(1).
		PaddingRight(1)

	return footerStyle.Render(content)
}

// renderStatusLine renders the status items line.
func (f Footer) renderStatusLine() string {
	if len(f.statusItems) == 0 {
		return ""
	}

	var parts []string
	for _, item := range f.statusItems {
		part := f.renderStatusItem(item)
		parts = append(parts, part)
	}

	separator := styles.HelpSepStyle.Render(" | ")
	return strings.Join(parts, separator)
}

// renderStatusItem renders a single status item.
func (f Footer) renderStatusItem(item StatusItem) string {
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

// renderHelpLine renders the help bindings line.
func (f Footer) renderHelpLine() string {
	if len(f.bindings) == 0 {
		return ""
	}

	var parts []string
	for _, b := range f.bindings {
		key := styles.HelpKeyStyle.Render(b.Key)
		desc := styles.HelpDescStyle.Render(b.Desc)
		parts = append(parts, key+" "+desc)
	}

	separator := styles.HelpSepStyle.Render(" • ")
	return strings.Join(parts, separator)
}

// ServerFooter creates a footer configured for server mode.
func ServerFooter(width int) Footer {
	return NewFooter().
		WithWidth(width).
		WithBindings(ServerBindings()...)
}

// ClientFooter creates a footer configured for client mode.
func ClientFooter(width int) Footer {
	return NewFooter().
		WithWidth(width).
		WithBindings(ClientBindings()...)
}
