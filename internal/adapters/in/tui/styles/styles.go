// Package styles provides global styling for the TUI.
package styles

import "github.com/charmbracelet/lipgloss"

// Base styles - foundational styles to build upon.
var (
	// BaseStyle is the default style with minimal padding.
	BaseStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// NoPadding removes all padding.
	NoPadding = lipgloss.NewStyle()
)

// Container styles - for grouping content.
var (
	// BoxStyle creates a rounded bordered box.
	BoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Border).
			Padding(1, 2)

	// BoxStyleThick creates a thick bordered box.
	BoxStyleThick = lipgloss.NewStyle().
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(Border).
			Padding(1, 2)

	// BoxStyleDouble creates a double-line bordered box.
	BoxStyleDouble = lipgloss.NewStyle().
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(Border).
			Padding(1, 2)

	// PanelStyle is for content panels with surface background.
	PanelStyle = lipgloss.NewStyle().
			Background(Surface).
			Padding(1, 2)

	// CardStyle combines border and background.
	CardStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Border).
			Background(Surface).
			Padding(1, 2)
)

// Text styles - for typography.
var (
	// TitleStyle is for main headings.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			MarginBottom(1)

	// SubtitleStyle is for secondary headings.
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(Secondary)

	// LabelStyle is for form labels and small headings.
	LabelStyle = lipgloss.NewStyle().
			Foreground(TextMuted).
			Bold(true)

	// ValueStyle is for displaying values.
	ValueStyle = lipgloss.NewStyle().
			Foreground(Text)

	// MutedStyle is for de-emphasized text.
	MutedStyle = lipgloss.NewStyle().
			Foreground(Muted)

	// CodeStyle is for inline code or technical values.
	CodeStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Background(Surface).
			Padding(0, 1)
)

// Status styles - for semantic messaging.
var (
	// SuccessStyle indicates positive/success state.
	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success)

	// SuccessStyleBold is bold success text.
	SuccessStyleBold = SuccessStyle.Bold(true)

	// ErrorStyle indicates error/failure state.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error)

	// ErrorStyleBold is bold error text.
	ErrorStyleBold = ErrorStyle.Bold(true)

	// WarningStyle indicates warning/caution state.
	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning)

	// WarningStyleBold is bold warning text.
	WarningStyleBold = WarningStyle.Bold(true)

	// InfoStyle indicates informational state.
	InfoStyle = lipgloss.NewStyle().
			Foreground(Info)

	// InfoStyleBold is bold info text.
	InfoStyleBold = InfoStyle.Bold(true)
)

// Interactive styles - for selectable/focusable elements.
var (
	// ActiveStyle is for the currently active/focused item.
	ActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			Background(Surface)

	// SelectedStyle is for selected but not focused items.
	SelectedStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true)

	// HoverStyle is for items being hovered (if mouse support).
	HoverStyle = lipgloss.NewStyle().
			Foreground(Active).
			Underline(true)

	// DisabledStyle is for disabled/inactive items.
	DisabledStyle = lipgloss.NewStyle().
			Foreground(Inactive)

	// FocusedBorderStyle is a border style for focused elements.
	FocusedBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(Primary)

	// UnfocusedBorderStyle is a border style for unfocused elements.
	UnfocusedBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(Border)
)

// List styles - for list items.
var (
	// ListItemStyle is the base style for list items.
	ListItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	// ListItemSelectedStyle is for selected list items.
	ListItemSelectedStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(Primary).
				Bold(true)

	// ListItemActiveStyle is for the active list item.
	ListItemActiveStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(Text).
				Background(Surface).
				Bold(true)

	// ListBullet is the bullet character for list items.
	ListBullet = IconBullet + " "

	// ListBulletSelected is the bullet for selected items.
	ListBulletSelected = IconChevronR + " "
)

// Badge styles - for status badges and tags.
var (
	// BadgeStyle is the base badge style.
	BadgeStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// BadgeSuccessStyle is a success badge.
	BadgeSuccessStyle = BadgeStyle.
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(Success)

	// BadgeErrorStyle is an error badge.
	BadgeErrorStyle = BadgeStyle.
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(Error)

	// BadgeWarningStyle is a warning badge.
	BadgeWarningStyle = BadgeStyle.
				Foreground(lipgloss.Color("#000000")).
				Background(Warning)

	// BadgeInfoStyle is an info badge.
	BadgeInfoStyle = BadgeStyle.
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(Info)

	// BadgeMutedStyle is a muted/neutral badge.
	BadgeMutedStyle = BadgeStyle.
			Foreground(Text).
			Background(Muted)
)

// Layout styles - for positioning and spacing.
var (
	// CenteredStyle centers content horizontally.
	CenteredStyle = lipgloss.NewStyle().
			Align(lipgloss.Center)

	// RightAlignedStyle aligns content to the right.
	RightAlignedStyle = lipgloss.NewStyle().
				Align(lipgloss.Right)

	// FullWidthStyle expands to full width.
	FullWidthStyle = lipgloss.NewStyle().
			Width(100) // Will be overridden with actual terminal width

	// SpacerStyle adds vertical spacing.
	SpacerStyle = lipgloss.NewStyle().
			MarginTop(1).
			MarginBottom(1)
)

// Header/Footer styles.
var (
	// HeaderStyle is for the app header.
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(Border).
			Padding(0, 1).
			MarginBottom(1)

	// FooterStyle is for the app footer/status bar.
	FooterStyle = lipgloss.NewStyle().
			Foreground(TextMuted).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(Border).
			Padding(0, 1).
			MarginTop(1)

	// StatusBarStyle is an alias for FooterStyle.
	StatusBarStyle = FooterStyle
)

// Help styles - for keybinding help.
var (
	// HelpKeyStyle is for keyboard shortcut keys.
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true)

	// HelpDescStyle is for help descriptions.
	HelpDescStyle = lipgloss.NewStyle().
			Foreground(TextMuted)

	// HelpSepStyle is for separators in help.
	HelpSepStyle = lipgloss.NewStyle().
			Foreground(Muted)
)

// Utility functions.

// Width returns a new style with the specified width.
func Width(s lipgloss.Style, w int) lipgloss.Style {
	return s.Width(w)
}

// Height returns a new style with the specified height.
func Height(s lipgloss.Style, h int) lipgloss.Style {
	return s.Height(h)
}

// Margin returns a new style with the specified margins.
func Margin(s lipgloss.Style, top, right, bottom, left int) lipgloss.Style {
	return s.Margin(top, right, bottom, left)
}

// Padding returns a new style with the specified padding.
func Padding(s lipgloss.Style, top, right, bottom, left int) lipgloss.Style {
	return s.Padding(top, right, bottom, left)
}

// JoinHorizontal joins strings horizontally with the given position.
func JoinHorizontal(pos lipgloss.Position, strs ...string) string {
	return lipgloss.JoinHorizontal(pos, strs...)
}

// JoinVertical joins strings vertically with the given position.
func JoinVertical(pos lipgloss.Position, strs ...string) string {
	return lipgloss.JoinVertical(pos, strs...)
}

// Place places content at the specified position within a given size.
func Place(width, height int, hPos, vPos lipgloss.Position, str string) string {
	return lipgloss.Place(width, height, hPos, vPos, str)
}
