package cmd

import (
	"fmt"
	"strings"

	"github.com/bnema/waymon/internal/ipc"
	"github.com/bnema/waymon/internal/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of the Waymon server",
	Long:  `Check the status of the running Waymon server including connected clients and current control state.`,
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

		// Build the status display
		var output strings.Builder

		// Header
		header := ui.FormatAppHeader("SERVER STATUS", fmt.Sprintf("Port %s", status.ServerHost))
		output.WriteString(header)
		output.WriteString("\n\n")

		// Connection status box
		statusContent := strings.Builder{}
		
		// Active status
		if status.Active {
			statusContent.WriteString(ui.SuccessStyle.Render("● Active"))
		} else {
			statusContent.WriteString(ui.ErrorStyle.Render("○ Inactive"))
		}
		statusContent.WriteString("\n")

		// Current control
		statusContent.WriteString(ui.SubheaderStyle.Render("Current Control: "))
		if status.CurrentComputer == 0 {
			statusContent.WriteString(ui.InfoStyle.Bold(true).Render("Local (Server)"))
		} else if status.CurrentComputer <= int32(len(status.ComputerNames)) {
			statusContent.WriteString(ui.InfoStyle.Bold(true).Render(status.ComputerNames[status.CurrentComputer-1]))
		}

		statusBox := ui.BoxStyle.Render(statusContent.String())
		output.WriteString(statusBox)
		output.WriteString("\n\n")

		// Connected computers section
		if status.TotalComputers > 0 {
			output.WriteString(ui.SubheaderStyle.Render(fmt.Sprintf("Connected Computers (%d)", status.TotalComputers)))
			output.WriteString("\n\n")

			// Local server entry
			slotStyle := lipgloss.NewStyle().Foreground(ui.ColorPrimary).Bold(true)
			nameStyle := lipgloss.NewStyle().Foreground(ui.ColorText)
			activeStyle := lipgloss.NewStyle().Foreground(ui.ColorSuccess).Bold(true)

			output.WriteString("  ")
			output.WriteString(slotStyle.Render("[0]"))
			output.WriteString(" ")
			output.WriteString(nameStyle.Render("Local (Server)"))
			if status.CurrentComputer == 0 {
				output.WriteString(" ")
				output.WriteString(activeStyle.Render("◀ ACTIVE"))
			}
			output.WriteString("\n")

			// Client entries
			for i, name := range status.ComputerNames {
				output.WriteString("  ")
				output.WriteString(slotStyle.Render(fmt.Sprintf("[%d]", i+1)))
				output.WriteString(" ")
				output.WriteString(nameStyle.Render(name))
				if status.CurrentComputer == int32(i+1) {
					output.WriteString(" ")
					output.WriteString(activeStyle.Render("◀ ACTIVE"))
				}
				output.WriteString("\n")
			}
		} else {
			noClientsMsg := ui.MutedStyle.Italic(true).Render("No clients connected")
			output.WriteString(noClientsMsg)
			output.WriteString("\n")
		}

		// Help section
		output.WriteString("\n")
		output.WriteString(ui.CreateSeparator(50, "─"))
		output.WriteString("\n")
		helpStyle := lipgloss.NewStyle().Foreground(ui.ColorSubtle)
		output.WriteString(helpStyle.Render("Use 'waymon connect <slot>' to switch control"))
		output.WriteString("\n")
		output.WriteString(helpStyle.Render("Use 'waymon release' to return to local control"))

		fmt.Println(output.String())

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}