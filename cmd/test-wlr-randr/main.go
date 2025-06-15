package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	fmt.Println("Testing wlr-randr with different environment configurations...")
	fmt.Println()

	// Test 1: Run wlr-randr normally
	fmt.Println("=== Test 1: Normal execution ===")
	runWlrRandr(nil)

	// Test 2: Run with sudo environment detection
	if os.Geteuid() == 0 {
		fmt.Println("\n=== Test 2: Running as root with sudo environment ===")
		sudoUser := os.Getenv("SUDO_USER")
		sudoUID := os.Getenv("SUDO_UID")
		fmt.Printf("SUDO_USER=%s, SUDO_UID=%s\n", sudoUser, sudoUID)

		if sudoUID != "" {
			env := os.Environ()
			
			// Set XDG_RUNTIME_DIR
			xdgRuntimeDir := fmt.Sprintf("/run/user/%s", sudoUID)
			env = append(env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntimeDir))
			fmt.Printf("Setting XDG_RUNTIME_DIR=%s\n", xdgRuntimeDir)

			// Try to detect WAYLAND_DISPLAY
			waylandDisplay := ""
			socketPath := fmt.Sprintf("/run/user/%s", sudoUID)
			if files, err := os.ReadDir(socketPath); err == nil {
				for _, file := range files {
					if strings.HasPrefix(file.Name(), "wayland-") && !strings.HasSuffix(file.Name(), ".lock") {
						waylandDisplay = file.Name()
						break
					}
				}
			}

			if waylandDisplay != "" {
				env = append(env, fmt.Sprintf("WAYLAND_DISPLAY=%s", waylandDisplay))
				fmt.Printf("Detected WAYLAND_DISPLAY=%s\n", waylandDisplay)
			} else if existingDisplay := os.Getenv("WAYLAND_DISPLAY"); existingDisplay != "" {
				env = append(env, fmt.Sprintf("WAYLAND_DISPLAY=%s", existingDisplay))
				fmt.Printf("Using existing WAYLAND_DISPLAY=%s\n", existingDisplay)
			} else {
				fmt.Println("WARNING: Could not detect WAYLAND_DISPLAY")
			}

			runWlrRandr(env)
		}
	} else {
		fmt.Println("\n=== To test sudo behavior, run with: sudo ./test-wlr-randr ===")
	}
}

func runWlrRandr(env []string) {
	// Try JSON mode first
	fmt.Println("\nTrying: wlr-randr --json")
	cmd := exec.Command("wlr-randr", "--json")
	if env != nil {
		cmd.Env = env
	}
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		if len(output) > 0 {
			fmt.Printf("Output: %s\n", string(output))
		}
	} else {
		fmt.Printf("Success! Output length: %d bytes\n", len(output))
		
		// Try to parse JSON
		var outputs []map[string]interface{}
		if err := json.Unmarshal(output, &outputs); err != nil {
			fmt.Printf("JSON parse error: %v\n", err)
		} else {
			fmt.Printf("Parsed %d outputs:\n", len(outputs))
			for i, out := range outputs {
				fmt.Printf("  Output %d: %v\n", i, out["name"])
				if currentMode, ok := out["current_mode"].(map[string]interface{}); ok {
					fmt.Printf("    Current mode: %vx%v\n", currentMode["width"], currentMode["height"])
				}
				fmt.Printf("    Width: %v, Height: %v\n", out["width"], out["height"])
				fmt.Printf("    Position: %v,%v\n", out["x"], out["y"])
				fmt.Printf("    Enabled: %v\n", out["enabled"])
			}
		}
	}

	// Try text mode
	fmt.Println("\nTrying: wlr-randr (text mode)")
	cmd = exec.Command("wlr-randr")
	if env != nil {
		cmd.Env = env
	}
	
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		if len(output) > 0 {
			fmt.Printf("Output: %s\n", string(output))
		}
	} else {
		fmt.Printf("Success! Output:\n%s\n", string(output))
	}
}