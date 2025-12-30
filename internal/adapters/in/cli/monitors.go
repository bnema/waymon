package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/bnema/waymon/internal/domain"
	"github.com/spf13/cobra"
)

// DisplayInfo represents the display information output.
type DisplayInfo struct {
	Monitors []MonitorInfo `json:"monitors"`
	Error    string        `json:"error,omitempty"`
}

// MonitorInfo represents information about a single monitor.
type MonitorInfo struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	X       int32   `json:"x"`
	Y       int32   `json:"y"`
	Width   int32   `json:"width"`
	Height  int32   `json:"height"`
	Primary bool    `json:"primary"`
	Scale   float64 `json:"scale"`
}

func (c *CLI) newMonitorsCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "monitors",
		Short: "Show monitor configuration",
		Long:  `Display information about connected monitors and their configuration.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.runMonitors(cmd.Context(), jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

func (c *CLI) runMonitors(ctx context.Context, jsonOutput bool) error {
	// Check if display port is available
	if c.displayPort == nil {
		return fmt.Errorf("display port not available - CLI not properly initialized")
	}

	// Get monitor information
	monitors, err := c.displayPort.GetMonitors(ctx)
	if err != nil {
		if jsonOutput {
			info := DisplayInfo{Error: err.Error()}
			return json.NewEncoder(os.Stdout).Encode(info)
		}
		return fmt.Errorf("failed to get monitors: %w", err)
	}

	if jsonOutput {
		return c.outputMonitorsJSON(monitors)
	}

	return c.outputMonitorsText(monitors)
}

func (c *CLI) outputMonitorsJSON(monitors []domain.Monitor) error {
	info := DisplayInfo{
		Monitors: make([]MonitorInfo, len(monitors)),
	}

	for i, mon := range monitors {
		info.Monitors[i] = MonitorInfo{
			ID:      mon.ID,
			Name:    mon.Name,
			X:       mon.X,
			Y:       mon.Y,
			Width:   mon.Width,
			Height:  mon.Height,
			Primary: mon.Primary,
			Scale:   mon.Scale,
		}
	}

	return json.NewEncoder(os.Stdout).Encode(info)
}

func (c *CLI) outputMonitorsText(monitors []domain.Monitor) error {
	if len(monitors) == 0 {
		fmt.Println("No monitors detected")
		return nil
	}

	fmt.Printf("Detected %d monitor(s):\n\n", len(monitors))

	for i, mon := range monitors {
		fmt.Printf("Monitor %d:\n", i+1)
		fmt.Printf("  Name:       %s\n", mon.Name)
		if mon.ID != "" && mon.ID != mon.Name {
			fmt.Printf("  ID:         %s\n", mon.ID)
		}
		fmt.Printf("  Resolution: %dx%d\n", mon.Width, mon.Height)
		fmt.Printf("  Position:   (%d, %d)\n", mon.X, mon.Y)

		if mon.Primary {
			fmt.Printf("  Primary:    Yes\n")
		}

		if mon.Scale != 1.0 {
			fmt.Printf("  Scale:      %.1fx\n", mon.Scale)
		}

		fmt.Println()
	}

	// Show total virtual screen size
	if len(monitors) > 1 {
		minX, minY := monitors[0].X, monitors[0].Y
		maxX, maxY := monitors[0].X+monitors[0].Width, monitors[0].Y+monitors[0].Height

		for _, mon := range monitors[1:] {
			if mon.X < minX {
				minX = mon.X
			}
			if mon.Y < minY {
				minY = mon.Y
			}
			if mon.X+mon.Width > maxX {
				maxX = mon.X + mon.Width
			}
			if mon.Y+mon.Height > maxY {
				maxY = mon.Y + mon.Height
			}
		}

		totalWidth := maxX - minX
		totalHeight := maxY - minY
		fmt.Printf("Total virtual screen: %dx%d\n", totalWidth, totalHeight)
	}

	return nil
}
