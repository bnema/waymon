package input

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bnema/waymon/internal/logger"
)

// DeviceMonitor monitors input device changes using inotify or polling
type DeviceMonitor struct {
	ctx        context.Context
	cancel     context.CancelFunc
	inputDir   string
	pollTicker *time.Ticker
}

// DeviceChange represents a device change event
type DeviceChange struct {
	Type   DeviceChangeType
	Path   string
	Device string // device name (e.g., "event0")
}

// DeviceChangeType represents the type of device change
type DeviceChangeType int

const (
	DeviceAdded DeviceChangeType = iota
	DeviceRemoved
)

// NewDeviceMonitor creates a new device monitor
func NewDeviceMonitor() *DeviceMonitor {
	return &DeviceMonitor{
		inputDir: "/dev/input",
	}
}

// Start starts monitoring for device changes
func (dm *DeviceMonitor) Start(ctx context.Context, callback func(DeviceChange)) error {
	dm.ctx, dm.cancel = context.WithCancel(ctx)

	// For now, use simple polling - can be enhanced with inotify later
	dm.pollTicker = time.NewTicker(2 * time.Second)

	go dm.monitorWithPolling(callback)
	
	logger.Debug("Device monitor started with polling")
	return nil
}

// Stop stops the device monitor
func (dm *DeviceMonitor) Stop() {
	if dm.cancel != nil {
		dm.cancel()
	}
	if dm.pollTicker != nil {
		dm.pollTicker.Stop()
	}
	logger.Debug("Device monitor stopped")
}

// monitorWithPolling monitors device changes using polling
func (dm *DeviceMonitor) monitorWithPolling(callback func(DeviceChange)) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Device monitor panic: %v", r)
		}
	}()

	var lastDevices map[string]bool

	// Get initial device list
	currentDevices := dm.getCurrentDevices()
	lastDevices = make(map[string]bool)
	for device := range currentDevices {
		lastDevices[device] = true
	}

	for {
		select {
		case <-dm.ctx.Done():
			return
		case <-dm.pollTicker.C:
			currentDevices := dm.getCurrentDevices()

			// Check for new devices
			for device := range currentDevices {
				if !lastDevices[device] {
					// New device added
					path := filepath.Join(dm.inputDir, device)
					callback(DeviceChange{
						Type:   DeviceAdded,
						Path:   path,
						Device: device,
					})
					logger.Debugf("Device added: %s", device)
				}
			}

			// Check for removed devices
			for device := range lastDevices {
				if !currentDevices[device] {
					// Device removed
					path := filepath.Join(dm.inputDir, device)
					callback(DeviceChange{
						Type:   DeviceRemoved,
						Path:   path,
						Device: device,
					})
					logger.Debugf("Device removed: %s", device)
				}
			}

			lastDevices = currentDevices
		}
	}
}

// getCurrentDevices returns a map of currently available input devices
func (dm *DeviceMonitor) getCurrentDevices() map[string]bool {
	devices := make(map[string]bool)

	entries, err := os.ReadDir(dm.inputDir)
	if err != nil {
		logger.Warnf("Failed to read input directory: %v", err)
		return devices
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "event") {
			devices[entry.Name()] = true
		}
	}

	return devices
}

// ListCurrentDevices returns a list of currently available input device paths
func (dm *DeviceMonitor) ListCurrentDevices() []string {
	var devices []string
	
	entries, err := os.ReadDir(dm.inputDir)
	if err != nil {
		logger.Warnf("Failed to read input directory: %v", err)
		return devices
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "event") {
			devices = append(devices, filepath.Join(dm.inputDir, entry.Name()))
		}
	}

	return devices
}