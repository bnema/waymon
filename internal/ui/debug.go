package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DebugServerModel is a minimal TUI for debugging
type DebugServerModel struct {
	logs      []string
	maxLogs   int
	height    int
	width     int
	startTime time.Time
}

// NewDebugServerModel creates a debug server UI
func NewDebugServerModel() *DebugServerModel {
	return &DebugServerModel{
		logs:      make([]string, 0),
		maxLogs:   50,
		startTime: time.Now(),
	}
}

func (m *DebugServerModel) Init() tea.Cmd {
	// Send a tick every second to show we're alive
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

type TickMsg time.Time

func (m *DebugServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case LogMsg:
		entry := fmt.Sprintf("[%s] %s: %s",
			msg.Entry.Timestamp.Format("15:04:05"),
			msg.Entry.Level,
			msg.Entry.Message,
		)
		m.logs = append(m.logs, entry)
		if len(m.logs) > m.maxLogs {
			m.logs = m.logs[1:]
		}

	case TickMsg:
		// Continue ticking
		return m, tea.Every(time.Second, func(t time.Time) tea.Msg {
			return TickMsg(t)
		})
	}

	return m, nil
}

func (m *DebugServerModel) View() string {
	var b strings.Builder

	// Header - uniform format using common styling
	elapsed := time.Since(m.startTime).Round(time.Second)
	statusText := fmt.Sprintf("Running for %s", elapsed)
	header := FormatAppHeader("DEBUG MODE", statusText)
	b.WriteString(header)
	b.WriteString("\n\n")

	// Logs
	if len(m.logs) == 0 {
		b.WriteString("No logs yet...\n")
	} else {
		for _, log := range m.logs {
			b.WriteString(log)
			b.WriteString("\n")
		}
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("[q] quit"))

	return b.String()
}
