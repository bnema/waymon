// Package ui provides consistent styling and components for the Waymon CLI
package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette - consistent across the application
var (
	// Primary colors
	ColorPrimary   = lipgloss.Color("39")  // Bright blue
	ColorSecondary = lipgloss.Color("205") // Pink/magenta
	ColorSuccess   = lipgloss.Color("82")  // Green
	ColorWarning   = lipgloss.Color("214") // Orange
	ColorError     = lipgloss.Color("196") // Red
	ColorInfo      = lipgloss.Color("86")  // Cyan

	// Neutral colors
	ColorText      = lipgloss.Color("252") // Light gray
	ColorSubtle    = lipgloss.Color("241") // Medium gray
	ColorMuted     = lipgloss.Color("238") // Dark gray
	ColorHighlight = lipgloss.Color("255") // White

	// Status colors
	ColorConnected    = ColorSuccess
	ColorDisconnected = ColorError
	ColorActive       = ColorPrimary
	ColorInactive     = ColorSubtle
)

// Base styles - building blocks for other styles
var (
	// Text styles
	TextStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	SubtleStyle = lipgloss.NewStyle().
			Foreground(ColorSubtle)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Emphasis styles
	BoldStyle = lipgloss.NewStyle().
			Bold(true)

	ItalicStyle = lipgloss.NewStyle().
			Italic(true)

	// Header styles
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	SubheaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText)

	// Title styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Background(ColorMuted).
			Padding(0, 1)

	// Status styles
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	InfoStyle = lipgloss.NewStyle().
			Foreground(ColorInfo)

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSubtle).
			Padding(1, 2)

	// List styles
	ListStyle = lipgloss.NewStyle().
			MarginLeft(2)

	ListItemStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	// Spinner style
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)
)

// Component-specific styles
var (
	// Server UI styles
	ServerHeaderStyle = HeaderStyle.Copy().
				Render("Waymon Server")

	ServerStatusStyle = SubtleStyle.Copy()

	// Client UI styles
	ClientHeaderStyle = HeaderStyle.Copy().
				Render("Waymon Client")

	ClientStatusStyle = SubtleStyle.Copy()

	// Connection status styles
	ConnectedIndicator = lipgloss.NewStyle().
				Foreground(ColorConnected).
				Render("●")

	DisconnectedIndicator = lipgloss.NewStyle().
				Foreground(ColorDisconnected).
				Render("○")

	// Control help styles
	ControlsHeaderStyle = SubheaderStyle.Copy().
				MarginTop(1)

	ControlKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	ControlDescStyle = lipgloss.NewStyle().
				Foreground(ColorText)
)

// Helper functions for consistent formatting
func FormatControl(key, desc string) string {
	return ControlKeyStyle.Render(key) + " - " + ControlDescStyle.Render(desc)
}

func FormatStatus(connected bool, status string) string {
	indicator := DisconnectedIndicator
	if connected {
		indicator = ConnectedIndicator
	}
	return indicator + " " + status
}

func FormatListItem(item string, active bool) string {
	style := ListItemStyle
	if active {
		style = style.Copy().Foreground(ColorActive)
	}
	return "  • " + style.Render(item)
}

// Table styles
var (
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(ColorSubtle)

	TableRowStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	TableCellStyle = lipgloss.NewStyle().
			PaddingRight(2)
)

// Spinner presets
var (
	SpinnerDot    = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
	SpinnerLine   = []string{"|", "/", "-", "\\"}
	SpinnerCircle = []string{"◐", "◓", "◑", "◒"}
)

// Layout helpers
func Center(width int, content string) string {
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, content)
}

func Right(width int, content string) string {
	return lipgloss.PlaceHorizontal(width, lipgloss.Right, content)
}
