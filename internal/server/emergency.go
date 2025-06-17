package server

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bnema/waymon/internal/logger"
)

// EmergencyRelease provides multiple mechanisms for emergency control release
type EmergencyRelease struct {
	manager          *ClientManager
	activityTimeout  time.Duration
	lastActivity     time.Time
	stopChan         chan struct{}
	stopOnce         sync.Once
}

// NewEmergencyRelease creates a new emergency release handler
func NewEmergencyRelease(manager *ClientManager) *EmergencyRelease {
	return &EmergencyRelease{
		manager:         manager,
		activityTimeout: 30 * time.Second, // Auto-release after 30s of inactivity
		lastActivity:    time.Now(),
		stopChan:        make(chan struct{}),
	}
}

// Start begins monitoring for emergency release conditions
func (er *EmergencyRelease) Start() {
	// 1. Signal handler for SIGUSR1
	go er.handleSignals()
	
	// 2. Activity timeout monitor
	go er.monitorActivity()
	
	// 3. File-based trigger (check for /tmp/waymon-release)
	go er.monitorFileTriger()
	
	logger.Info("[EMERGENCY] Emergency release mechanisms activated")
}

// Stop stops all emergency monitoring
func (er *EmergencyRelease) Stop() {
	er.stopOnce.Do(func() {
		close(er.stopChan)
	})
}

// UpdateActivity updates the last activity timestamp
func (er *EmergencyRelease) UpdateActivity() {
	er.lastActivity = time.Now()
}

// handleSignals listens for SIGUSR1 to trigger emergency release
func (er *EmergencyRelease) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGUSR1)
	
	for {
		select {
		case <-sigChan:
			logger.Warn("[EMERGENCY] SIGUSR1 received - triggering emergency release")
			er.triggerRelease("signal")
		case <-er.stopChan:
			return
		}
	}
}

// monitorActivity checks for input activity timeout
func (er *EmergencyRelease) monitorActivity() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if er.manager.IsControllingLocal() {
				continue // No timeout when controlling local
			}
			
			if time.Since(er.lastActivity) > er.activityTimeout {
				logger.Warnf("[EMERGENCY] No activity for %v - triggering emergency release", er.activityTimeout)
				er.triggerRelease("timeout")
			}
		case <-er.stopChan:
			return
		}
	}
}

// monitorFileTriger checks for presence of /tmp/waymon-release file
func (er *EmergencyRelease) monitorFileTriger() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	triggerFile := "/tmp/waymon-release"
	
	for {
		select {
		case <-ticker.C:
			if _, err := os.Stat(triggerFile); err == nil {
				logger.Warn("[EMERGENCY] Release file detected - triggering emergency release")
				os.Remove(triggerFile) // Clean up
				er.triggerRelease("file")
			}
		case <-er.stopChan:
			return
		}
	}
}

// triggerRelease performs the emergency release
func (er *EmergencyRelease) triggerRelease(reason string) {
	logger.Warnf("[EMERGENCY] Emergency release triggered (reason: %s)", reason)
	
	// Mark emergency release to start cooldown period
	er.manager.MarkEmergencyRelease()
	
	// Force switch to local control
	if err := er.manager.SwitchToLocal(); err != nil {
		logger.Errorf("[EMERGENCY] Failed to switch to local: %v", err)
		
		// Force release at input backend level
		if backend := er.manager.inputBackend; backend != nil {
			if err := backend.SetTarget(""); err != nil {
				logger.Errorf("[EMERGENCY] Failed to release input backend: %v", err)
			}
		}
	}
	
	// Reset activity timer
	er.lastActivity = time.Now()
}