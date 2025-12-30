// Package components provides reusable TUI components.
package components

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
	"github.com/bnema/waymon/internal/domain"
)

// ClientList displays a list of connected clients.
type ClientList struct {
	clients       []domain.Client
	selectedIndex int
	activeID      string
	width         int
	showDetails   bool
}

// NewClientList creates a new client list component.
func NewClientList() ClientList {
	return ClientList{
		clients:       make([]domain.Client, 0),
		selectedIndex: 0,
		width:         60,
		showDetails:   true,
	}
}

// WithClients sets the list of clients.
func (c ClientList) WithClients(clients []domain.Client) ClientList {
	c.clients = clients
	return c
}

// WithSelectedIndex sets the selected client index.
func (c ClientList) WithSelectedIndex(index int) ClientList {
	c.selectedIndex = index
	return c
}

// WithActiveID sets the active client ID (being controlled).
func (c ClientList) WithActiveID(id string) ClientList {
	c.activeID = id
	return c
}

// WithWidth sets the component width.
func (c ClientList) WithWidth(width int) ClientList {
	c.width = width
	return c
}

// WithShowDetails sets whether to show client details.
func (c ClientList) WithShowDetails(show bool) ClientList {
	c.showDetails = show
	return c
}

// SelectNext moves selection to the next client.
func (c ClientList) SelectNext() ClientList {
	if len(c.clients) > 0 {
		c.selectedIndex = (c.selectedIndex + 1) % len(c.clients)
	}
	return c
}

// SelectPrevious moves selection to the previous client.
func (c ClientList) SelectPrevious() ClientList {
	if len(c.clients) > 0 {
		c.selectedIndex = (c.selectedIndex - 1 + len(c.clients)) % len(c.clients)
	}
	return c
}

// SelectedClient returns the currently selected client.
func (c ClientList) SelectedClient() *domain.Client {
	if c.selectedIndex >= 0 && c.selectedIndex < len(c.clients) {
		return &c.clients[c.selectedIndex]
	}
	return nil
}

// View renders the client list.
func (c ClientList) View() string {
	if len(c.clients) == 0 {
		return c.renderEmpty()
	}

	var lines []string

	// Title
	title := styles.TitleStyle.Render(
		fmt.Sprintf("%s Connected Clients (%d)", styles.IconUsers, len(c.clients)),
	)
	lines = append(lines, title)
	lines = append(lines, "")

	// Client items
	for i, client := range c.clients {
		item := c.renderClient(i, client)
		lines = append(lines, item)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderEmpty renders the empty state.
func (c ClientList) renderEmpty() string {
	emptyStyle := styles.MutedStyle.
		Width(c.width).
		Align(lipgloss.Center).
		Padding(2, 0)

	return emptyStyle.Render(
		styles.IconInfo + " No clients connected\n\n" +
			"Waiting for clients to connect...",
	)
}

// renderClient renders a single client item.
func (c ClientList) renderClient(index int, client domain.Client) string {
	isSelected := index == c.selectedIndex
	isActive := client.ID == c.activeID

	// Build status indicator
	var statusIcon string
	var statusStyle lipgloss.Style
	switch client.Status {
	case domain.ClientBeingControlled:
		statusIcon = styles.IconMouse
		statusStyle = styles.SuccessStyleBold
	case domain.ClientDisconnected:
		statusIcon = styles.IconDisconnected
		statusStyle = styles.ErrorStyle
	default:
		statusIcon = styles.IconCircle
		statusStyle = styles.MutedStyle
	}

	// Build the item content
	var itemStyle lipgloss.Style
	bullet := "  "
	if isSelected {
		itemStyle = styles.ListItemSelectedStyle
		bullet = styles.IconChevronR + " "
	} else {
		itemStyle = styles.ListItemStyle
	}

	// Client name and address
	nameText := client.Name
	if nameText == "" {
		nameText = client.ID
	}

	addressText := styles.MutedStyle.Render(client.Address)

	// Status badge
	var statusBadge string
	if isActive {
		statusBadge = ControlledBadge().View()
	} else {
		statusBadge = statusStyle.Render(statusIcon)
	}

	// First line: bullet + name + status
	firstLine := bullet + itemStyle.Render(nameText) + " " + statusBadge

	// Build result
	var result string
	if c.showDetails {
		// Details: address and connection time
		details := "   " + addressText
		if !client.ConnectedAt.IsZero() {
			duration := formatDuration(time.Since(client.ConnectedAt))
			details += " • " + styles.MutedStyle.Render("Connected "+duration)
		}

		// Monitor count if available
		if len(client.Monitors) > 0 {
			monitorInfo := fmt.Sprintf(" • %s %d monitor(s)",
				styles.IconMonitor,
				len(client.Monitors),
			)
			details += styles.MutedStyle.Render(monitorInfo)
		}

		result = lipgloss.JoinVertical(lipgloss.Left, firstLine, details)
	} else {
		result = firstLine
	}

	return result
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}
