// test-visual creates a visual test to verify mouse capture and injection
package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/protocol"
	"github.com/gvalkov/golang-evdev"
)

func main() {
	logger.Info("🖱️  Waymon Mouse Capture & Injection Test")
	logger.Info("==========================================")
	logger.Info("")
	logger.Info("This test will:")
	logger.Info("1. Load mouse device from config")
	logger.Info("2. Test direct evdev capture")
	logger.Info("3. Test Wayland virtual input injection")
	logger.Info("4. Draw a circle to verify mouse movement")
	logger.Info("")
	logger.Info("Press Ctrl+C to stop")
	logger.Info("")

	// Initialize and load config
	if err := config.Init(); err != nil {
		logger.Errorf("❌ Failed to initialize config: %v", err)
		os.Exit(1)
	}
	
	cfg := config.Get()
	if cfg.Input.MouseDevice == "" {
		logger.Error("❌ No mouse device configured!")
		logger.Info("Run 'waymon setup --devices' to configure devices")
		os.Exit(1)
	}

	logger.Infof("📍 Using mouse device: %s", cfg.Input.MouseDevice)

	// Test 1: Direct evdev access
	logger.Info("\n🔍 Test 1: Direct evdev device access")
	if err := testDirectEvdev(cfg.Input.MouseDevice); err != nil {
		logger.Errorf("❌ Direct evdev test failed: %v", err)
		os.Exit(1)
	}
	logger.Info("✅ Direct evdev access: SUCCESS")

	// Test 2: Wayland virtual input
	logger.Info("\n🔍 Test 2: Wayland virtual input injection")
	waylandBackend, err := testWaylandVirtualInput()
	if err != nil {
		logger.Errorf("❌ Wayland virtual input test failed: %v", err)
		os.Exit(1)
	}
	logger.Info("✅ Wayland virtual input: SUCCESS")
	defer waylandBackend.Stop()

	// Test 3: Evdev capture with Wayland injection (full pipeline)
	logger.Info("\n🔍 Test 3: Full capture + injection pipeline")
	if err := testFullPipeline(cfg.Input.MouseDevice, waylandBackend); err != nil {
		logger.Errorf("❌ Full pipeline test failed: %v", err)
		os.Exit(1)
	}

	logger.Info("✅ All tests passed!")
}

// testDirectEvdev tests direct access to the evdev device
func testDirectEvdev(devicePath string) error {
	logger.Infof("Opening device: %s", devicePath)
	
	device, err := evdev.Open(devicePath)
	if err != nil {
		return fmt.Errorf("failed to open device: %w", err)
	}
	// Note: evdev.InputDevice doesn't have Close method

	if device == nil {
		return fmt.Errorf("device is nil after opening")
	}

	logger.Infof("✓ Device opened: %s", device.Name)
	logger.Infof("  Physical: %s", device.Phys)
	
	// Check device capabilities
	caps := device.Capabilities
	if caps != nil {
		logger.Info("Device capabilities:")
		for evType, _ := range caps {
			if evType.Type == evdev.EV_REL {
				logger.Info("  - Supports relative movement (mouse-like)")
			}
			if evType.Type == evdev.EV_KEY {
				logger.Info("  - Supports key/button events")
			}
			if evType.Type == evdev.EV_ABS {
				logger.Info("  - Supports absolute positioning")
			}
		}
	}
	
	// First try reading without grabbing
	logger.Info("Testing event reading WITHOUT grab (move your mouse)...")
	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel1()
	
	if err := testReadEvents(device, ctx1, false); err != nil {
		logger.Warnf("Reading without grab failed: %v", err)
		logger.Info("Now testing WITH grab...")
		
		// Test grabbing
		logger.Info("Testing device grab...")
		if err := device.Grab(); err != nil {
			return fmt.Errorf("failed to grab device: %w", err)
		}
		logger.Info("✓ Device grabbed successfully")
		defer device.Release()
		
		// Test reading events with grab
		ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel2()
		
		if err := testReadEvents(device, ctx2, true); err != nil {
			return err
		}
	}
	
	return nil
}

