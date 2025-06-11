package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StatusBar represents a reusable status bar component
type StatusBar struct {
	Width      int
	Title      string
	Status     string
	Connected  bool
	ShowSpinner bool
	spinner    spinner.Model
}

// NewStatusBar creates a new status bar
func NewStatusBar(title string) *StatusBar {
	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: SpinnerDot,
		FPS:    time.Second / 10,
	}
	s.Style = SpinnerStyle

	return &StatusBar{
		Title:      title,
		ShowSpinner: true,
		spinner:    s,
	}
}

// Init implements tea.Model
func (s *StatusBar) Init() tea.Cmd {
	return s.spinner.Tick
}

// Update implements tea.Model
func (s *StatusBar) Update(msg tea.Msg) (*StatusBar, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd
	case tea.WindowSizeMsg:
		s.Width = msg.Width
	}
	return s, nil
}

// View renders the status bar
func (s *StatusBar) View() string {
	title := TitleStyle.Render(s.Title)
	
	var status string
	if s.ShowSpinner {
		status = s.spinner.View() + " " + s.Status
	} else {
		status = s.Status
	}
	
	statusFormatted := FormatStatus(s.Connected, status)
	
	// Create a line that spans the width
	gap := s.Width - lipgloss.Width(title) - lipgloss.Width(statusFormatted) - 2
	if gap < 0 {
		gap = 0
	}
	
	line := title + strings.Repeat(" ", gap) + statusFormatted
	
	return BoxStyle.Width(s.Width).Render(line)
}

// InfoPanel represents a panel with information
type InfoPanel struct {
	Title   string
	Content []string
	Width   int
}

// View renders the info panel
func (p *InfoPanel) View() string {
	var b strings.Builder
	
	if p.Title != "" {
		b.WriteString(SubheaderStyle.Render(p.Title))
		b.WriteString("\n")
	}
	
	for _, line := range p.Content {
		b.WriteString(TextStyle.Render(line))
		b.WriteString("\n")
	}
	
	return BoxStyle.Width(p.Width).Render(b.String())
}

// ConnectionList shows a list of connections
type ConnectionList struct {
	Title       string
	Connections []Connection
	Width       int
}

// Connection represents a connection entry
type Connection struct {
	Name      string
	Address   string
	Connected bool
	Active    bool
}

// View renders the connection list
func (c *ConnectionList) View() string {
	var b strings.Builder
	
	b.WriteString(SubheaderStyle.Render(c.Title))
	b.WriteString("\n\n")
	
	if len(c.Connections) == 0 {
		b.WriteString(MutedStyle.Render("No connections"))
	} else {
		for _, conn := range c.Connections {
			indicator := DisconnectedIndicator
			style := ListItemStyle
			
			if conn.Connected {
				indicator = ConnectedIndicator
			}
			if conn.Active {
				style = style.Copy().Foreground(ColorActive)
			}
			
			line := fmt.Sprintf("%s %s (%s)", 
				indicator, 
				style.Render(conn.Name), 
				SubtleStyle.Render(conn.Address))
			
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	
	return BoxStyle.Width(c.Width).Render(b.String())
}

// MonitorInfo displays monitor configuration
type MonitorInfo struct {
	Monitors []Monitor
	Width    int
}

// Monitor represents display information
type Monitor struct {
	Name    string
	Size    string
	Position string
	Primary bool
}

// View renders the monitor info
func (m *MonitorInfo) View() string {
	var b strings.Builder
	
	b.WriteString(SubheaderStyle.Render(fmt.Sprintf("Detected %d monitor(s):", len(m.Monitors))))
	b.WriteString("\n\n")
	
	for i, mon := range m.Monitors {
		name := mon.Name
		if mon.Primary {
			name += " " + InfoStyle.Render("(primary)")
		}
		
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, BoldStyle.Render(name)))
		b.WriteString(fmt.Sprintf("   %s at %s\n", 
			TextStyle.Render(mon.Size),
			SubtleStyle.Render(mon.Position)))
		
		if i < len(m.Monitors)-1 {
			b.WriteString("\n")
		}
	}
	
	return BoxStyle.Width(m.Width).Render(b.String())
}

// ControlsHelp displays keyboard controls
type ControlsHelp struct {
	Controls []Control
	Width    int
}

// Control represents a keyboard control
type Control struct {
	Key  string
	Desc string
}

// View renders the controls help
func (c *ControlsHelp) View() string {
	var b strings.Builder
	
	b.WriteString(SubheaderStyle.Render("Controls:"))
	b.WriteString("\n\n")
	
	maxKeyLen := 0
	for _, ctrl := range c.Controls {
		if len(ctrl.Key) > maxKeyLen {
			maxKeyLen = len(ctrl.Key)
		}
	}
	
	for _, ctrl := range c.Controls {
		key := ControlKeyStyle.Width(maxKeyLen).Render(ctrl.Key)
		desc := ControlDescStyle.Render(ctrl.Desc)
		b.WriteString(fmt.Sprintf("  %s  %s\n", key, desc))
	}
	
	return BoxStyle.Width(c.Width).Render(b.String())
}

// ProgressIndicator shows progress
type ProgressIndicator struct {
	Label    string
	Current  int
	Total    int
	Width    int
	ShowPercentage bool
}

// View renders the progress indicator
func (p *ProgressIndicator) View() string {
	percentage := float64(p.Current) / float64(p.Total)
	if percentage > 1.0 {
		percentage = 1.0
	}
	
	barWidth := p.Width - len(p.Label) - 10 // Leave space for label and percentage
	if barWidth < 10 {
		barWidth = 10
	}
	
	filled := int(float64(barWidth) * percentage)
	empty := barWidth - filled
	
	bar := SuccessStyle.Render(strings.Repeat("█", filled)) +
		MutedStyle.Render(strings.Repeat("░", empty))
	
	var result string
	if p.ShowPercentage {
		result = fmt.Sprintf("%s %s %3.0f%%", 
			TextStyle.Render(p.Label), 
			bar, 
			percentage*100)
	} else {
		result = fmt.Sprintf("%s %s", 
			TextStyle.Render(p.Label), 
			bar)
	}
	
	return result
}

// Message displays a styled message
type Message struct {
	Type    MessageType
	Content string
}

// MessageType represents the type of message
type MessageType int

const (
	MessageInfo MessageType = iota
	MessageSuccess
	MessageWarning
	MessageError
)

// View renders the message
func (m *Message) View() string {
	var style lipgloss.Style
	var prefix string
	
	switch m.Type {
	case MessageSuccess:
		style = SuccessStyle
		prefix = "✓ "
	case MessageWarning:
		style = WarningStyle
		prefix = "⚠ "
	case MessageError:
		style = ErrorStyle
		prefix = "✗ "
	default:
		style = InfoStyle
		prefix = "ℹ "
	}
	
	return style.Render(prefix + m.Content)
}