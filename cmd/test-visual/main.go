// test-visual creates a visual test pattern with the mouse
package main

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/proto"
)

func main() {
	fmt.Println("Waymon Visual Mouse Test")
	fmt.Println("=======================")
	fmt.Println()
	fmt.Println("This will draw a circle with your mouse cursor")
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()
	fmt.Println("Starting in 3 seconds...")
	time.Sleep(3 * time.Second)

	// Create handler
	handler, err := input.NewHandler()
	if err != nil {
		log.Fatalf("Failed to create input handler: %v", err)
	}
	defer handler.Close()

	coord := input.NewCoordinator(handler)

	// Get starting position (center of circle)
	centerX, centerY := 960.0, 540.0 // Center of 1920x1080 screen
	radius := 200.0
	steps := 60

	fmt.Printf("Drawing circle at (%.0f, %.0f) with radius %.0f\n", centerX, centerY, radius)

	// Move to starting position
	startEvent := &proto.MouseEvent{
		Type:        proto.EventType_EVENT_TYPE_MOVE,
		X:           centerX + radius,
		Y:           centerY,
		TimestampMs: time.Now().UnixMilli(),
	}
	if err := coord.ProcessEvent(startEvent); err != nil {
		log.Fatalf("Error moving to start: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	// Draw circle
	for {
		for i := 0; i <= steps; i++ {
			angle := float64(i) * 2 * math.Pi / float64(steps)
			x := centerX + radius*math.Cos(angle)
			y := centerY + radius*math.Sin(angle)

			event := &proto.MouseEvent{
				Type:        proto.EventType_EVENT_TYPE_MOVE,
				X:           x,
				Y:           y,
				TimestampMs: time.Now().UnixMilli(),
			}

			if err := coord.ProcessEvent(event); err != nil {
				log.Printf("Error moving mouse: %v", err)
			}

			time.Sleep(20 * time.Millisecond) // 50 FPS
		}
	}
}
