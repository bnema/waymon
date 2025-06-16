package input

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PersistentDeviceInfo contains device information that persists across reconnections
type PersistentDeviceInfo struct {
	// Name is the human-readable device name
	Name string `json:"name" mapstructure:"name"`
	
	// ByIDPath is the persistent path in /dev/input/by-id/ (if available)
	ByIDPath string `json:"by_id_path" mapstructure:"by_id_path"`
	
	// ByPathPath is the persistent path in /dev/input/by-path/ (fallback)
	ByPathPath string `json:"by_path_path" mapstructure:"by_path_path"`
	
	// VendorID and ProductID for USB devices
	VendorID  string `json:"vendor_id,omitempty" mapstructure:"vendor_id"`
	ProductID string `json:"product_id,omitempty" mapstructure:"product_id"`
	
	// Physical location for built-in devices
	Phys string `json:"phys,omitempty" mapstructure:"phys"`
}

// ResolveToPersistentPath finds the persistent path for a given event device
func ResolveToPersistentPath(eventPath string) (*PersistentDeviceInfo, error) {
	info := &PersistentDeviceInfo{}
	
	// Get the event file name (e.g., "event13")
	eventName := filepath.Base(eventPath)
	
	// Try to find in /dev/input/by-id/ first (most reliable)
	byIDDir := "/dev/input/by-id"
	if entries, err := os.ReadDir(byIDDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.Contains(entry.Name(), "event") {
				symlinkPath := filepath.Join(byIDDir, entry.Name())
				if target, err := os.Readlink(symlinkPath); err == nil {
					// Check if this symlink points to our event device
					if filepath.Base(target) == eventName {
						info.ByIDPath = symlinkPath
						info.Name = cleanDeviceName(entry.Name())
						break
					}
				}
			}
		}
	}
	
	// Try /dev/input/by-path/ as fallback
	if info.ByIDPath == "" {
		byPathDir := "/dev/input/by-path"
		if entries, err := os.ReadDir(byPathDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.Contains(entry.Name(), "event") {
					symlinkPath := filepath.Join(byPathDir, entry.Name())
					if target, err := os.Readlink(symlinkPath); err == nil {
						if filepath.Base(target) == eventName {
							info.ByPathPath = symlinkPath
							if info.Name == "" {
								info.Name = cleanDeviceName(entry.Name())
							}
							break
						}
					}
				}
			}
		}
	}
	
	// Read additional info from sysfs
	sysPath := fmt.Sprintf("/sys/class/input/%s/device", eventName)
	
	// Try to get name from sysfs
	if info.Name == "" {
		if data, err := os.ReadFile(filepath.Join(sysPath, "name")); err == nil {
			info.Name = strings.TrimSpace(string(data))
		}
	}
	
	// Get physical location
	if data, err := os.ReadFile(filepath.Join(sysPath, "phys")); err == nil {
		info.Phys = strings.TrimSpace(string(data))
	}
	
	// For USB devices, try to get vendor/product IDs
	if data, err := os.ReadFile(filepath.Join(sysPath, "id/vendor")); err == nil {
		info.VendorID = strings.TrimSpace(string(data))
	}
	if data, err := os.ReadFile(filepath.Join(sysPath, "id/product")); err == nil {
		info.ProductID = strings.TrimSpace(string(data))
	}
	
	if info.ByIDPath == "" && info.ByPathPath == "" && info.Phys == "" {
		return nil, fmt.Errorf("could not find persistent identifier for %s", eventPath)
	}
	
	return info, nil
}

// ResolveToEventPath resolves a PersistentDeviceInfo back to the current event path
func (p *PersistentDeviceInfo) ResolveToEventPath() (string, error) {
	// Try by-id path first
	if p.ByIDPath != "" {
		if target, err := os.Readlink(p.ByIDPath); err == nil {
			// Convert relative path to absolute
			return filepath.Clean(filepath.Join("/dev/input", filepath.Base(target))), nil
		}
	}
	
	// Try by-path as fallback
	if p.ByPathPath != "" {
		if target, err := os.Readlink(p.ByPathPath); err == nil {
			return filepath.Clean(filepath.Join("/dev/input", filepath.Base(target))), nil
		}
	}
	
	// Last resort: search by physical location
	if p.Phys != "" {
		inputDir := "/sys/class/input"
		entries, err := os.ReadDir(inputDir)
		if err != nil {
			return "", err
		}
		
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "event") {
				physPath := filepath.Join(inputDir, entry.Name(), "device", "phys")
				if data, err := os.ReadFile(physPath); err == nil {
					if strings.TrimSpace(string(data)) == p.Phys {
						return filepath.Join("/dev/input", entry.Name()), nil
					}
				}
			}
		}
	}
	
	return "", fmt.Errorf("device not found: %s", p.Name)
}

// cleanDeviceName removes common prefixes/suffixes for cleaner display
func cleanDeviceName(name string) string {
	// Remove common prefixes
	name = strings.TrimPrefix(name, "usb-")
	
	// Remove event type suffixes
	name = strings.TrimSuffix(name, "-event-kbd")
	name = strings.TrimSuffix(name, "-event-mouse")
	name = strings.TrimSuffix(name, "-event-if01")
	name = strings.TrimSuffix(name, "-event-if02")
	name = strings.TrimSuffix(name, "-if01")
	name = strings.TrimSuffix(name, "-if02")
	
	return name
}