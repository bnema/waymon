package cmd

import (
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/proto"
)

func testVisualMain() error {
	fmt.Println("Waymon Visual Test")
	fmt.Println("==================")
	fmt.Println()
	fmt.Println("This test will move your mouse in a circle pattern.")
	fmt.Println("Make sure you have a window open where you can see mouse movement.")
	fmt.Println()

	// Check if running with sudo
	if os.Geteuid() != 0 {
		return fmt.Errorf("this test requires root privileges for uinput access\nPlease run with: sudo waymon test input")
	}

	fmt.Println("Starting in 3 seconds... (Press Ctrl+C to stop)")
	time.Sleep(3 * time.Second)

	// Create input handler
	handler, err := input.NewHandler()
	if err != nil {
		log.Fatalf("Failed to create input handler: %v", err)
	}
	defer handler.Close()

	fmt.Println("Drawing circles with the mouse...")
	fmt.Println("Watch your cursor move!")

	// Parameters for circle
	centerX := float64(500)
	centerY := float64(500)
	radius := float64(100)
	steps := 60
	duration := 5 * time.Second

	// Move in a circle
	startTime := time.Now()
	stepDuration := duration / time.Duration(steps)

	for i := 0; i < steps && time.Since(startTime) < duration; i++ {
		// Calculate position on circle
		angle := float64(i) * (2 * math.Pi / float64(steps))
		x := centerX + radius*math.Cos(angle)
		y := centerY + radius*math.Sin(angle)

		// Create mouse move event
		event := &proto.MouseEvent{
			Type: proto.EventType_EVENT_TYPE_MOVE,
			X:    x,
			Y:    y,
		}

		// Handle the event
		if err := handler.ProcessEvent(event); err != nil {
			log.Printf("Error handling event: %v", err)
		}

		// Wait before next move
		time.Sleep(stepDuration)
	}

	fmt.Println("\nTest complete!")
	fmt.Println("Did you see the mouse move in circles?")

	// Test click
	fmt.Println("\nTesting mouse click in 2 seconds...")
	time.Sleep(2 * time.Second)

	clickEvent := &proto.MouseEvent{
		Type:   proto.EventType_EVENT_TYPE_CLICK,
		X:      centerX,
		Y:      centerY,
		Button: proto.MouseButton_MOUSE_BUTTON_LEFT,
	}

	if err := handler.ProcessEvent(clickEvent); err != nil {
		log.Printf("Error handling click: %v", err)
	}

	fmt.Println("Click test complete!")

	return nil
}
