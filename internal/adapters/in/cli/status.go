package cli

import (
	"fmt"
	"strings"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

func (c *CLI) newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check the status of the Waymon server",
		Long:  `Check the status of the running Waymon server including connected clients and current control state.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.runStatus()
		},
	}
}

func (c *CLI) runStatus() error {
	// Check if IPC client is available
	if c.ipcClient == nil {
		return fmt.Errorf("IPC client not available - CLI not properly initialized")
	}

	// Check if waymon is running
	if !c.ipcClient.IsRunning() {
		fmt.Println("Waymon server is not running")
		return nil
	}

	// Get status
	status, err := c.ipcClient.SendStatus()
	if err != nil {
		return fmt.Errorf("failed to get server status: %w", err)
	}

	// Build the status display
	var output strings.Builder

	// Header
	header := formatAppHeader("SERVER STATUS", fmt.Sprintf("Port %s", status.ServerHost))
	output.WriteString(header)
	output.WriteString("\n\n")

	// Connection status box
	statusContent := strings.Builder{}

	// Active status
	if status.Active {
		statusContent.WriteString(styles.SuccessStyle.Render(styles.IconCircle + " Active"))
	} else {
		statusContent.WriteString(styles.ErrorStyle.Render(styles.IconCircleEmpty + " Inactive"))
	}
	statusContent.WriteString("\n")

	// Current control
	statusContent.WriteString(styles.SubtitleStyle.Render("Current Control: "))
	if status.CurrentIndex == 0 {
		statusContent.WriteString(styles.InfoStyleBold.Render("Local (Server)"))
	} else if int(status.CurrentIndex) <= len(status.ComputerNames) {
		statusContent.WriteString(styles.InfoStyleBold.Render(status.ComputerNames[status.CurrentIndex-1]))
	}

	statusBox := styles.BoxStyle.Render(statusContent.String())
	output.WriteString(statusBox)
	output.WriteString("\n\n")

	// Connected computers section
	if status.TotalCount > 0 {
		output.WriteString(styles.SubtitleStyle.Render(fmt.Sprintf("Connected Computers (%d)", status.TotalCount)))
		output.WriteString("\n\n")

		// Local server entry
		slotStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
		nameStyle := lipgloss.NewStyle().Foreground(styles.Text)
		activeStyle := lipgloss.NewStyle().Foreground(styles.Success).Bold(true)

		output.WriteString("  ")
		output.WriteString(slotStyle.Render("[0]"))
		output.WriteString(" ")
		output.WriteString(nameStyle.Render("Local (Server)"))
		if status.CurrentIndex == 0 {
			output.WriteString(" ")
			output.WriteString(activeStyle.Render(styles.IconChevronL + " ACTIVE"))
		}
		output.WriteString("\n")

		// Client entries
		for i, name := range status.ComputerNames {
			output.WriteString("  ")
			output.WriteString(slotStyle.Render(fmt.Sprintf("[%d]", i+1)))
			output.WriteString(" ")
			output.WriteString(nameStyle.Render(name))
			// Index is bounded by slice length, safe conversion
			if status.CurrentIndex == int32(i+1) { //nolint:gosec // i is bounded by slice length
				output.WriteString(" ")
				output.WriteString(activeStyle.Render(styles.IconChevronL + " ACTIVE"))
			}
			output.WriteString("\n")
		}
	} else {
		noClientsMsg := styles.MutedStyle.Italic(true).Render("No clients connected")
		output.WriteString(noClientsMsg)
		output.WriteString("\n")
	}

	// Help section
	output.WriteString("\n")
	output.WriteString(createSeparator(50, "─"))
	output.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	output.WriteString(helpStyle.Render("Use 'waymon connect <slot>' to switch control"))
	output.WriteString("\n")
	output.WriteString(helpStyle.Render("Use 'waymon release' to return to local control"))

	fmt.Println(output.String())

	return nil
}

// formatAppHeader creates a formatted header for CLI output.
func formatAppHeader(title, subtitle string) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Primary)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(styles.Muted)

	return titleStyle.Render(styles.IconServer+" WAYMON "+title) + "  " + subtitleStyle.Render(subtitle)
}

// createSeparator creates a separator line.
func createSeparator(width int, char string) string {
	return lipgloss.NewStyle().
		Foreground(styles.Border).
		Render(strings.Repeat(char, width))
}
