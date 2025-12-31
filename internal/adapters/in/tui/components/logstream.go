// Package components provides reusable TUI components.
package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
)

// LogLevel represents the severity of a log entry.
type LogLevel string

const (
	LogLevelDebug LogLevel = "DBG"
	LogLevelInfo  LogLevel = "INF"
	LogLevelWarn  LogLevel = "WRN"
	LogLevelError LogLevel = "ERR"
)

// LogEntry represents a single log entry.
type LogEntry struct {
	Time    time.Time
	Level   LogLevel
	Message string
	Fields  map[string]string
}

// LogStreamMsg is sent when new log entries are available.
type LogStreamMsg struct {
	Entries []LogEntry
}

// LogStream displays a scrollable log viewer.
type LogStream struct {
	viewport viewport.Model
	entries  []LogEntry
	maxLines int
	width    int
	height   int
	ready    bool

	// Auto-scroll to bottom when new logs arrive
	autoScroll bool
}

// NewLogStream creates a new log stream component.
func NewLogStream() LogStream {
	return LogStream{
		entries:    make([]LogEntry, 0),
		maxLines:   500,
		autoScroll: true,
	}
}

// WithSize sets the log stream dimensions.
func (l LogStream) WithSize(width, height int) LogStream {
	l.width = width
	l.height = height

	if !l.ready {
		l.viewport = viewport.New(width, height)
		l.viewport.Style = lipgloss.NewStyle()
		l.ready = true
	} else {
		l.viewport.Width = width
		l.viewport.Height = height
	}

	// Re-render content with new width
	l.viewport.SetContent(l.renderContent())

	return l
}

// WithMaxLines sets the maximum number of log lines to keep.
func (l LogStream) WithMaxLines(max int) LogStream {
	l.maxLines = max
	return l
}

// AddEntry adds a log entry to the stream.
func (l LogStream) AddEntry(entry LogEntry) LogStream {
	l.entries = append(l.entries, entry)

	// Trim old entries if over max
	if len(l.entries) > l.maxLines {
		l.entries = l.entries[len(l.entries)-l.maxLines:]
	}

	// Update viewport content
	l.viewport.SetContent(l.renderContent())

	// Auto-scroll to bottom
	if l.autoScroll {
		l.viewport.GotoBottom()
	}

	return l
}

// AddEntries adds multiple log entries.
func (l LogStream) AddEntries(entries []LogEntry) LogStream {
	for _, entry := range entries {
		l = l.AddEntry(entry)
	}
	return l
}

// Clear removes all log entries.
func (l LogStream) Clear() LogStream {
	l.entries = make([]LogEntry, 0)
	l.viewport.SetContent("")
	return l
}

// Update handles viewport updates.
func (l LogStream) Update(msg tea.Msg) (LogStream, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "home", "g":
			l.viewport.GotoTop()
			l.autoScroll = false
		case "end", "G":
			l.viewport.GotoBottom()
			l.autoScroll = true
		case "up", "k":
			l.viewport.ScrollUp(1)
			l.autoScroll = false
		case "down", "j":
			l.viewport.ScrollDown(1)
			// Re-enable auto-scroll if at bottom
			if l.viewport.AtBottom() {
				l.autoScroll = true
			}
		case "pgup":
			l.viewport.HalfPageUp()
			l.autoScroll = false
		case "pgdown":
			l.viewport.HalfPageDown()
			if l.viewport.AtBottom() {
				l.autoScroll = true
			}
		}

	case LogStreamMsg:
		l = l.AddEntries(msg.Entries)
	}

	l.viewport, cmd = l.viewport.Update(msg)
	return l, cmd
}

// View renders the log stream.
func (l LogStream) View() string {
	if !l.ready {
		return ""
	}

	return l.viewport.View()
}

// renderContent renders all log entries as a single string.
func (l LogStream) renderContent() string {
	if len(l.entries) == 0 {
		return styles.MutedStyle.Render("No logs yet...")
	}

	var lines []string
	for _, entry := range l.entries {
		line := l.renderEntry(entry)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderEntry renders a single log entry.
func (l LogStream) renderEntry(entry LogEntry) string {
	// Time
	timeStr := entry.Time.Format("15:04:05")
	timeStyled := styles.MutedStyle.Render(timeStr)

	// Level with color
	var levelStyled string
	switch entry.Level {
	case LogLevelDebug:
		levelStyled = styles.MutedStyle.Render(string(entry.Level))
	case LogLevelInfo:
		levelStyled = styles.InfoStyle.Render(string(entry.Level))
	case LogLevelWarn:
		levelStyled = styles.WarningStyle.Render(string(entry.Level))
	case LogLevelError:
		levelStyled = styles.ErrorStyle.Render(string(entry.Level))
	default:
		levelStyled = string(entry.Level)
	}

	// Message
	msg := entry.Message

	// Fields (if any)
	var fieldsStr string
	if len(entry.Fields) > 0 {
		var fieldParts []string
		for k, v := range entry.Fields {
			fieldParts = append(fieldParts, fmt.Sprintf("%s=%s",
				styles.MutedStyle.Render(k),
				styles.ValueStyle.Render(v)))
		}
		fieldsStr = " " + strings.Join(fieldParts, " ")
	}

	// Truncate if too wide
	maxMsgWidth := l.width - 20 // Leave room for time, level, spacing
	if maxMsgWidth > 0 && len(msg) > maxMsgWidth {
		msg = msg[:maxMsgWidth-3] + "..."
	}

	return fmt.Sprintf("%s %s %s%s", timeStyled, levelStyled, msg, fieldsStr)
}

// GetViewport returns the underlying viewport for direct manipulation.
func (l LogStream) GetViewport() viewport.Model {
	return l.viewport
}

// IsAtBottom returns true if the viewport is scrolled to the bottom.
func (l LogStream) IsAtBottom() bool {
	return l.viewport.AtBottom()
}

// EntryCount returns the number of log entries.
func (l LogStream) EntryCount() int {
	return len(l.entries)
}
