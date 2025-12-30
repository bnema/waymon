//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bnema/waymon/internal/domain"
	serveruc "github.com/bnema/waymon/internal/usecase/server"
)

// TestServer_HelperFunctions tests display bounds and cursor helper functions.
// These are pure functions that don't require mocks.
func TestServer_HelperFunctions(t *testing.T) {
	// Test with various monitor layouts
	for _, layout := range AllMonitorLayouts() {
		t.Run(MonitorLayoutName(layout), func(t *testing.T) {
			monitors := CreateTestMonitors(layout)
			require.NotEmpty(t, monitors, "should have monitors")

			// Calculate bounds
			bounds := serveruc.CalculateTotalDisplayBounds(monitors)

			// Verify bounds cover all monitors
			for _, m := range monitors {
				assert.LessOrEqual(t, bounds.MinX, float64(m.X), "minX should be <= monitor X")
				assert.LessOrEqual(t, bounds.MinY, float64(m.Y), "minY should be <= monitor Y")
				assert.GreaterOrEqual(t, bounds.MaxX, float64(m.X+m.Width), "maxX should be >= monitor right edge")
				assert.GreaterOrEqual(t, bounds.MaxY, float64(m.Y+m.Height), "maxY should be >= monitor bottom edge")
			}

			// Find primary monitor
			primary := serveruc.FindPrimaryMonitor(monitors)
			require.NotNil(t, primary, "should find primary monitor")

			// Test cursor constraint
			for _, pos := range GetTestPositions(monitors) {
				constrainedX, constrainedY := serveruc.ConstrainCursorPosition(float64(pos.X), float64(pos.Y), bounds)
				assert.GreaterOrEqual(t, constrainedX, bounds.MinX, "constrained X should be >= minX")
				assert.LessOrEqual(t, constrainedX, bounds.MaxX, "constrained X should be <= maxX")
				assert.GreaterOrEqual(t, constrainedY, bounds.MinY, "constrained Y should be >= minY")
				assert.LessOrEqual(t, constrainedY, bounds.MaxY, "constrained Y should be <= maxY")
			}
		})
	}
}

// TestServer_MonitorCenterCalculation tests monitor center calculation.
func TestServer_MonitorCenterCalculation(t *testing.T) {
	tests := []struct {
		name      string
		monitor   domain.Monitor
		expectedX float64
		expectedY float64
	}{
		{
			name:      "standard_1080p",
			monitor:   domain.Monitor{X: 0, Y: 0, Width: 1920, Height: 1080},
			expectedX: 960,
			expectedY: 540,
		},
		{
			name:      "offset_monitor",
			monitor:   domain.Monitor{X: 1920, Y: 200, Width: 2560, Height: 1440},
			expectedX: 1920 + 1280,
			expectedY: 200 + 720,
		},
		{
			name:      "4k_monitor",
			monitor:   domain.Monitor{X: 0, Y: 0, Width: 3840, Height: 2160},
			expectedX: 1920,
			expectedY: 1080,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			centerX, centerY := serveruc.CalculateMonitorCenter(&tt.monitor)
			assert.Equal(t, tt.expectedX, centerX, "center X should match")
			assert.Equal(t, tt.expectedY, centerY, "center Y should match")
		})
	}
}

// TestServer_IsWithinBounds tests bounds checking.
func TestServer_IsWithinBounds(t *testing.T) {
	bounds := domain.DisplayBounds{
		MinX: 0,
		MinY: 0,
		MaxX: 1920,
		MaxY: 1080,
	}

	tests := []struct {
		name     string
		x, y     float64
		expected bool
	}{
		{"center", 960, 540, true},
		{"top_left", 0, 0, true},
		{"bottom_right_inside", 1919, 1079, true},
		{"at_right_edge", 1920, 540, true}, // Edge is inclusive per implementation
		{"at_bottom_edge", 960, 1080, true},
		{"outside_left", -1, 540, false},
		{"outside_top", 960, -1, false},
		{"far_outside", 5000, 5000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serveruc.IsWithinBounds(tt.x, tt.y, bounds)
			assert.Equal(t, tt.expected, result, "bounds check should match")
		})
	}
}

