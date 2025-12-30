// Package styles provides global styling for the TUI.
package styles

import "github.com/charmbracelet/lipgloss"

// Adaptive colors for light/dark terminals.
// These automatically adjust based on terminal background.
var (
	// Brand colors
	Primary   = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7B78FF"}
	Secondary = lipgloss.AdaptiveColor{Light: "#6E6E6E", Dark: "#A0A0A0"}

	// Semantic colors
	Success = lipgloss.AdaptiveColor{Light: "#00A651", Dark: "#4ADE80"}
	Warning = lipgloss.AdaptiveColor{Light: "#F5A623", Dark: "#FBBF24"}
	Error   = lipgloss.AdaptiveColor{Light: "#D0021B", Dark: "#EF4444"}
	Info    = lipgloss.AdaptiveColor{Light: "#2196F3", Dark: "#60A5FA"}

	// Neutral colors
	Muted      = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"}
	Border     = lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"}
	Surface    = lipgloss.AdaptiveColor{Light: "#F9FAFB", Dark: "#1F2937"}
	Background = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#111827"}
	Text       = lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#F9FAFB"}
	TextMuted  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}

	// Accent colors for different states
	Active   = lipgloss.AdaptiveColor{Light: "#3B82F6", Dark: "#60A5FA"}
	Inactive = lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#4B5563"}

	// Client status colors
	Connected    = Success
	Disconnected = Error
	Connecting   = Warning
	Controlled   = lipgloss.AdaptiveColor{Light: "#8B5CF6", Dark: "#A78BFA"}
)

// Color is a convenience type for lipgloss.AdaptiveColor.
type Color = lipgloss.AdaptiveColor
