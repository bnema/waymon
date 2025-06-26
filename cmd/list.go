package cmd

import (
	"fmt"
	"strings"

	"github.com/bnema/waymon/internal/ipc"
	"github.com/bnema/waymon/internal/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List connected clients",
	Long:  `List all connected clients and their slot numbers for use with the connect command.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if waymon is running
		if !ipc.IsWaymonRunning() {
			fmt.Println("Waymon server is not running")
			return nil
		}

		// Create IPC client
		client, err := ipc.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create IPC client: %w", err)
		}
		defer client.Close()

		// Get status
		status, err := client.GetStatus()
		if err != nil {
			return fmt.Errorf("failed to get server status: %w", err)
		}

		// Build the list display
		var output strings.Builder

		// Header
		header := ui.FormatAppHeader("CLIENT LIST", fmt.Sprintf("Server: %s", status.ServerHost))
		output.WriteString(header)
		output.WriteString("\n\n")

		// Create table
		rows := [][]string{}
		
		// Add local server row
		activeMarker := ""
		if status.CurrentComputer == 0 {
			activeMarker = "◀"
		}
		rows = append(rows, []string{"0", "Local (Server)", "-", activeMarker})

		// Add client rows
		for i, name := range status.ComputerNames {
			activeMarker = ""
			if status.CurrentComputer == int32(i+1) {
				activeMarker = "◀"
			}
			// For now, we don't have client addresses in the status response
			// so we'll just show the name
			rows = append(rows, []string{fmt.Sprintf("%d", i+1), name, "-", activeMarker})
		}

		// Style the table
		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(ui.ColorSubtle)).
			StyleFunc(func(row, col int) lipgloss.Style {
				switch {
				case row == 0: // Header row
					return lipgloss.NewStyle().
						Foreground(ui.ColorPrimary).
						Bold(true).
						Padding(0, 1)
				case col == 0: // Slot column
					return lipgloss.NewStyle().
						Foreground(ui.ColorInfo).
						Bold(true).
						Padding(0, 1)
				case col == 3 && rows[row-1][3] != "": // Active marker
					return lipgloss.NewStyle().
						Foreground(ui.ColorSuccess).
						Bold(true).
						Padding(0, 1)
				default:
					return lipgloss.NewStyle().
						Foreground(ui.ColorText).
						Padding(0, 1)
				}
			}).
			Headers("SLOT", "NAME", "ADDRESS", "STATUS").
			Rows(rows...)

		output.WriteString(t.String())

		// Show total count
		output.WriteString("\n\n")
		countStyle := lipgloss.NewStyle().Foreground(ui.ColorSubtle)
		if len(status.ComputerNames) == 0 {
			output.WriteString(countStyle.Render("No clients connected"))
		} else {
			output.WriteString(countStyle.Render(fmt.Sprintf("Total: %d client(s) connected", len(status.ComputerNames))))
		}

		// Help section
		output.WriteString("\n\n")
		helpBox := ui.BoxStyle.
			BorderStyle(lipgloss.HiddenBorder()).
			PaddingLeft(0).
			Render(strings.Join([]string{
				ui.InfoStyle.Render("Commands:"),
				"  " + ui.ControlKeyStyle.Render("waymon connect <slot>") + " - Switch to a specific computer",
				"  " + ui.ControlKeyStyle.Render("waymon release") + " - Return control to local",
				"  " + ui.ControlKeyStyle.Render("waymon switch next") + " - Cycle to next computer",
			}, "\n"))
		output.WriteString(helpBox)

		fmt.Println(output.String())

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}