package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bnema/wayland-virtual-input-go/virtual_keyboard"
	"github.com/bnema/wayland-virtual-input-go/virtual_pointer"
)

func main() {
	fmt.Println("Testing Wayland Virtual Input Bindings")
	fmt.Println("======================================")

	ctx := context.Background()

	// Test 1: Virtual Pointer Manager
	fmt.Println("\n1. Testing Virtual Pointer Manager...")
	pointerMgr, err := virtual_pointer.NewVirtualPointerManager(ctx)
	if err != nil {
		log.Fatalf("Failed to create virtual pointer manager: %v", err)
	}
	fmt.Println("✓ Virtual pointer manager created successfully")

	// Create a virtual pointer
	fmt.Println("\n2. Creating Virtual Pointer...")
	virtualPtr, err := pointerMgr.CreatePointer()
	if err != nil {
		log.Fatalf("Failed to create virtual pointer: %v", err)
	}
	fmt.Println("✓ Virtual pointer created successfully")

	// Test mouse movement
	fmt.Println("\n3. Testing Mouse Movement...")
	fmt.Println("Moving mouse 100 pixels right and down in 2 seconds...")
	time.Sleep(2 * time.Second)

	err = virtualPtr.Motion(time.Now(), 100.0, 100.0)
	if err != nil {
		log.Printf("Failed to inject mouse motion: %v", err)
	} else {
		fmt.Println("✓ Mouse motion injected")
	}

	err = virtualPtr.Frame()
	if err != nil {
		log.Printf("Failed to frame pointer event: %v", err)
	} else {
		fmt.Println("✓ Pointer frame sent")
	}

	// Test mouse click
	fmt.Println("\n4. Testing Mouse Click...")
	fmt.Println("Clicking left mouse button in 1 second...")
	time.Sleep(1 * time.Second)

	// Press
	err = virtualPtr.Button(time.Now(), 272, virtual_pointer.ButtonStatePressed) // BTN_LEFT
	if err != nil {
		log.Printf("Failed to press mouse button: %v", err)
	} else {
		fmt.Println("✓ Mouse button pressed")
	}
	if err := virtualPtr.Frame(); err != nil {
		log.Printf("Failed to frame virtual pointer: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Release
	err = virtualPtr.Button(time.Now(), 272, virtual_pointer.ButtonStateReleased)
	if err != nil {
		log.Printf("Failed to release mouse button: %v", err)
	} else {
		fmt.Println("✓ Mouse button released")
	}
	if err := virtualPtr.Frame(); err != nil {
		log.Printf("Failed to frame virtual pointer: %v", err)
	}

	// Test mouse scroll
	fmt.Println("\n5. Testing Mouse Scroll...")
	fmt.Println("Scrolling down in 1 second...")
	time.Sleep(1 * time.Second)

	err = virtualPtr.AxisSource(virtual_pointer.AxisSourceWheel)
	if err != nil {
		log.Printf("Failed to set axis source: %v", err)
	}

	err = virtualPtr.Axis(time.Now(), virtual_pointer.AxisVertical, -5.0)
	if err != nil {
		log.Printf("Failed to inject scroll: %v", err)
	} else {
		fmt.Println("✓ Scroll event injected")
	}
	if err := virtualPtr.Frame(); err != nil {
		log.Printf("Failed to frame virtual pointer: %v", err)
	}

	// Test 6: Virtual Keyboard Manager
	fmt.Println("\n6. Testing Virtual Keyboard Manager...")
	keyboardMgr, err := virtual_keyboard.NewVirtualKeyboardManager(ctx)
	if err != nil {
		log.Printf("WARNING: Failed to create virtual keyboard manager: %v", err)
		fmt.Println("Your compositor may not support virtual keyboard protocol")
	} else {
		fmt.Println("✓ Virtual keyboard manager created successfully")

		// Create a virtual keyboard
		fmt.Println("\n7. Creating Virtual Keyboard...")
		virtualKbd, err := keyboardMgr.CreateKeyboard()
		if err != nil {
			log.Printf("Failed to create virtual keyboard: %v", err)
		} else {
			fmt.Println("✓ Virtual keyboard created successfully")

			// Test key press
			fmt.Println("\n8. Testing Key Press...")
			fmt.Println("Pressing 'A' key in 1 second...")
			time.Sleep(1 * time.Second)

			// Press A key (KEY_A = 30)
			err = virtualKbd.Key(time.Now(), 30, virtual_keyboard.KeyStatePressed)
			if err != nil {
				log.Printf("Failed to press key: %v", err)
			} else {
				fmt.Println("✓ Key pressed")
			}

			time.Sleep(100 * time.Millisecond)

			// Release A key
			err = virtualKbd.Key(time.Now(), 30, virtual_keyboard.KeyStateReleased)
			if err != nil {
				log.Printf("Failed to release key: %v", err)
			} else {
				fmt.Println("✓ Key released")
			}

			// Clean up keyboard
			if err := virtualKbd.Close(); err != nil {
				log.Printf("Failed to close virtual keyboard: %v", err)
			}
		}
		if err := keyboardMgr.Close(); err != nil {
			log.Printf("Failed to close keyboard manager: %v", err)
		}
	}

	// Clean up
	fmt.Println("\n9. Cleaning up...")
	if err := virtualPtr.Close(); err != nil {
		log.Printf("Failed to close virtual pointer: %v", err)
	}
	if err := pointerMgr.Close(); err != nil {
		log.Printf("Failed to close pointer manager: %v", err)
	}

	fmt.Println("\n✓ All tests completed!")
	fmt.Println("\nNote: If you didn't see mouse movement or keyboard input,")
	fmt.Println("make sure your Wayland compositor supports the virtual input protocols.")
	fmt.Println("Compositors like Hyprland, Sway, and other wlroots-based ones typically support these.")
}
