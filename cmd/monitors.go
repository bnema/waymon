package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bnema/waymon/internal/display"
	"github.com/spf13/cobra"
)

// DisplayInfo represents the display information output
type DisplayInfo struct {
	Monitors []MonitorInfo `json:"monitors"`
	Error    string        `json:"error,omitempty"`
}

// MonitorInfo represents information about a single monitor
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

var (
	jsonOutput bool
)

var monitorsCmd = &cobra.Command{
	Use:   "monitors",
	Short: "Show monitor configuration",
	Long:  `Display information about connected monitors and their configuration.`,
	RunE:  runMonitors,
}

func init() {
	monitorsCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.AddCommand(monitorsCmd)
}

func runMonitors(cmd *cobra.Command, args []string) error {
	// Initialize display detection
	disp, err := display.New()
	if err != nil {
		if jsonOutput {
			// Output error as JSON
			info := DisplayInfo{Error: err.Error()}
			return json.NewEncoder(os.Stdout).Encode(info)
		}
		return fmt.Errorf("failed to initialize display detection: %w", err)
	}
	defer disp.Close()

	// Get monitor information
	monitors := disp.GetMonitors()
	
	if jsonOutput {
		// Output JSON format for programmatic usage
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

	// Human-readable format
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