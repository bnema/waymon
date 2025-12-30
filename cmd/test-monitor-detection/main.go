package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bnema/waymon/internal/adapters/out/display"
	"github.com/rs/zerolog"
)

func main() {
	// Create context with logger
	ctx := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		Level(zerolog.DebugLevel).
		With().
		Timestamp().
		Logger().
		WithContext(context.Background())

	fmt.Println("Testing monitor detection...")
	fmt.Printf("Running as UID: %d\n", os.Geteuid())
	fmt.Printf("SUDO_USER: %s\n", os.Getenv("SUDO_USER"))
	fmt.Printf("WAYLAND_DISPLAY: %s\n", os.Getenv("WAYLAND_DISPLAY"))
	fmt.Printf("XDG_RUNTIME_DIR: %s\n", os.Getenv("XDG_RUNTIME_DIR"))
	fmt.Println()

	// Create display adapter
	disp, err := display.New(ctx)
	if err != nil {
		fmt.Printf("Error creating display adapter: %v\n", err)
		os.Exit(1)
	}
	defer disp.Close()

	// Get monitors
	monitors, err := disp.GetMonitors(ctx)
	if err != nil {
		fmt.Printf("Error getting monitors: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Detected %d monitor(s):\n", len(monitors))

	for _, mon := range monitors {
		fmt.Printf("\nMonitor: %s\n", mon.Name)
		fmt.Printf("  ID: %s\n", mon.ID)
		fmt.Printf("  Resolution: %dx%d\n", mon.Width, mon.Height)
		fmt.Printf("  Position: (%d, %d)\n", mon.X, mon.Y)
		fmt.Printf("  Scale: %.1f\n", mon.Scale)
		if mon.Primary {
			fmt.Printf("  Primary: YES\n")
		}
	}
}
