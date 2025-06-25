package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/protocol"
	"github.com/charmbracelet/lipgloss"
)

var (
	verbose = flag.Bool("v", false, "verbose output")
	timeout = flag.Duration("timeout", 10*time.Second, "test timeout")

	// Lipgloss styles
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).MarginTop(1).MarginBottom(1)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	testStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	dimStyle     = lipgloss.NewStyle().Faint(true)
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

type TestResult struct {
	Name    string
	Passed  bool
	Message string
}

func main() {
	flag.Parse()

	fmt.Println(titleStyle.Render("=== Waymon Wayland Injection Tests ==="))
	fmt.Println(infoStyle.Render("Testing Wayland virtual input device creation and injection"))
	fmt.Println(warnStyle.Render("Note: This test requires a running Wayland compositor with virtual input support"))
	fmt.Println()

	// Check if we're running under Wayland
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		fmt.Println(errorStyle.Render("Error: WAYLAND_DISPLAY not set - not running under Wayland"))
		fmt.Println(dimStyle.Render("This test must be run in a Wayland session"))
		os.Exit(1)
	}

	var results []TestResult

	// Run tests
	results = append(results, testWaylandBackendCreation())
	results = append(results, testVirtualDeviceCreation())
	results = append(results, testMouseInjection())
	results = append(results, testKeyboardInjection())
	results = append(results, testCombinedInjection())

	// Print summary
	printSummary(results)
}

func testWaylandBackendCreation() TestResult {
	fmt.Println(testStyle.Render("[Test: Wayland Backend Creation]"))

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Try to create Wayland virtual input backend
	backend, err := input.CreateClientBackend()
	if err != nil {
		return TestResult{
			Name:    "Wayland Backend Creation",
			Passed:  false,
			Message: fmt.Sprintf("Failed to create backend: %v", err),
		}
	}

	// Start the backend
	if err := backend.Start(ctx); err != nil {
		return TestResult{
			Name:    "Wayland Backend Creation",
			Passed:  false,
			Message: fmt.Sprintf("Failed to start backend: %v", err),
		}
	}

	// Clean shutdown
	if err := backend.Stop(); err != nil {
		return TestResult{
			Name:    "Wayland Backend Creation",
			Passed:  false,
			Message: fmt.Sprintf("Failed to stop backend: %v", err),
		}
	}

	return TestResult{
		Name:    "Wayland Backend Creation",
		Passed:  true,
		Message: "Successfully created and started Wayland virtual input backend",
	}
}

func testVirtualDeviceCreation() TestResult {
	fmt.Println("\n" + testStyle.Render("[Test: Virtual Device Creation]"))

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	backend, err := input.CreateClientBackend()
	if err != nil {
		return TestResult{
			Name:    "Virtual Device Creation",
			Passed:  false,
			Message: fmt.Sprintf("Failed to create backend: %v", err),
		}
	}

	if err := backend.Start(ctx); err != nil {
		return TestResult{
			Name:    "Virtual Device Creation",
			Passed:  false,
			Message: fmt.Sprintf("Failed to start backend: %v", err),
		}
	}
	defer backend.Stop()

	// Give time for device creation
	time.Sleep(500 * time.Millisecond)

	// The fact that Start() succeeded means virtual devices were created
	// We can't easily verify they exist without compositor-specific tools

	return TestResult{
		Name:    "Virtual Device Creation",
		Passed:  true,
		Message: "Virtual pointer and keyboard devices created successfully",
	}
}

