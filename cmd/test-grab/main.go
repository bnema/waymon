package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	charmlog "github.com/charmbracelet/log"
	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/protocol"
)

func main() {
	// Set log level to debug for verbose output
	logger.Logger.SetLevel(charmlog.DebugLevel)

	fmt.Println("Testing evdev device grab functionality...")
	fmt.Println("This test will grab your mouse and keyboard exclusively for 10 seconds")
	fmt.Println("Press Ctrl+C to stop early")

	// Create evdev capture
	capture := input.NewEvdevCapture()

	// Set up event handler
	capture.OnInputEvent(func(event *protocol.InputEvent) {
		switch e := event.Event.(type) {
		case *protocol.InputEvent_MouseMove:
			fmt.Printf("Mouse move: dx=%.0f, dy=%.0f\n", e.MouseMove.Dx, e.MouseMove.Dy)
		case *protocol.InputEvent_MouseButton:
			action := "released"
			if e.MouseButton.Pressed {
				action = "pressed"
			}
			fmt.Printf("Mouse button %d %s\n", e.MouseButton.Button, action)
		case *protocol.InputEvent_Keyboard:
			action := "released"
			if e.Keyboard.Pressed {
				action = "pressed"
			}
			fmt.Printf("Key %d %s\n", e.Keyboard.Key, action)
		}
	})

	// Start capture
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := capture.Start(ctx); err != nil {
		log.Fatalf("Failed to start capture: %v", err)
	}
	defer capture.Stop()

	fmt.Println("\nCapture started successfully")
	fmt.Println("Setting target to 'test-client' (this should grab devices)...")

	// Set target (this should trigger grab)
	if err := capture.SetTarget("test-client"); err != nil {
		log.Fatalf("Failed to set target (grab devices): %v", err)
	}

	fmt.Println("\nDevices grabbed successfully!")
	fmt.Println("Your mouse and keyboard input should now be captured exclusively")
	fmt.Println("Try moving your mouse - you should see events printed but cursor shouldn't move")

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run for 10 seconds or until interrupted
	timer := time.NewTimer(10 * time.Second)
	select {
	case <-timer.C:
		fmt.Println("\n10 seconds elapsed, releasing devices...")
	case <-sigChan:
		fmt.Println("\nInterrupted, releasing devices...")
	}

	// Clear target (this should ungrab)
	if err := capture.SetTarget(""); err != nil {
		log.Printf("Failed to clear target: %v", err)
	}

	fmt.Println("Devices released, input should work normally now")
}