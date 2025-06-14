package display

import (
	"testing"
)

func TestPrimaryMonitorDetermination(t *testing.T) {
	tests := []struct {
		name           string
		monitors       []*Monitor
		expectedPrimary string
		description    string
	}{
		{
			name: "monitor at 0,0 should be primary",
			monitors: []*Monitor{
				{ID: "0", Name: "Monitor1", X: -1920, Y: 0, Width: 1920, Height: 1080, Primary: false},
				{ID: "1", Name: "Monitor2", X: 0, Y: 0, Width: 1920, Height: 1080, Primary: false},
			},
			expectedPrimary: "1",
			description: "When one monitor is at position (0,0), it should be marked as primary",
		},
		{
			name: "first monitor fallback when no monitor at 0,0",
			monitors: []*Monitor{
				{ID: "0", Name: "Monitor1", X: -1920, Y: 0, Width: 1920, Height: 1080, Primary: false},
				{ID: "1", Name: "Monitor2", X: 1920, Y: 0, Width: 1920, Height: 1080, Primary: false},
			},
			expectedPrimary: "0",
			description: "When no monitor is at (0,0), first monitor should be primary",
		},
		{
			name: "single monitor at 0,0",
			monitors: []*Monitor{
				{ID: "0", Name: "Monitor1", X: 0, Y: 0, Width: 1920, Height: 1080, Primary: false},
			},
			expectedPrimary: "0",
			description: "Single monitor at (0,0) should be primary",
		},
		{
			name: "multiple monitors, one at 0,0",
			monitors: []*Monitor{
				{ID: "0", Name: "Monitor1", X: -3840, Y: 0, Width: 3840, Height: 2160, Primary: false},
				{ID: "1", Name: "Monitor2", X: 0, Y: 0, Width: 3840, Height: 2160, Primary: false},
				{ID: "2", Name: "Monitor3", X: 3840, Y: 0, Width: 1920, Height: 1080, Primary: false},
			},
			expectedPrimary: "1",
			description: "With multiple monitors, the one at (0,0) should be primary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply the primary monitor determination logic
			determinePrimaryMonitor(tt.monitors)
			
			// Find which monitor is marked as primary
			var primaryID string
			primaryCount := 0
			for _, monitor := range tt.monitors {
				if monitor.Primary {
					primaryID = monitor.ID
					primaryCount++
				}
			}
			
			// Verify exactly one monitor is marked as primary
			if primaryCount != 1 {
				t.Errorf("Expected exactly 1 primary monitor, got %d", primaryCount)
			}
			
			// Verify the correct monitor is marked as primary
			if primaryID != tt.expectedPrimary {
				t.Errorf("Expected monitor %s to be primary, got %s", tt.expectedPrimary, primaryID)
			}
		})
	}
}