func testMouseInjection() TestResult {
	fmt.Println("\n" + testStyle.Render("[Test: Mouse Injection]"))
	fmt.Println(infoStyle.Render("Watch your mouse cursor - it should move in a square pattern"))

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	backend, err := input.CreateClientBackend()
	if err != nil {
		return TestResult{
			Name:    "Mouse Injection",
			Passed:  false,
			Message: fmt.Sprintf("Failed to create backend: %v", err),
		}
	}

	if err := backend.Start(ctx); err != nil {
		return TestResult{
			Name:    "Mouse Injection",
			Passed:  false,
			Message: fmt.Sprintf("Failed to start backend: %v", err),
		}
	}
	defer backend.Stop()

	// Set up injection via callback
	injected := 0
	backend.OnInputEvent(func(event *protocol.InputEvent) {
		// This would be called by the real implementation
		injected++
	})

	// Inject mouse movement in a square pattern
	movements := []struct {
		dx, dy float64
		delay  time.Duration
	}{
		{50.0, 0, 200 * time.Millisecond},  // Right
		{0, 50.0, 200 * time.Millisecond},  // Down
		{-50.0, 0, 200 * time.Millisecond}, // Left
		{0, -50.0, 200 * time.Millisecond}, // Up
	}

	fmt.Print(dimStyle.Render("Injecting mouse movements: "))

	// Get the Wayland backend to inject directly
	if waylandBackend, ok := backend.(*input.WaylandVirtualInput); ok {
		for i, move := range movements {
			if err := waylandBackend.InjectMouseMove(move.dx, move.dy); err != nil {
				fmt.Println(errorStyle.Render(fmt.Sprintf("✗ Error: %v", err)))
				return TestResult{
					Name:    "Mouse Injection",
					Passed:  false,
					Message: fmt.Sprintf("Failed to inject mouse movement: %v", err),
				}
			}

			fmt.Print(dimStyle.Render("→"))

			if i < len(movements)-1 {
				time.Sleep(move.delay)
			}
		}
		fmt.Println(successStyle.Render(" ✓"))

		// Test mouse button click
		fmt.Print(dimStyle.Render("Injecting mouse click: "))

		// Press
		if err := waylandBackend.InjectMouseButton(1, true); err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("✗ Error: %v", err)))
			return TestResult{
				Name:    "Mouse Injection",
				Passed:  false,
				Message: fmt.Sprintf("Failed to inject mouse button press: %v", err),
			}
		}
		time.Sleep(50 * time.Millisecond)

		// Release
		if err := waylandBackend.InjectMouseButton(1, false); err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("✗ Error: %v", err)))
			return TestResult{
				Name:    "Mouse Injection",
				Passed:  false,
				Message: fmt.Sprintf("Failed to inject mouse button release: %v", err),
			}
		}
		fmt.Println(successStyle.Render("✓"))

		return TestResult{
			Name:    "Mouse Injection",
			Passed:  true,
			Message: "Mouse movements and clicks injected successfully",
		}
	}

	return TestResult{
		Name:    "Mouse Injection",
		Passed:  false,
		Message: "Could not cast to WaylandVirtualInput backend",
	}
}

func testKeyboardInjection() TestResult {
	fmt.Println("\n" + testStyle.Render("[Test: Keyboard Injection]"))
	fmt.Println(infoStyle.Render("Focus a text editor - the test will type 'Hello Waymon!'"))
	fmt.Print(dimStyle.Render("Waiting 3 seconds for you to focus a text field..."))

	time.Sleep(3 * time.Second)
	fmt.Println(dimStyle.Render(" ready!"))

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	backend, err := input.CreateClientBackend()
	if err != nil {
		return TestResult{
			Name:    "Keyboard Injection",
			Passed:  false,
			Message: fmt.Sprintf("Failed to create backend: %v", err),
		}
	}

	if err := backend.Start(ctx); err != nil {
		return TestResult{
			Name:    "Keyboard Injection",
			Passed:  false,
			Message: fmt.Sprintf("Failed to start backend: %v", err),
		}
	}
	defer backend.Stop()

	// Linux key codes for "Hello Waymon!"
	// These are approximations - real mapping depends on layout
	keySequence := []struct {
		key   uint32
		shift bool
		char  string
	}{
		{35, true, "H"},  // H (shift+h)
		{18, false, "e"}, // e
		{38, false, "l"}, // l
		{38, false, "l"}, // l
		{24, false, "o"}, // o
		{57, false, " "}, // space
		{50, true, "W"},  // W (shift+w)
		{30, false, "a"}, // a
		{21, false, "y"}, // y
		{50, false, "m"}, // m
		{24, false, "o"}, // o
		{49, false, "n"}, // n
		{2, true, "!"},   // ! (shift+1)
	}

	if waylandBackend, ok := backend.(*input.WaylandVirtualInput); ok {
		fmt.Print(dimStyle.Render("Typing: "))

		for _, key := range keySequence {
			// Press shift if needed
			if key.shift {
				if err := waylandBackend.InjectKeyEvent(42, true); err != nil {
					fmt.Println(errorStyle.Render(fmt.Sprintf("✗ Error: %v", err)))
					return TestResult{
						Name:    "Keyboard Injection",
						Passed:  false,
						Message: fmt.Sprintf("Failed to inject shift press: %v", err),
					}
				}
				time.Sleep(10 * time.Millisecond)
			}

			// Key press
			if err := waylandBackend.InjectKeyEvent(key.key, true); err != nil {
				fmt.Println(errorStyle.Render(fmt.Sprintf("✗ Error: %v", err)))
				return TestResult{
					Name:    "Keyboard Injection",
					Passed:  false,
					Message: fmt.Sprintf("Failed to inject key press: %v", err),
				}
			}
			time.Sleep(50 * time.Millisecond) // Increased delay to avoid duplicate key events

			// Key release
			if err := waylandBackend.InjectKeyEvent(key.key, false); err != nil {
				fmt.Println(errorStyle.Render(fmt.Sprintf("✗ Error: %v", err)))
				return TestResult{
					Name:    "Keyboard Injection",
					Passed:  false,
					Message: fmt.Sprintf("Failed to inject key release: %v", err),
				}
			}

			// Release shift if needed
			if key.shift {
				if err := waylandBackend.InjectKeyEvent(42, false); err != nil {
					fmt.Println(errorStyle.Render(fmt.Sprintf("✗ Error: %v", err)))
					return TestResult{
						Name:    "Keyboard Injection",
						Passed:  false,
						Message: fmt.Sprintf("Failed to inject shift release: %v", err),
					}
				}
			}

			time.Sleep(20 * time.Millisecond) // Small delay after key release
			fmt.Print(successStyle.Render(key.char))
			time.Sleep(50 * time.Millisecond)
		}

		fmt.Println()

		return TestResult{
			Name:    "Keyboard Injection",
			Passed:  true,
			Message: "Keyboard events injected successfully",
		}
	}

	return TestResult{
		Name:    "Keyboard Injection",
		Passed:  false,
		Message: "Could not cast to WaylandVirtualInput backend",
	}
}

