package client

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bnema/waymon/internal/logger"
)

// EmergencyRelease provides multiple mechanisms for emergency client shutdown
type EmergencyRelease struct {
	receiver        *InputReceiver
	activityTimeout time.Duration
	lastActivity    time.Time
	mu              sync.Mutex
	onEmergency     func(reason string)
}

// NewEmergencyRelease creates a new emergency release handler for client
func NewEmergencyRelease(receiver *InputReceiver, onEmergency func(reason string)) *EmergencyRelease {
	return &EmergencyRelease{
		receiver:        receiver,
		activityTimeout: 60 * time.Second, // Auto-disconnect after 60s of inactivity
		lastActivity:    time.Now(),
		onEmergency:     onEmergency,
	}
}

// Start begins monitoring for emergency release conditions
func (er *EmergencyRelease) Start(ctx context.Context) {
	// 1. Signal handler for SIGUSR1
	go er.handleSignals(ctx)

	// 2. Activity timeout monitor
	go er.monitorActivity(ctx)

	// 3. File-based trigger (check for /tmp/waymon-client-release)
	go er.monitorFileTriger(ctx)

	logger.Info("[CLIENT EMERGENCY] Emergency release mechanisms activated")
}

// UpdateActivity updates the last activity timestamp
func (er *EmergencyRelease) UpdateActivity() {
	er.mu.Lock()
	defer er.mu.Unlock()
	er.lastActivity = time.Now()
}

// handleSignals listens for SIGUSR1 to trigger emergency release
func (er *EmergencyRelease) handleSignals(ctx context.Context) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGUSR1)
	defer signal.Stop(sigChan)

	for {
		select {
		case <-sigChan:
			logger.Warn("[CLIENT EMERGENCY] SIGUSR1 received - triggering emergency release")
			er.triggerRelease("signal")
		case <-ctx.Done():
			return
		}
	}
}

// monitorActivity checks for input activity timeout
func (er *EmergencyRelease) monitorActivity(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !er.receiver.IsConnected() {
				continue // No timeout when not connected
			}

			er.mu.Lock()
			lastActivity := er.lastActivity
			er.mu.Unlock()

			if time.Since(lastActivity) > er.activityTimeout {
				logger.Warnf("[CLIENT EMERGENCY] No activity for %v - triggering emergency release", er.activityTimeout)
				er.triggerRelease("timeout")
			}
		case <-ctx.Done():
			return
		}
	}
}

// monitorFileTriger checks for presence of /tmp/waymon-client-release file
func (er *EmergencyRelease) monitorFileTriger(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	triggerFile := "/tmp/waymon-client-release"

	for {
		select {
		case <-ticker.C:
			if _, err := os.Stat(triggerFile); err == nil {
				logger.Warn("[CLIENT EMERGENCY] Release file detected - triggering emergency release")
				os.Remove(triggerFile) // Clean up
				er.triggerRelease("file")
			}
		case <-ctx.Done():
			return
		}
	}
}

// triggerRelease performs the emergency release
func (er *EmergencyRelease) triggerRelease(reason string) {
	logger.Warnf("[CLIENT EMERGENCY] Emergency release triggered (reason: %s)", reason)

	// Call the emergency callback if provided
	// This should be called first so the UI can respond immediately
	if er.onEmergency != nil {
		er.onEmergency(reason)
	}

	// Disconnect from server if connected
	// Note: The receiver's Disconnect method will be called by the client command
	// after the emergency callback triggers shutdown

	// Reset activity timer
	er.lastActivity = time.Now()
}