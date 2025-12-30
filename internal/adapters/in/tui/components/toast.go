// Package components provides reusable TUI components.
package components

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
)

// ToastLevel represents the severity level of a toast.
type ToastLevel int

const (
	// ToastInfo is for informational messages.
	ToastInfo ToastLevel = iota
	// ToastSuccess is for success messages.
	ToastSuccess
	// ToastWarning is for warning messages.
	ToastWarning
	// ToastError is for error messages.
	ToastError
)

// Toast represents a temporary notification message.
type Toast struct {
	message   string
	level     ToastLevel
	duration  time.Duration
	createdAt time.Time
	visible   bool
	width     int
}

// ToastDismissMsg is sent when a toast should be dismissed.
type ToastDismissMsg struct {
	ID int64
}

// NewToast creates a new toast notification.
func NewToast(message string) Toast {
	return Toast{
		message:   message,
		level:     ToastInfo,
		duration:  3 * time.Second,
		createdAt: time.Now(),
		visible:   true,
		width:     40,
	}
}

// WithMessage sets the toast message.
func (t Toast) WithMessage(message string) Toast {
	t.message = message
	return t
}

// WithLevel sets the toast level.
func (t Toast) WithLevel(level ToastLevel) Toast {
	t.level = level
	return t
}

// WithDuration sets how long the toast is visible.
func (t Toast) WithDuration(d time.Duration) Toast {
	t.duration = d
	return t
}

// WithWidth sets the toast width.
func (t Toast) WithWidth(width int) Toast {
	t.width = width
	return t
}

// Show makes the toast visible.
func (t Toast) Show() Toast {
	t.visible = true
	t.createdAt = time.Now()
	return t
}

// Hide makes the toast invisible.
func (t Toast) Hide() Toast {
	t.visible = false
	return t
}

// IsVisible returns whether the toast is currently visible.
func (t Toast) IsVisible() bool {
	return t.visible
}

// IsExpired returns whether the toast duration has elapsed.
func (t Toast) IsExpired() bool {
	return time.Since(t.createdAt) > t.duration
}

// TimeRemaining returns how much time is left before expiration.
func (t Toast) TimeRemaining() time.Duration {
	remaining := t.duration - time.Since(t.createdAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Init returns a command to dismiss the toast after duration.
func (t Toast) Init() tea.Cmd {
	if !t.visible {
		return nil
	}
	id := t.createdAt.UnixNano()
	return tea.Tick(t.duration, func(time.Time) tea.Msg {
		return ToastDismissMsg{ID: id}
	})
}

// View renders the toast notification.
func (t Toast) View() string {
	if !t.visible {
		return ""
	}

	icon := t.icon()
	style := t.style()

	// Build content
	content := icon + " " + t.message

	// Apply style with width
	return style.
		Width(t.width).
		Render(content)
}

// icon returns the appropriate icon for the toast level.
func (t Toast) icon() string {
	switch t.level {
	case ToastSuccess:
		return styles.IconCheck
	case ToastWarning:
		return styles.IconWarning
	case ToastError:
		return styles.IconCross
	default:
		return styles.IconInfo
	}
}

// style returns the appropriate style for the toast level.
func (t Toast) style() lipgloss.Style {
	base := lipgloss.NewStyle().
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder())

	switch t.level {
	case ToastSuccess:
		return base.
			BorderForeground(styles.Success).
			Foreground(styles.Success)
	case ToastWarning:
		return base.
			BorderForeground(styles.Warning).
			Foreground(styles.Warning)
	case ToastError:
		return base.
			BorderForeground(styles.Error).
			Foreground(styles.Error)
	default:
		return base.
			BorderForeground(styles.Info).
			Foreground(styles.Info)
	}
}

// ToastManager manages multiple toasts with stacking.
type ToastManager struct {
	toasts  []Toast
	maxSize int
	width   int
}

// NewToastManager creates a new toast manager.
func NewToastManager() ToastManager {
	return ToastManager{
		toasts:  make([]Toast, 0),
		maxSize: 5,
		width:   40,
	}
}

// WithMaxSize sets the maximum number of visible toasts.
func (m ToastManager) WithMaxSize(size int) ToastManager {
	m.maxSize = size
	return m
}

// WithWidth sets the width for all toasts.
func (m ToastManager) WithWidth(width int) ToastManager {
	m.width = width
	return m
}

// Add adds a new toast to the manager.
func (m ToastManager) Add(toast Toast) ToastManager {
	toast = toast.WithWidth(m.width).Show()
	m.toasts = append(m.toasts, toast)

	// Trim oldest toasts if over max
	if len(m.toasts) > m.maxSize {
		m.toasts = m.toasts[len(m.toasts)-m.maxSize:]
	}

	return m
}

// AddMessage adds a simple info message.
func (m ToastManager) AddMessage(message string) ToastManager {
	return m.Add(NewToast(message))
}

// AddSuccess adds a success toast.
func (m ToastManager) AddSuccess(message string) ToastManager {
	return m.Add(NewToast(message).WithLevel(ToastSuccess))
}

// AddWarning adds a warning toast.
func (m ToastManager) AddWarning(message string) ToastManager {
	return m.Add(NewToast(message).WithLevel(ToastWarning))
}

// AddError adds an error toast.
func (m ToastManager) AddError(message string) ToastManager {
	return m.Add(NewToast(message).WithLevel(ToastError))
}

// Update handles toast dismiss messages.
func (m ToastManager) Update(msg tea.Msg) ToastManager {
	if dismiss, ok := msg.(ToastDismissMsg); ok {
		newToasts := make([]Toast, 0, len(m.toasts))
		for _, t := range m.toasts {
			if t.createdAt.UnixNano() != dismiss.ID {
				newToasts = append(newToasts, t)
			}
		}
		m.toasts = newToasts
	}
	return m
}

// CleanExpired removes expired toasts.
func (m ToastManager) CleanExpired() ToastManager {
	newToasts := make([]Toast, 0, len(m.toasts))
	for _, t := range m.toasts {
		if !t.IsExpired() {
			newToasts = append(newToasts, t)
		}
	}
	m.toasts = newToasts
	return m
}

// Init returns commands to set up auto-dismiss for all toasts.
func (m ToastManager) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, t := range m.toasts {
		if cmd := t.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// View renders all visible toasts stacked vertically.
func (m ToastManager) View() string {
	if len(m.toasts) == 0 {
		return ""
	}

	var views []string
	for _, t := range m.toasts {
		if t.IsVisible() {
			views = append(views, t.View())
		}
	}

	return lipgloss.JoinVertical(lipgloss.Right, views...)
}

// HasToasts returns whether there are any visible toasts.
func (m ToastManager) HasToasts() bool {
	return len(m.toasts) > 0
}

// Count returns the number of visible toasts.
func (m ToastManager) Count() int {
	return len(m.toasts)
}

// Convenience constructors for common toast types.

// InfoToast creates an info toast.
func InfoToast(message string) Toast {
	return NewToast(message).WithLevel(ToastInfo)
}

// SuccessToast creates a success toast.
func SuccessToast(message string) Toast {
	return NewToast(message).WithLevel(ToastSuccess)
}

// WarningToast creates a warning toast.
func WarningToast(message string) Toast {
	return NewToast(message).WithLevel(ToastWarning)
}

// ErrorToast creates an error toast.
func ErrorToast(message string) Toast {
	return NewToast(message).WithLevel(ToastError)
}
