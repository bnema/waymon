// Package monitors provides the monitors command TUI view.
// This view displays detected monitors and their configuration.
package monitors

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bnema/waymon/internal/adapters/in/tui/components"
	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
	"github.com/bnema/waymon/internal/domain"
)

// Model is the Bubble Tea model for the monitors view.
type Model struct {
	// Monitor data
	monitors []domain.Monitor

	// UI state
	err            error
	loading        bool
	selectedIndex  int
	width          int
	height         int
	monitorDisplay components.MonitorDisplay
}

// New creates a new monitors view model.
func New() Model {
	return Model{
		monitors:       make([]domain.Monitor, 0),
		loading:        true,
		selectedIndex:  0,
		width:          80,
		height:         24,
		monitorDisplay: components.NewMonitorDisplay(),
	}
}

// SetMonitors sets the monitors to display.
func (m Model) SetMonitors(monitors []domain.Monitor) Model {
	m.monitors = monitors
	m.loading = false
	m.monitorDisplay = m.monitorDisplay.WithMonitors(monitors)
	return m
}

// SetError sets an error state.
func (m Model) SetError(err error) Model {
	m.err = err
	m.loading = false
	return m
}

// SetLoading sets the loading state.
func (m Model) SetLoading(loading bool) Model {
	m.loading = loading
	return m
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.monitorDisplay = m.monitorDisplay.WithSize(m.width-4, m.height/2)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit

		case "j", "down":
			if len(m.monitors) > 0 {
				m.selectedIndex = (m.selectedIndex + 1) % len(m.monitors)
				m.monitorDisplay = m.monitorDisplay.WithHighlighted(m.monitors[m.selectedIndex].ID)
			}

		case "k", "up":
			if len(m.monitors) > 0 {
				m.selectedIndex = (m.selectedIndex - 1 + len(m.monitors)) % len(m.monitors)
				m.monitorDisplay = m.monitorDisplay.WithHighlighted(m.monitors[m.selectedIndex].ID)
			}
		}
	}

	return m, nil
}

// View renders the monitors view.
func (m Model) View() string {
	if m.loading {
		return m.renderLoading()
	}

	if m.err != nil {
		return m.renderError()
	}

	var lines []string

	// Header
	header := components.NewHeader("Monitor Configuration").
		WithIcon(styles.IconMonitor).
		WithWidth(m.width)
	lines = append(lines, header.View())
	lines = append(lines, "")

	// Monitor display visualization
	if len(m.monitors) > 0 {
		m.monitorDisplay = m.monitorDisplay.
			WithMonitors(m.monitors).
			WithSize(m.width-4, m.height/3)
		lines = append(lines, m.monitorDisplay.View())
		lines = append(lines, "")

		// Detailed monitor list
		lines = append(lines, m.renderMonitorDetails())
	} else {
		lines = append(lines, m.renderNoMonitors())
	}

	lines = append(lines, "")
	lines = append(lines, m.renderHelp())

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderLoading renders the loading state.
func (m Model) renderLoading() string {
	style := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height-6).
		Align(lipgloss.Center, lipgloss.Center)

	return style.Render(
		styles.IconSpinner + " Detecting monitors...\n\n" +
			styles.MutedStyle.Render("This may take a moment"),
	)
}

// renderError renders the error state.
func (m Model) renderError() string {
	errorBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Error).
		Padding(1, 2)

	content := styles.IconCross + " Failed to detect monitors\n\n" +
		styles.ErrorStyle.Render(m.err.Error()) + "\n\n" +
		styles.MutedStyle.Render("Press 'q' to exit")

	return errorBox.Render(content)
}

// renderNoMonitors renders when no monitors are detected.
func (m Model) renderNoMonitors() string {
	style := lipgloss.NewStyle().
		Width(m.width).
		Padding(2, 2).
		Align(lipgloss.Center)

	return style.Render(
		styles.IconWarning + " No monitors detected\n\n" +
			styles.MutedStyle.Render("Make sure you have a display backend available"),
	)
}

// renderMonitorDetails renders the detailed monitor list.
func (m Model) renderMonitorDetails() string {
	var lines []string

	lines = append(lines, styles.LabelStyle.Render("Monitor Details"))
	lines = append(lines, "")

	for i, mon := range m.monitors {
		lines = append(lines, m.renderMonitorEntry(i, mon))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderMonitorEntry renders a single monitor entry.
func (m Model) renderMonitorEntry(index int, mon domain.Monitor) string {
	isSelected := index == m.selectedIndex

	// Build the entry
	var entryStyle lipgloss.Style
	bullet := "  "
	if isSelected {
		entryStyle = styles.SelectedStyle
		bullet = styles.IconChevronR + " "
	} else {
		entryStyle = lipgloss.NewStyle()
	}

	// Monitor name/ID
	name := mon.Name
	if name == "" {
		name = mon.ID
	}
	if mon.Primary {
		name += " (Primary)"
	}

	// First line: index and name
	firstLine := bullet + entryStyle.Render(fmt.Sprintf("%d. %s %s",
		index+1,
		styles.IconMonitor,
		name,
	))

	// Details
	details := fmt.Sprintf("     Resolution: %dx%d", mon.Width, mon.Height)
	if mon.RefreshRate > 0 {
		details += fmt.Sprintf(" @ %dHz", mon.RefreshRate)
	}
	details += fmt.Sprintf("\n     Position: (%d, %d)", mon.X, mon.Y)
	if mon.Scale != 0 && mon.Scale != 1.0 {
		details += fmt.Sprintf("\n     Scale: %.1f", mon.Scale)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		firstLine,
		styles.MutedStyle.Render(details),
	)
}

// renderHelp renders the help footer.
func (m Model) renderHelp() string {
	help := components.NewHelp().WithBindings(
		components.Binding{Key: "j/k", Desc: "navigate"},
		components.Binding{Key: "q", Desc: "quit"},
	)
	return help.View()
}

// RenderStatic renders the monitors as a static string (for non-TUI output).
func (m Model) RenderStatic() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %s\n", m.err.Error())
	}

	if len(m.monitors) == 0 {
		return "No monitors detected\n"
	}

	var output string
	output += fmt.Sprintf("Monitors detected: %d\n\n", len(m.monitors))

	for i, mon := range m.monitors {
		name := mon.Name
		if name == "" {
			name = mon.ID
		}

		primary := ""
		if mon.Primary {
			primary = " [primary]"
		}

		output += fmt.Sprintf("%d. %s%s\n", i+1, name, primary)
		output += fmt.Sprintf("   Resolution: %dx%d", mon.Width, mon.Height)
		if mon.RefreshRate > 0 {
			output += fmt.Sprintf(" @ %dHz", mon.RefreshRate)
		}
		output += "\n"
		output += fmt.Sprintf("   Position: (%d, %d)\n", mon.X, mon.Y)
		if mon.Scale != 0 && mon.Scale != 1.0 {
			output += fmt.Sprintf("   Scale: %.1f\n", mon.Scale)
		}
		output += "\n"
	}

	return output
}
