package cmd

import (
	"fmt"

	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/logger"
)

func testDisplayMain() error {
	logger.Info("Waymon Display Detection Test")
	logger.Info("=============================")
	logger.Info("")

	// Create display manager
	disp, err := display.New()
	if err != nil {
		return fmt.Errorf("failed to create display manager: %v", err)
	}
	defer disp.Close()

	// Show monitors
	monitors := disp.GetMonitors()
	logger.Infof("Detected %d monitor(s):", len(monitors))
	logger.Info("")

	for i, mon := range monitors {
		logger.Infof("Monitor %d: %s", i+1, mon.Name)
		logger.Infof("  ID:       %s", mon.ID)
		logger.Infof("  Position: %d,%d", mon.X, mon.Y)
		logger.Infof("  Size:     %dx%d", mon.Width, mon.Height)
		logger.Infof("  Primary:  %v", mon.Primary)
		if mon.Scale != 0 && mon.Scale != 1 {
			logger.Infof("  Scale:    %.2f", mon.Scale)
		}
		logger.Info("")
	}

	// Try to get cursor position
	x, y, monitor, err := disp.GetCursorPosition()
	if err != nil {
		logger.Infof("Cursor position: unavailable (%v)", err)
		logger.Info("Note: Cursor tracking on Wayland requires special permissions")
		logger.Info("      We'll track position internally based on movements")
	} else {
		if monitor != nil {
			logger.Infof("Cursor position: %d,%d (on %s)", x, y, monitor.Name)
		} else {
			logger.Infof("Cursor position: %d,%d", x, y)
		}
	}

	// Show edge detection zones
	logger.Info("Edge detection zones:")
	primary := disp.GetPrimaryMonitor()
	if primary != nil {
		x1, y1, x2, y2 := primary.Bounds()
		threshold := int32(5)
		logger.Infof("  Left edge:   x < %d", x1+threshold)
		logger.Infof("  Right edge:  x > %d", x2-threshold)
		logger.Infof("  Top edge:    y < %d", y1+threshold)
		logger.Infof("  Bottom edge: y > %d", y2-threshold)
	}

	// Test arrangement detection
	logger.Info("Monitor arrangement:")
	for _, mon := range monitors {
		x, y := mon.X+mon.Width/2, mon.Y+mon.Height/2

		// Check what's to the right
		rightMon := disp.GetMonitorAt(x+mon.Width, y)
		if rightMon != nil && rightMon != mon {
			logger.Infof("  %s -> %s (right)", mon.Name, rightMon.Name)
		}

		// Check what's below
		belowMon := disp.GetMonitorAt(x, y+mon.Height)
		if belowMon != nil && belowMon != mon {
			logger.Infof("  %s -> %s (below)", mon.Name, belowMon.Name)
		}
	}

	return nil
}
