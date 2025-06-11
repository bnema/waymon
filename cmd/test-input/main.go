// test-input is a manual testing tool for uinput functionality
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/proto"
)

func main() {
	fmt.Println("Waymon Input Test Tool")
	fmt.Println("=====================")
	fmt.Println()
	fmt.Println("This tool will test mouse input injection.")
	fmt.Println("Make sure you have proper permissions for /dev/uinput")
	fmt.Println()
	fmt.Println("The test will:")
	fmt.Println("1. Move the mouse in a square pattern")
	fmt.Println("2. Click at each corner")
	fmt.Println("3. Scroll up and down")
	fmt.Println()
	fmt.Println("Press Enter to start (you have 3 seconds to position your mouse)...")
	fmt.Scanln()

	// Create handler
	handler, err := input.NewHandler()
	if err != nil {
		log.Fatalf("Failed to create input handler: %v", err)
	}
	defer handler.Close()

	fmt.Println("Starting in 3 seconds...")
	time.Sleep(3 * time.Second)

	// Create coordinator
	coord := input.NewCoordinator(handler)

	// Test 1: Move in a square
	fmt.Println("Test 1: Moving in a square pattern...")
	movements := []struct {
		x, y float64
		desc string
	}{
		{100, 100, "top-left"},
		{300, 100, "top-right"},
		{300, 300, "bottom-right"},
		{100, 300, "bottom-left"},
		{100, 100, "back to start"},
	}

	for _, move := range movements {
		fmt.Printf("  Moving to %s (%v, %v)\n", move.desc, move.x, move.y)
		event := &proto.MouseEvent{
			Type:        proto.EventType_EVENT_TYPE_MOVE,
			X:           move.x,
			Y:           move.y,
			TimestampMs: time.Now().UnixMilli(),
		}
		if err := coord.ProcessEvent(event); err != nil {
			log.Printf("Error moving mouse: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Test 2: Clicks
	fmt.Println("\nTest 2: Testing clicks...")
	buttons := []struct {
		button proto.MouseButton
		name   string
	}{
		{proto.MouseButton_MOUSE_BUTTON_LEFT, "left"},
		{proto.MouseButton_MOUSE_BUTTON_RIGHT, "right"},
		{proto.MouseButton_MOUSE_BUTTON_MIDDLE, "middle"},
	}

	for _, btn := range buttons {
		fmt.Printf("  Testing %s click\n", btn.name)
		
		// Press
		pressEvent := &proto.MouseEvent{
			Type:        proto.EventType_EVENT_TYPE_CLICK,
			X:           200,
			Y:           200,
			Button:      btn.button,
			IsPressed:   true,
			TimestampMs: time.Now().UnixMilli(),
		}
		if err := coord.ProcessEvent(pressEvent); err != nil {
			log.Printf("Error pressing %s button: %v", btn.name, err)
		}
		
		time.Sleep(100 * time.Millisecond)
		
		// Release
		releaseEvent := &proto.MouseEvent{
			Type:        proto.EventType_EVENT_TYPE_CLICK,
			X:           200,
			Y:           200,
			Button:      btn.button,
			IsPressed:   false,
			TimestampMs: time.Now().UnixMilli(),
		}
		if err := coord.ProcessEvent(releaseEvent); err != nil {
			log.Printf("Error releasing %s button: %v", btn.name, err)
		}
		
		time.Sleep(500 * time.Millisecond)
	}

	// Test 3: Scrolling
	fmt.Println("\nTest 3: Testing scroll...")
	scrolls := []struct {
		dir  proto.ScrollDirection
		name string
	}{
		{proto.ScrollDirection_SCROLL_DIRECTION_UP, "up"},
		{proto.ScrollDirection_SCROLL_DIRECTION_DOWN, "down"},
	}

	for _, scroll := range scrolls {
		fmt.Printf("  Scrolling %s\n", scroll.name)
		for i := 0; i < 3; i++ {
			event := &proto.MouseEvent{
				Type:        proto.EventType_EVENT_TYPE_SCROLL,
				X:           200,
				Y:           200,
				Direction:   scroll.dir,
				TimestampMs: time.Now().UnixMilli(),
			}
			if err := coord.ProcessEvent(event); err != nil {
				log.Printf("Error scrolling %s: %v", scroll.name, err)
			}
			time.Sleep(200 * time.Millisecond)
		}
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\nTest completed!")
}