// testReadEvents tests reading events from the device
func testReadEvents(device *evdev.InputDevice, ctx context.Context, grabbed bool) error {
	grabStatus := "WITHOUT grab"
	if grabbed {
		grabStatus = "WITH grab"
	}
	logger.Infof("Testing event reading %s (move your mouse)...", grabStatus)
	
	eventCount := 0
	eventChan := make(chan *evdev.InputEvent, 100)
	errChan := make(chan error, 1)
	
	// Start a goroutine to read events
	go func() {
		logger.Info("Event reader goroutine started")
		readCount := 0
		emptyReads := 0
		for {
			select {
			case <-ctx.Done():
				logger.Infof("Reader goroutine: context done, exiting after %d reads (%d empty)", readCount, emptyReads)
				return
			default:
				// Try using Read() instead of ReadOne() to get multiple events at once
				events, err := device.Read()
				readCount++
				if err != nil {
					logger.Errorf("Read error after %d attempts: %v", readCount, err)
					select {
					case errChan <- err:
					default:
					}
					return
				}
				
				if len(events) > 0 {
					logger.Infof("📍 Read %d events on attempt %d", len(events), readCount)
					for _, event := range events {
						select {
						case eventChan <- &event:
						case <-ctx.Done():
							return
						}
					}
				} else {
					emptyReads++
					if emptyReads % 10 == 0 {
						logger.Debugf("Still reading... %d empty reads so far", emptyReads)
					}
				}
			}
		}
	}()
	
	for {
		select {
		case <-ctx.Done():
			if eventCount == 0 {
				return fmt.Errorf("no mouse events detected - device may not be correct or mouse not moved")
			}
			logger.Infof("✓ Detected %d mouse events", eventCount)
			return nil
		case err := <-errChan:
			return fmt.Errorf("error reading events: %w", err)
		case event := <-eventChan:
			if event == nil {
				continue
			}
			
			if event.Type == evdev.EV_REL && (event.Code == evdev.REL_X || event.Code == evdev.REL_Y) {
				eventCount++
				axis := "X"
				if event.Code == evdev.REL_Y {
					axis = "Y"
				}
				logger.Infof("📍 Mouse %s movement: %d (total events: %d)", axis, event.Value, eventCount)
			} else if event.Type == evdev.EV_KEY && event.Code >= evdev.BTN_LEFT && event.Code <= evdev.BTN_TASK {
				logger.Infof("🖱️ Mouse button %d: %d", event.Code, event.Value)
			} else {
				logger.Debugf("Other event: type=%d, code=%d, value=%d", event.Type, event.Code, event.Value)
			}
		}
	}
}

// testWaylandVirtualInput tests Wayland virtual input creation
func testWaylandVirtualInput() (*input.WaylandVirtualInput, error) {
	backend, err := input.NewWaylandVirtualInput()
	if err != nil {
		return nil, fmt.Errorf("failed to create Wayland virtual input: %w", err)
	}

	ctx := context.Background()
	if err := backend.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start Wayland virtual input: %w", err)
	}

	logger.Info("✓ Wayland virtual input backend created and started")

	// Test injection by drawing a small circle
	logger.Info("Drawing test circle with virtual mouse...")
	
	radius := 50.0
	steps := 20

	for i := 0; i <= steps; i++ {
		angle := float64(i) * 2 * math.Pi / float64(steps)
		dx := radius * math.Cos(angle) / float64(steps)
		dy := radius * math.Sin(angle) / float64(steps)

		if err := backend.InjectMouseMove(dx, dy); err != nil {
			logger.Warnf("Failed to inject mouse move: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	logger.Info("✓ Test circle drawn successfully")
	return backend, nil
}

// testFullPipeline tests the complete evdev capture -> Wayland injection pipeline
func testFullPipeline(devicePath string, waylandBackend *input.WaylandVirtualInput) error {
	logger.Info("Testing full capture + injection pipeline...")
	logger.Info("Move your mouse - movements should be mirrored to Wayland virtual input")

	// Create evdev capture backend
	evdevBackend := input.NewEvdevCaptureWithDevices(devicePath, "")
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Set up event forwarding
	var eventCount int
	var mu sync.Mutex
	
	evdevBackend.OnInputEvent(func(event *protocol.InputEvent) {
		mu.Lock()
		eventCount++
		count := eventCount
		mu.Unlock()

		logger.Infof("📤 Event #%d captured, injecting...", count)
		
		// Forward to Wayland virtual input
		switch e := event.Event.(type) {
		case *protocol.InputEvent_MouseMove:
			if err := waylandBackend.InjectMouseMove(e.MouseMove.Dx, e.MouseMove.Dy); err != nil {
				logger.Errorf("Failed to inject mouse move: %v", err)
			}
		case *protocol.InputEvent_MouseButton:
			if err := waylandBackend.InjectMouseButton(e.MouseButton.Button, e.MouseButton.Pressed); err != nil {
				logger.Errorf("Failed to inject mouse button: %v", err)
			}
		case *protocol.InputEvent_MouseScroll:
			if err := waylandBackend.InjectMouseScroll(e.MouseScroll.Dx, e.MouseScroll.Dy); err != nil {
				logger.Errorf("Failed to inject mouse scroll: %v", err)
			}
		}
	})

	// Start evdev capture
	if err := evdevBackend.Start(ctx); err != nil {
		return fmt.Errorf("failed to start evdev capture: %w", err)
	}
	defer evdevBackend.Stop()

	// Set target to enable capture
	if err := evdevBackend.SetTarget("test-client"); err != nil {
		return fmt.Errorf("failed to set evdev target: %w", err)
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("🎯 Pipeline active! Move your mouse to test capture + injection")
	logger.Info("Press Ctrl+C to stop the test")

	select {
	case <-ctx.Done():
		logger.Info("⏰ Test timeout reached")
	case <-sigChan:
		logger.Info("🛑 Test interrupted by user")
	}

	mu.Lock()
	finalCount := eventCount
	mu.Unlock()

	if finalCount == 0 {
		return fmt.Errorf("no events captured during test period")
	}

	logger.Infof("✅ Pipeline test completed - captured and injected %d events", finalCount)
	return nil
}