package input

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/network"
)

// EdgeBarrier implements edge detection and mouse capture for Wayland
// Since Wayland doesn't allow global mouse capture, we use a different approach:
// 1. Create an invisible barrier at screen edges
// 2. When mouse hits barrier, we "capture" by warping cursor back
// 3. Send relative movements to remote host
type EdgeBarrier struct {
	display      *display.Display
	client       network.Client
	edgeDetector *EdgeDetector
	
	mu           sync.Mutex
	active       bool
	capturing    bool
	barrier      Handler // Local input handler for cursor warping
	
	// Track position and accumulated movement
	lastX, lastY int32
	accumX, accumY float64
	
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// NewEdgeBarrier creates a new edge barrier system
func NewEdgeBarrier(display *display.Display, client network.Client, edgeDetector *EdgeDetector) (*EdgeBarrier, error) {
	// Create a local input handler for cursor warping
	handler, err := NewHandler()
	if err != nil {
		return nil, fmt.Errorf("failed to create input handler: %w", err)
	}
	
	return &EdgeBarrier{
		display:      display,
		client:       client,
		edgeDetector: edgeDetector,
		barrier:      handler,
	}, nil
}

// Start begins the edge barrier system
func (b *EdgeBarrier) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if b.active {
		return nil
	}
	
	b.active = true
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	
	// Start barrier monitoring
	b.wg.Add(1)
	go b.barrierLoop(ctx)
	
	logger.Info("Edge barrier system started")
	return nil
}

// Stop stops the edge barrier system
func (b *EdgeBarrier) Stop() {
	b.mu.Lock()
	if !b.active {
		b.mu.Unlock()
		return
	}
	
	b.active = false
	if b.cancel != nil {
		b.cancel()
	}
	b.mu.Unlock()
	
	b.wg.Wait()
	
	if b.barrier != nil {
		b.barrier.Close()
	}
	
	logger.Info("Edge barrier system stopped")
}

// barrierLoop monitors for edge hits and manages capture
func (b *EdgeBarrier) barrierLoop(ctx context.Context) {
	defer b.wg.Done()
	
	// We need a way to detect when mouse is at edge
	// Since Wayland doesn't give us cursor position, we have to be creative
	
	// Option 1: Use uinput to create a virtual input device that can track position
	// Option 2: Create invisible windows at edges (requires compositor support)
	// Option 3: Use libinput or evdev to track mouse movement directly
	
	ticker := time.NewTicker(10 * time.Millisecond) // 100Hz polling
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.checkBarrier()
		}
	}
}

// checkBarrier checks if we should activate the barrier
func (b *EdgeBarrier) checkBarrier() {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if !b.active {
		return
	}
	
	// This is where we'd implement the actual barrier logic
	// For now, let's rely on the edge detector's capture state
	if b.edgeDetector.IsCapturing() && !b.capturing {
		b.startCapture()
	} else if !b.edgeDetector.IsCapturing() && b.capturing {
		b.stopCapture()
	}
}

// startCapture begins capturing mouse movements
func (b *EdgeBarrier) startCapture() {
	b.capturing = true
	b.accumX = 0
	b.accumY = 0
	
	logger.Info("Edge barrier activated - mouse movements are now captured")
	
	// TODO: Implement actual cursor confinement
	// This would involve creating an invisible barrier or window at the edge
}

// stopCapture stops capturing mouse movements
func (b *EdgeBarrier) stopCapture() {
	b.capturing = false
	
	logger.Info("Edge barrier deactivated - mouse control returned to local")
}

// HandleMouseMovement processes mouse movement when barrier is active
// This would be called by the actual input capture mechanism
func (b *EdgeBarrier) HandleMouseMovement(dx, dy float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if !b.capturing {
		return
	}
	
	// Accumulate fractional movements
	b.accumX += dx
	b.accumY += dy
	
	// Send integer movements
	intX := int32(b.accumX)
	intY := int32(b.accumY)
	
	if intX != 0 || intY != 0 {
		// Update accumulator
		b.accumX -= float64(intX)
		b.accumY -= float64(intY)
		
		// Forward to edge detector which will send to remote
		b.edgeDetector.HandleMouseMove(intX, intY)
		
		// TODO: Warp cursor back to prevent it from leaving the edge
		// This requires uinput absolute positioning support
	}
}