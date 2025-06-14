// Package ui provides consistent styling and components for the Waymon CLI
package ui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
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

// Icons and indicators for consistent app-wide usage (using simple ASCII/Unicode symbols)
var (
	// Status icons
	IconSuccess    = "✓"
	IconError      = "✗"
	IconWarning    = "!"
	IconInfo       = "i"
	IconProcessing = "~"
	IconCheck      = "✓"
	IconCross      = "✗"

	// Application icons
	IconSetup   = "»"
	IconServer  = "S"
	IconClient  = "C"
	IconConfig  = "*"
	IconNetwork = "#"
	IconSummary = "="
	IconSteps   = "→"
	IconPhase   = "·"

	// Progress icons
	IconProgress = "..."
	IconDone     = "✓"
	IconPending  = "·"
)

// Setup-specific styles
var (
	// Setup header styles
	SetupHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

	SetupPhaseStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorInfo)

	// Setup result styles
	SetupSuccessStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess)

	SetupErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	SetupWarningStyle = lipgloss.NewStyle().
				Foreground(ColorWarning)

	// Setup summary styles
	SummaryHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

	SummarySuccessStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	SummaryWarningStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)

	SummaryErrorStyle = lipgloss.NewStyle().
				Foreground(ColorError).
				Bold(true)

	// Action item styles
	ActionItemStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			MarginLeft(1)
)

// Setup formatting functions
func FormatSetupHeader(title string) string {
	coloredIcon := InfoStyle.Render(IconSetup)
	header := SetupHeaderStyle.Render(coloredIcon + " " + title)
	return header + "\n" + CreateSeparator(50, "─")
}

func FormatSetupPhase(phase string) string {
	coloredIcon := InfoStyle.Render(IconPhase)
	return SetupPhaseStyle.Render(coloredIcon + " " + phase)
}

func FormatSetupResult(success bool, step, message string) string {
	var coloredIcon string
	var style lipgloss.Style

	if success {
		coloredIcon = SuccessStyle.Render(IconSuccess)
		style = SetupSuccessStyle
	} else {
		coloredIcon = ErrorStyle.Render(IconError)
		style = SetupErrorStyle
	}

	result := "   " + coloredIcon + " " + step
	if message != "" {
		result += " - " + style.Render(message)
	}
	return result
}

func FormatSummaryHeader(title string) string {
	coloredIcon := InfoStyle.Render(IconSummary)
	header := SummaryHeaderStyle.Render(coloredIcon + " " + title)
	return header + "\n" + CreateSeparator(50, "─")
}

func FormatSummaryStatus(allSuccess, needsRelogin bool) string {
	if allSuccess && !needsRelogin {
		coloredIcon := SuccessStyle.Render(IconDone)
		return SummarySuccessStyle.Render(coloredIcon + " Setup completed successfully!")
	} else if needsRelogin {
		coloredIcon := WarningStyle.Render(IconWarning)
		return SummaryWarningStyle.Render(coloredIcon + " Setup completed, but requires relogin")
	} else {
		coloredIcon := ErrorStyle.Render(IconError)
		return SummaryErrorStyle.Render(coloredIcon + " Setup completed with some issues")
	}
}

func FormatActionItem(index int, action string) string {
	return ActionItemStyle.Render(fmt.Sprintf("   %d. %s", index, action))
}

func FormatNextStepsHeader() string {
	return SetupPhaseStyle.Render(IconSteps + " Next Steps:")
}

// Layout helpers
func Center(width int, content string) string {
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, content)
}

func Right(width int, content string) string {
	return lipgloss.PlaceHorizontal(width, lipgloss.Right, content)
}

// CreateSeparator creates a horizontal line separator
func CreateSeparator(width int, char string) string {
	if width <= 0 {
		width = 50 // Default width
	}

	// For all separator types, just repeat the character
	if char == "" {
		char = "─" // Default to horizontal line
	}

	return lipgloss.NewStyle().
		Foreground(ColorSubtle).
		Render(strings.Repeat(char, width))
}