// TestServer_FindMonitorFunctions tests monitor finder functions.
func TestServer_FindMonitorFunctions(t *testing.T) {
	t.Run("FindPrimaryMonitor", func(t *testing.T) {
		tests := []struct {
			name      string
			monitors  []domain.Monitor
			expectID  string
			expectNil bool
		}{
			{
				name: "finds_primary",
				monitors: []domain.Monitor{
					{ID: "1", Primary: false},
					{ID: "2", Primary: true},
					{ID: "3", Primary: false},
				},
				expectID: "2",
			},
			{
				name: "returns_nil_if_no_primary",
				monitors: []domain.Monitor{
					{ID: "1", Primary: false},
					{ID: "2", Primary: false},
				},
				expectNil: true,
			},
			{
				name:      "returns_nil_for_empty",
				monitors:  []domain.Monitor{},
				expectNil: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := serveruc.FindPrimaryMonitor(tt.monitors)
				if tt.expectNil {
					assert.Nil(t, result)
				} else {
					require.NotNil(t, result)
					assert.Equal(t, tt.expectID, result.ID)
				}
			})
		}
	})

	t.Run("FindMonitorAtOrigin", func(t *testing.T) {
		tests := []struct {
			name      string
			monitors  []domain.Monitor
			expectID  string
			expectNil bool
		}{
			{
				name: "finds_origin_monitor",
				monitors: []domain.Monitor{
					{ID: "1", X: 1920, Y: 0},
					{ID: "2", X: 0, Y: 0},
					{ID: "3", X: 0, Y: 1080},
				},
				expectID: "2",
			},
			{
				name: "returns_nil_if_none_at_origin",
				monitors: []domain.Monitor{
					{ID: "1", X: 100, Y: 100},
				},
				expectNil: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := serveruc.FindMonitorAtOrigin(tt.monitors)
				if tt.expectNil {
					assert.Nil(t, result)
				} else {
					require.NotNil(t, result)
					assert.Equal(t, tt.expectID, result.ID)
				}
			})
		}
	})

	t.Run("FindMainMonitor", func(t *testing.T) {
		tests := []struct {
			name      string
			monitors  []domain.Monitor
			expectID  string
			expectNil bool
		}{
			{
				name: "prefers_primary",
				monitors: []domain.Monitor{
					{ID: "1", X: 0, Y: 0, Primary: false},
					{ID: "2", X: 1920, Y: 0, Primary: true},
				},
				expectID: "2",
			},
			{
				name: "falls_back_to_origin",
				monitors: []domain.Monitor{
					{ID: "1", X: 1920, Y: 0, Primary: false},
					{ID: "2", X: 0, Y: 0, Primary: false},
				},
				expectID: "2",
			},
			{
				name: "falls_back_to_first",
				monitors: []domain.Monitor{
					{ID: "1", X: 100, Y: 100, Primary: false},
					{ID: "2", X: 200, Y: 200, Primary: false},
				},
				expectID: "1",
			},
			{
				name:      "returns_nil_for_empty",
				monitors:  []domain.Monitor{},
				expectNil: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := serveruc.FindMainMonitor(tt.monitors)
				if tt.expectNil {
					assert.Nil(t, result)
				} else {
					require.NotNil(t, result)
					assert.Equal(t, tt.expectID, result.ID)
				}
			})
		}
	})
}

// TestServer_BoundsCalculation tests comprehensive bounds calculations.
func TestServer_BoundsCalculation(t *testing.T) {
	tests := []struct {
		name     string
		monitors []domain.Monitor
		expected domain.DisplayBounds
	}{
		{
			name:     "empty_monitors_returns_default",
			monitors: []domain.Monitor{},
			expected: domain.DisplayBounds{MinX: 0, MinY: 0, MaxX: 1920, MaxY: 1080},
		},
		{
			name: "single_monitor",
			monitors: []domain.Monitor{
				{X: 0, Y: 0, Width: 1920, Height: 1080},
			},
			expected: domain.DisplayBounds{MinX: 0, MinY: 0, MaxX: 1920, MaxY: 1080},
		},
		{
			name: "horizontal_monitors",
			monitors: []domain.Monitor{
				{X: 0, Y: 0, Width: 1920, Height: 1080},
				{X: 1920, Y: 0, Width: 1920, Height: 1080},
			},
			expected: domain.DisplayBounds{MinX: 0, MinY: 0, MaxX: 3840, MaxY: 1080},
		},
		{
			name: "vertical_monitors",
			monitors: []domain.Monitor{
				{X: 0, Y: 0, Width: 1920, Height: 1080},
				{X: 0, Y: 1080, Width: 1920, Height: 1080},
			},
			expected: domain.DisplayBounds{MinX: 0, MinY: 0, MaxX: 1920, MaxY: 2160},
		},
		{
			name: "offset_monitors",
			monitors: []domain.Monitor{
				{X: 0, Y: 200, Width: 1920, Height: 1080},
				{X: 1920, Y: 0, Width: 2560, Height: 1440},
			},
			expected: domain.DisplayBounds{MinX: 0, MinY: 0, MaxX: 4480, MaxY: 1440},
		},
		{
			name: "negative_coords",
			monitors: []domain.Monitor{
				{X: -1920, Y: 0, Width: 1920, Height: 1080},
				{X: 0, Y: 0, Width: 1920, Height: 1080},
			},
			expected: domain.DisplayBounds{MinX: -1920, MinY: 0, MaxX: 1920, MaxY: 1080},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serveruc.CalculateTotalDisplayBounds(tt.monitors)
			assert.Equal(t, tt.expected.MinX, result.MinX, "MinX should match")
			assert.Equal(t, tt.expected.MinY, result.MinY, "MinY should match")
			assert.Equal(t, tt.expected.MaxX, result.MaxX, "MaxX should match")
			assert.Equal(t, tt.expected.MaxY, result.MaxY, "MaxY should match")
		})
	}
}

// TestServer_CursorConstraint tests cursor constraint edge cases.
func TestServer_CursorConstraint(t *testing.T) {
	bounds := domain.DisplayBounds{MinX: 0, MinY: 0, MaxX: 1920, MaxY: 1080}

	tests := []struct {
		name      string
		inputX    float64
		inputY    float64
		expectedX float64
		expectedY float64
	}{
		{"center_unchanged", 960, 540, 960, 540},
		{"constrain_left", -100, 540, 0, 540},
		{"constrain_right", 2000, 540, 1920, 540},
		{"constrain_top", 960, -50, 960, 0},
		{"constrain_bottom", 960, 1200, 960, 1080},
		{"constrain_corner", -100, -100, 0, 0},
		{"at_edge", 0, 0, 0, 0},
		{"at_max", 1920, 1080, 1920, 1080},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultX, resultY := serveruc.ConstrainCursorPosition(tt.inputX, tt.inputY, bounds)
			assert.Equal(t, tt.expectedX, resultX, "X should match")
			assert.Equal(t, tt.expectedY, resultY, "Y should match")
		})
	}
}
