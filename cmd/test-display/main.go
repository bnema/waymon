// test-display shows detected monitor configuration
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
		With().
		Timestamp().
		Logger().
		WithContext(context.Background())

	log := zerolog.Ctx(ctx)
	log.Info().Msg("Waymon Display Detection Test")
	log.Info().Msg("=============================")

	// Create display adapter
	disp, err := display.New(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create display adapter")
	}

	// Show monitors
	monitors, err := disp.GetMonitors(ctx)
	if err != nil {
		if closeErr := disp.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("Failed to close display adapter")
		}
		log.Fatal().Err(err).Msg("Failed to get monitors")
	}
	// Defer close after error checks are done
	defer func() {
		if err := disp.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close display adapter")
		}
	}()

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
	pos, err := disp.GetCursorPosition(ctx)
	if err != nil {
		fmt.Printf("Cursor position: unavailable (%v)\n", err)
		fmt.Println("Note: Cursor tracking on Wayland requires special permissions")
		fmt.Println("      We'll track position internally based on movements")
	} else {
		fmt.Printf("Cursor position: %.0f,%.0f\n", pos.X, pos.Y)
	}
}
