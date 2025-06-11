package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bnema/waymon/internal/display"
	"github.com/spf13/cobra"
)

var displayHelperCmd = &cobra.Command{
	Use:    "display-helper",
	Hidden: true, // Hidden from normal help
	Short:  "Internal helper for display detection",
	RunE:   runDisplayHelper,
}

func init() {
	rootCmd.AddCommand(displayHelperCmd)
}

// DisplayInfo is the JSON structure for display information
type DisplayInfo struct {
	Monitors []MonitorInfo `json:"monitors"`
	Error    string        `json:"error,omitempty"`
}

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

func runDisplayHelper(cmd *cobra.Command, args []string) error {
	// Clear SUDO_USER to prevent infinite recursion
	os.Unsetenv("SUDO_USER")
	os.Unsetenv("SUDO_UID")
	os.Unsetenv("SUDO_GID")
	os.Unsetenv("SUDO_COMMAND")
	
	// This runs as the normal user to detect displays
	disp, err := display.New()
	if err != nil {
		// Output error as JSON
		info := DisplayInfo{Error: err.Error()}
		json.NewEncoder(os.Stdout).Encode(info)
		return nil // Don't return error to avoid extra output
	}
	defer disp.Close()

	// Convert monitors to JSON structure
	monitors := disp.GetMonitors()
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

	// Output as JSON
	if err := json.NewEncoder(os.Stdout).Encode(info); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode display info: %v\n", err)
		return err
	}

	return nil
}