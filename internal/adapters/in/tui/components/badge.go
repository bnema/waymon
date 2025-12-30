// Package components provides reusable TUI components.
package components

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
)

// BadgeVariant defines the visual style of a badge.
type BadgeVariant int

const (
	// BadgeDefault is the neutral/default badge style.
	BadgeDefault BadgeVariant = iota
	// BadgeSuccess indicates a positive/success state.
	BadgeSuccess
	// BadgeWarning indicates a warning/caution state.
	BadgeWarning
	// BadgeError indicates an error/failure state.
	BadgeError
	// BadgeInfo indicates an informational state.
	BadgeInfo
	// BadgePrimary uses the primary brand color.
	BadgePrimary
)

// Badge is a styled label for displaying status or categories.
type Badge struct {
	text    string
	variant BadgeVariant
	icon    string
}

// NewBadge creates a new badge with the given text.
func NewBadge(text string) Badge {
	return Badge{
		text:    text,
		variant: BadgeDefault,
	}
}

// WithVariant sets the badge variant.
func (b Badge) WithVariant(v BadgeVariant) Badge {
	b.variant = v
	return b
}

// WithIcon adds an icon prefix to the badge.
func (b Badge) WithIcon(icon string) Badge {
	b.icon = icon
	return b
}

// View renders the badge.
func (b Badge) View() string {
	style := b.getStyle()
	content := b.text
	if b.icon != "" {
		content = b.icon + " " + content
	}
	return style.Render(content)
}

// getStyle returns the lipgloss style for this badge variant.
func (b Badge) getStyle() lipgloss.Style {
	switch b.variant {
	case BadgeSuccess:
		return styles.BadgeSuccessStyle
	case BadgeWarning:
		return styles.BadgeWarningStyle
	case BadgeError:
		return styles.BadgeErrorStyle
	case BadgeInfo:
		return styles.BadgeInfoStyle
	case BadgePrimary:
		return styles.BadgeStyle.
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(styles.Primary)
	default:
		return styles.BadgeMutedStyle
	}
}

// Convenience constructors for common badge types.

// SuccessBadge creates a success-styled badge.
func SuccessBadge(text string) Badge {
	return NewBadge(text).WithVariant(BadgeSuccess)
}

// ErrorBadge creates an error-styled badge.
func ErrorBadge(text string) Badge {
	return NewBadge(text).WithVariant(BadgeError)
}

// WarningBadge creates a warning-styled badge.
func WarningBadge(text string) Badge {
	return NewBadge(text).WithVariant(BadgeWarning)
}

// InfoBadge creates an info-styled badge.
func InfoBadge(text string) Badge {
	return NewBadge(text).WithVariant(BadgeInfo)
}

// ConnectedBadge creates a badge indicating connected status.
func ConnectedBadge() Badge {
	return NewBadge("Connected").
		WithVariant(BadgeSuccess).
		WithIcon(styles.IconConnected)
}

// DisconnectedBadge creates a badge indicating disconnected status.
func DisconnectedBadge() Badge {
	return NewBadge("Disconnected").
		WithVariant(BadgeError).
		WithIcon(styles.IconDisconnected)
}

// ControlledBadge creates a badge indicating being controlled.
func ControlledBadge() Badge {
	return NewBadge("Controlled").
		WithVariant(BadgePrimary).
		WithIcon(styles.IconMouse)
}

// IdleBadge creates a badge indicating idle status.
func IdleBadge() Badge {
	return NewBadge("Idle").
		WithVariant(BadgeDefault).
		WithIcon(styles.IconCircleEmpty)
}
