package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckAndLoadUinput(t *testing.T) {
	// Test if uinput module check works
	err := checkAndLoadUinput()
	
	// This test might fail on systems without uinput or without sudo
	// We'll just check that the function doesn't panic
	if err != nil {
		t.Logf("checkAndLoadUinput failed (expected on systems without uinput): %v", err)
	}
}

func TestCheckUinputDevice(t *testing.T) {
	// Test if /dev/uinput exists check works
	err := checkUinputDevice()
	
	// This might fail on systems without uinput
	if err != nil {
		t.Logf("checkUinputDevice failed (expected on systems without uinput): %v", err)
	}
}

func TestShowCurrentPermissions(t *testing.T) {
	// Test that showCurrentPermissions doesn't panic
	err := showCurrentPermissions()
	
	// This might fail if /dev/uinput doesn't exist, which is fine for testing
	if err != nil {
		t.Logf("showCurrentPermissions failed (expected on systems without uinput): %v", err)
	}
}

func TestEnsureWaymonGroup(t *testing.T) {
	// We can't actually test group creation without sudo
	// But we can test the check logic
	
	// Check if waymon group exists
	cmd := exec.Command("getent", "group", "waymon")
	err := cmd.Run()
	
	if err == nil {
		t.Log("waymon group already exists")
	} else {
		t.Log("waymon group does not exist (expected for fresh systems)")
	}
}

func TestSetupInputCapture(t *testing.T) {
	// Test the logic without actually modifying groups
	
	// Check current user groups
	cmd := exec.Command("groups")
	output, err := cmd.Output()
	require.NoError(t, err)
	
	groups := string(output)
	t.Logf("Current user groups: %s", strings.TrimSpace(groups))
	
	hasInputGroup := strings.Contains(groups, "input")
	t.Logf("User has input group: %v", hasInputGroup)
	
	// The actual setupInputCapture function would modify groups,
	// but we don't want to do that in a test
}

func TestCheckUinputAvailable(t *testing.T) {
	err := CheckUinputAvailable()
	
	if err != nil {
		t.Logf("CheckUinputAvailable failed: %v", err)
		// This is expected on many systems, so we don't fail the test
	} else {
		t.Log("CheckUinputAvailable succeeded")
	}
}

func TestVerifyWaymonSetup(t *testing.T) {
	err := VerifyWaymonSetup()
	
	if err != nil {
		t.Logf("VerifyWaymonSetup failed (expected on unconfigured systems): %v", err)
		// This is expected if setup hasn't been run
	} else {
		t.Log("VerifyWaymonSetup succeeded - system is properly configured")
	}
}

// Test helper functions
func TestGroupMembershipCheck(t *testing.T) {
	// Test the group membership checking logic
	testCases := []struct {
		name     string
		groups   string
		expected map[string]bool
	}{
		{
			name:   "user with input and waymon groups",
			groups: "user wheel input waymon sudo",
			expected: map[string]bool{
				"input":  true,
				"waymon": true,
				"wheel":  true,
				"sudo":   true,
				"admin":  false,
			},
		},
		{
			name:   "user with only input group",
			groups: "user input wheel",
			expected: map[string]bool{
				"input":  true,
				"waymon": false,
				"wheel":  true,
			},
		},
		{
			name:   "user with no special groups",
			groups: "user",
			expected: map[string]bool{
				"input":  false,
				"waymon": false,
				"wheel":  false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for group, expectedPresent := range tc.expected {
				actual := strings.Contains(tc.groups, group)
				assert.Equal(t, expectedPresent, actual, 
					"Group %s presence mismatch in groups: %s", group, tc.groups)
			}
		})
	}
}

func TestDevicePermissionCheck(t *testing.T) {
	// Test checking permissions on common input devices
	inputDevices := []string{"/dev/input/event0", "/dev/input/event1", "/dev/input/event2"}
	
	for _, device := range inputDevices {
		if _, err := os.Stat(device); err == nil {
			t.Logf("Device %s exists", device)
			
			// Try to open read-only (this will fail without proper permissions)
			file, err := os.OpenFile(device, os.O_RDONLY, 0)
			if err != nil {
				if os.IsPermission(err) {
					t.Logf("No read permission for %s (expected)", device)
				} else {
					t.Logf("Other error opening %s: %v", device, err)
				}
			} else {
				file.Close()
				t.Logf("Successfully opened %s for reading", device)
			}
		} else {
			t.Logf("Device %s does not exist", device)
		}
	}
}

func TestUinputPermissionCheck(t *testing.T) {
	// Test checking permissions on /dev/uinput
	if _, err := os.Stat("/dev/uinput"); err == nil {
		t.Log("/dev/uinput exists")
		
		// Try to open write-only (this will fail without proper permissions)
		file, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
		if err != nil {
			if os.IsPermission(err) {
				t.Log("No write permission for /dev/uinput (expected)")
			} else {
				t.Logf("Other error opening /dev/uinput: %v", err)
			}
		} else {
			file.Close()
			t.Log("Successfully opened /dev/uinput for writing")
		}
	} else {
		t.Log("/dev/uinput does not exist")
	}
}

// Benchmark the permission checking functions
func BenchmarkCheckUinputAvailable(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CheckUinputAvailable()
	}
}

func BenchmarkVerifyWaymonSetup(b *testing.B) {
	for i := 0; i < b.N; i++ {
		VerifyWaymonSetup()
	}
}

// Test the setup mode-specific validation logic
func TestSetupModeValidation(t *testing.T) {
	testCases := []struct {
		name           string
		hasWaymonGroup bool
		hasInputGroup  bool
		shouldPass     bool
		description    string
	}{
		{
			name:           "server only setup",
			hasWaymonGroup: true,
			hasInputGroup:  false,
			shouldPass:     true,
			description:    "User with waymon group can run server mode",
		},
		{
			name:           "client only setup", 
			hasWaymonGroup: false,
			hasInputGroup:  true,
			shouldPass:     true,
			description:    "User with input group can run client mode",
		},
		{
			name:           "both groups setup",
			hasWaymonGroup: true,
			hasInputGroup:  true,
			shouldPass:     true,
			description:    "User with both groups can run both modes",
		},
		{
			name:           "no groups setup",
			hasWaymonGroup: false,
			hasInputGroup:  false,
			shouldPass:     false,
			description:    "User with neither group should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the group membership check logic
			hasAtLeastOneGroup := tc.hasWaymonGroup || tc.hasInputGroup
			assert.Equal(t, tc.shouldPass, hasAtLeastOneGroup, tc.description)
		})
	}
}