func testCombinedInjection() TestResult {
	fmt.Println("\n" + testStyle.Render("[Test: Combined Mouse & Keyboard]"))
	fmt.Println(infoStyle.Render("Testing simultaneous mouse and keyboard injection"))

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	backend, err := input.CreateClientBackend()
	if err != nil {
		return TestResult{
			Name:    "Combined Injection",
			Passed:  false,
			Message: fmt.Sprintf("Failed to create backend: %v", err),
		}
	}

	if err := backend.Start(ctx); err != nil {
		return TestResult{
			Name:    "Combined Injection",
			Passed:  false,
			Message: fmt.Sprintf("Failed to start backend: %v", err),
		}
	}
	defer backend.Stop()

	if waylandBackend, ok := backend.(*input.WaylandVirtualInput); ok {
		// Inject Ctrl+Click (common UI pattern)
		fmt.Print(dimStyle.Render("Injecting Ctrl+Click: "))

		// Press Ctrl
		if err := waylandBackend.InjectKeyEvent(29, true); err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("✗ Error: %v", err)))
			return TestResult{
				Name:    "Combined Injection",
				Passed:  false,
				Message: fmt.Sprintf("Failed to inject Ctrl press: %v", err),
			}
		}
		time.Sleep(50 * time.Millisecond)

		// Move mouse
		if err := waylandBackend.InjectMouseMove(20.0, 20.0); err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("✗ Error: %v", err)))
			return TestResult{
				Name:    "Combined Injection",
				Passed:  false,
				Message: fmt.Sprintf("Failed to inject mouse move: %v", err),
			}
		}
		time.Sleep(50 * time.Millisecond)

		// Click mouse
		if err := waylandBackend.InjectMouseButton(1, true); err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("✗ Error: %v", err)))
			return TestResult{
				Name:    "Combined Injection",
				Passed:  false,
				Message: fmt.Sprintf("Failed to inject mouse click: %v", err),
			}
		}
		time.Sleep(50 * time.Millisecond)

		// Release mouse
		if err := waylandBackend.InjectMouseButton(1, false); err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("✗ Error: %v", err)))
			return TestResult{
				Name:    "Combined Injection",
				Passed:  false,
				Message: fmt.Sprintf("Failed to inject mouse release: %v", err),
			}
		}
		time.Sleep(50 * time.Millisecond)

		// Release Ctrl
		if err := waylandBackend.InjectKeyEvent(29, false); err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("✗ Error: %v", err)))
			return TestResult{
				Name:    "Combined Injection",
				Passed:  false,
				Message: fmt.Sprintf("Failed to inject Ctrl release: %v", err),
			}
		}

		fmt.Println(successStyle.Render("✓"))

		return TestResult{
			Name:    "Combined Injection",
			Passed:  true,
			Message: "Combined mouse and keyboard events injected successfully",
		}
	}

	return TestResult{
		Name:    "Combined Injection",
		Passed:  false,
		Message: "Could not cast to WaylandVirtualInput backend",
	}
}

func printSummary(results []TestResult) {
	fmt.Println("\n" + titleStyle.Render("=== Test Summary ==="))

	passed := 0
	failed := 0

	for _, result := range results {
		if result.Passed {
			passed++
			fmt.Println(successStyle.Render(fmt.Sprintf("✓ %s: %s", result.Name, result.Message)))
		} else {
			failed++
			fmt.Println(errorStyle.Render(fmt.Sprintf("✗ %s: %s", result.Name, result.Message)))
		}
	}

	status := successStyle
	if failed > 0 {
		status = errorStyle
	}

	fmt.Println("\n" + status.Render(fmt.Sprintf("Total: %d passed, %d failed", passed, failed)))

	if failed > 0 {
		os.Exit(1)
	}
}
