package cli

import (
	"fmt"
	"strings"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
)

func (c *CLI) newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List connected clients",
		Long:  `List all connected clients and their slot numbers for use with the connect command.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.runList()
		},
	}
}

func (c *CLI) runList() error {
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

	// Build the list display
	var output strings.Builder

	// Header
	header := formatAppHeader("CLIENT LIST", fmt.Sprintf("Server: %s", status.ServerHost))
	output.WriteString(header)
	output.WriteString("\n\n")

	// Create table
	rows := [][]string{}

	// Add local server row
	activeMarker := ""
	if status.CurrentIndex == 0 {
		activeMarker = styles.IconChevronL
	}
	rows = append(rows, []string{"0", "Local (Server)", "-", activeMarker})

	// Add client rows
	for i, name := range status.ComputerNames {
		activeMarker = ""
		// Client index is bounded by slice length which is << MaxInt32
		if status.CurrentIndex == int32(i+1) { //nolint:gosec // G115: i is bounded by slice length
			activeMarker = styles.IconChevronL
		}
		rows = append(rows, []string{fmt.Sprintf("%d", i+1), name, "-", activeMarker})
	}

	// Style the table
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(styles.Muted)).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == 0: // Header row
				return lipgloss.NewStyle().
					Foreground(styles.Primary).
					Bold(true).
					Padding(0, 1)
			case col == 0: // Slot column
				return lipgloss.NewStyle().
					Foreground(styles.Info).
					Bold(true).
					Padding(0, 1)
			case col == 3 && len(rows) > row-1 && rows[row-1][3] != "": // Active marker
				return lipgloss.NewStyle().
					Foreground(styles.Success).
					Bold(true).
					Padding(0, 1)
			default:
				return lipgloss.NewStyle().
					Foreground(styles.Text).
					Padding(0, 1)
			}
		}).
		Headers("SLOT", "NAME", "ADDRESS", "STATUS").
		Rows(rows...)

	output.WriteString(t.String())

	// Show total count
	output.WriteString("\n\n")
	countStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	if len(status.ComputerNames) == 0 {
		output.WriteString(countStyle.Render("No clients connected"))
	} else {
		output.WriteString(countStyle.Render(fmt.Sprintf("Total: %d client(s) connected", len(status.ComputerNames))))
	}

	// Help section
	output.WriteString("\n\n")
	helpBox := styles.BoxStyle.
		BorderStyle(lipgloss.HiddenBorder()).
		PaddingLeft(0).
		Render(strings.Join([]string{
			styles.InfoStyle.Render("Commands:"),
			"  " + styles.HelpKeyStyle.Render("waymon connect <slot>") + " - Switch to a specific computer",
			"  " + styles.HelpKeyStyle.Render("waymon release") + " - Return control to local",
			"  " + styles.HelpKeyStyle.Render("waymon switch") + " - Cycle to next computer",
		}, "\n"))
	output.WriteString(helpBox)

	fmt.Println(output.String())

	return nil
}
