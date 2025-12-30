// Package components provides reusable TUI components.
package components

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
)

// SpinnerStyle defines the visual style of the spinner.
type SpinnerStyle int

const (
	// SpinnerDot uses a dot spinner.
	SpinnerDot SpinnerStyle = iota
	// SpinnerLine uses a line spinner.
	SpinnerLine
	// SpinnerMiniDot uses a mini dot spinner.
	SpinnerMiniDot
	// SpinnerJump uses a jumping spinner.
	SpinnerJump
	// SpinnerPulse uses a pulsing spinner.
	SpinnerPulse
	// SpinnerPoints uses points spinner.
	SpinnerPoints
	// SpinnerGlobe uses a globe spinner.
	SpinnerGlobe
	// SpinnerMoon uses a moon spinner.
	SpinnerMoon
	// SpinnerMonkey uses a monkey spinner.
	SpinnerMonkey
)

// Spinner wraps the bubbles spinner with styling.
type Spinner struct {
	spinner spinner.Model
	label   string
}

// NewSpinner creates a new spinner component.
func NewSpinner() Spinner {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.Primary)

	return Spinner{
		spinner: s,
	}
}

// WithStyle sets the spinner animation style.
func (s Spinner) WithStyle(style SpinnerStyle) Spinner {
	s.spinner.Spinner = s.getSpinnerType(style)
	return s
}

// WithLabel sets the spinner label text.
func (s Spinner) WithLabel(label string) Spinner {
	s.label = label
	return s
}

// WithColor sets the spinner color.
func (s Spinner) WithColor(color lipgloss.AdaptiveColor) Spinner {
	s.spinner.Style = lipgloss.NewStyle().Foreground(color)
	return s
}

// Init initializes the spinner.
func (s Spinner) Init() tea.Cmd {
	return s.spinner.Tick
}

// Update handles spinner messages.
func (s Spinner) Update(msg tea.Msg) (Spinner, tea.Cmd) {
	var cmd tea.Cmd
	s.spinner, cmd = s.spinner.Update(msg)
	return s, cmd
}

// View renders the spinner.
func (s Spinner) View() string {
	if s.label != "" {
		return s.spinner.View() + " " + s.label
	}
	return s.spinner.View()
}

// Tick returns the tick command for animation.
func (s Spinner) Tick() tea.Cmd {
	return s.spinner.Tick
}

// getSpinnerType converts SpinnerStyle to bubbles spinner type.
func (s Spinner) getSpinnerType(style SpinnerStyle) spinner.Spinner {
	switch style {
	case SpinnerLine:
		return spinner.Line
	case SpinnerMiniDot:
		return spinner.MiniDot
	case SpinnerJump:
		return spinner.Jump
	case SpinnerPulse:
		return spinner.Pulse
	case SpinnerPoints:
		return spinner.Points
	case SpinnerGlobe:
		return spinner.Globe
	case SpinnerMoon:
		return spinner.Moon
	case SpinnerMonkey:
		return spinner.Monkey
	default:
		return spinner.Dot
	}
}

// LoadingSpinner creates a spinner with "Loading..." label.
func LoadingSpinner() Spinner {
	return NewSpinner().WithLabel("Loading...")
}

// ConnectingSpinner creates a spinner with "Connecting..." label.
func ConnectingSpinner() Spinner {
	return NewSpinner().
		WithStyle(SpinnerDot).
		WithLabel("Connecting...")
}

// WaitingSpinner creates a spinner with "Waiting..." label.
func WaitingSpinner() Spinner {
	return NewSpinner().
		WithStyle(SpinnerPulse).
		WithLabel("Waiting for connection...")
}
