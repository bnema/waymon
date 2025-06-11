package cmd

import (
	"fmt"

	"github.com/bnema/waymon/internal/display"
)

func testDisplayMain() error {
	fmt.Println("Waymon Display Detection Test")
	fmt.Println("=============================")
	fmt.Println()

	// Create display manager
	disp, err := display.New()
	if err != nil {
		return fmt.Errorf("failed to create display manager: %v", err)
	}
	defer disp.Close()

	// Show monitors
	monitors := disp.GetMonitors()
	fmt.Printf("Detected %d monitor(s):\n\n", len(monitors))

	for i, mon := range monitors {
		fmt.Printf("Monitor %d: %s\n", i+1, mon.Name)
		fmt.Printf("  ID:       %s\n", mon.ID)
		fmt.Printf("  Position: %d,%d\n", mon.X, mon.Y)
		fmt.Printf("  Size:     %dx%d\n", mon.Width, mon.Height)
		fmt.Printf("  Primary:  %v\n", mon.Primary)
		if mon.Scale != 0 && mon.Scale != 1 {
			fmt.Printf("  Scale:    %.2f\n", mon.Scale)
		}
		fmt.Println()
	}

	// Try to get cursor position
	x, y, monitor, err := disp.GetCursorPosition()
	if err != nil {
		fmt.Printf("Cursor position: unavailable (%v)\n", err)
		fmt.Println("Note: Cursor tracking on Wayland requires special permissions")
		fmt.Println("      We'll track position internally based on movements")
	} else {
		fmt.Printf("Cursor position: %d,%d", x, y)
		if monitor != nil {
			fmt.Printf(" (on %s)", monitor.Name)
		}
		fmt.Println()
	}

	// Show edge detection zones
	fmt.Println("\nEdge detection zones:")
	primary := disp.GetPrimaryMonitor()
	if primary != nil {
		x1, y1, x2, y2 := primary.Bounds()
		threshold := int32(5)
		fmt.Printf("  Left edge:   x < %d\n", x1+threshold)
		fmt.Printf("  Right edge:  x > %d\n", x2-threshold)
		fmt.Printf("  Top edge:    y < %d\n", y1+threshold)
		fmt.Printf("  Bottom edge: y > %d\n", y2-threshold)
	}

	// Test arrangement detection
	fmt.Println("\nMonitor arrangement:")
	for _, mon := range monitors {
		x, y := mon.X+mon.Width/2, mon.Y+mon.Height/2
		
		// Check what's to the right
		rightMon := disp.GetMonitorAt(x+mon.Width, y)
		if rightMon != nil && rightMon != mon {
			fmt.Printf("  %s -> %s (right)\n", mon.Name, rightMon.Name)
		}
		
		// Check what's below
		belowMon := disp.GetMonitorAt(x, y+mon.Height)
		if belowMon != nil && belowMon != mon {
			fmt.Printf("  %s -> %s (below)\n", mon.Name, belowMon.Name)
		}
	}

	return nil
}