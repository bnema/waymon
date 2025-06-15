package server

import (
	"testing"

	"github.com/bnema/waymon/internal/protocol"
)

func TestCalculateTotalDisplayBounds(t *testing.T) {
	cm := &ClientManager{}

	tests := []struct {
		name     string
		monitors []*protocol.Monitor
		expected rect
	}{
		{
			name:     "empty monitors",
			monitors: []*protocol.Monitor{},
			expected: rect{minX: 0, minY: 0, maxX: 1920, maxY: 1080},
		},
		{
			name: "single monitor",
			monitors: []*protocol.Monitor{
				{X: 0, Y: 0, Width: 1920, Height: 1080},
			},
			expected: rect{minX: 0, minY: 0, maxX: 1920, maxY: 1080},
		},
		{
			name: "dual monitors horizontal",
			monitors: []*protocol.Monitor{
				{X: 0, Y: 0, Width: 1920, Height: 1080},
				{X: 1920, Y: 0, Width: 1920, Height: 1080},
			},
			expected: rect{minX: 0, minY: 0, maxX: 3840, maxY: 1080},
		},
		{
			name: "dual monitors vertical",
			monitors: []*protocol.Monitor{
				{X: 0, Y: 0, Width: 1920, Height: 1080},
				{X: 0, Y: 1080, Width: 1920, Height: 1080},
			},
			expected: rect{minX: 0, minY: 0, maxX: 1920, maxY: 2160},
		},
		{
			name: "complex multi-monitor setup",
			monitors: []*protocol.Monitor{
				{X: 0, Y: 0, Width: 1920, Height: 1080},
				{X: 1920, Y: -200, Width: 2560, Height: 1440},
				{X: -1920, Y: 0, Width: 1920, Height: 1080},
			},
			expected: rect{minX: -1920, minY: -200, maxX: 4480, maxY: 1240},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cm.calculateTotalDisplayBounds(tt.monitors)
			if got != tt.expected {
				t.Errorf("calculateTotalDisplayBounds() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestConstrainCursorPosition(t *testing.T) {
	cm := &ClientManager{}

	bounds := rect{minX: 0, minY: 0, maxX: 1920, maxY: 1080}

	tests := []struct {
		name      string
		x, y      float64
		expectedX float64
		expectedY float64
	}{
		{
			name:      "within bounds",
			x:         500,
			y:         500,
			expectedX: 500,
			expectedY: 500,
		},
		{
			name:      "left edge constraint",
			x:         -100,
			y:         500,
			expectedX: 0,
			expectedY: 500,
		},
		{
			name:      "right edge constraint",
			x:         2000,
			y:         500,
			expectedX: 1920,
			expectedY: 500,
		},
		{
			name:      "top edge constraint",
			x:         500,
			y:         -100,
			expectedX: 500,
			expectedY: 0,
		},
		{
			name:      "bottom edge constraint",
			x:         500,
			y:         1200,
			expectedX: 500,
			expectedY: 1080,
		},
		{
			name:      "corner constraint",
			x:         2000,
			y:         1200,
			expectedX: 1920,
			expectedY: 1080,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotX, gotY := cm.constrainCursorPosition(tt.x, tt.y, bounds)
			if gotX != tt.expectedX || gotY != tt.expectedY {
				t.Errorf("constrainCursorPosition(%v, %v) = (%v, %v), want (%v, %v)",
					tt.x, tt.y, gotX, gotY, tt.expectedX, tt.expectedY)
			}
		})
	}
}

func TestFindMainMonitor(t *testing.T) {
	cm := &ClientManager{}

	tests := []struct {
		name          string
		monitors      []*protocol.Monitor
		expectedIndex int // -1 if nil expected
	}{
		{
			name:          "empty monitors",
			monitors:      []*protocol.Monitor{},
			expectedIndex: -1,
		},
		{
			name: "primary monitor exists",
			monitors: []*protocol.Monitor{
				{Name: "Monitor1", X: 1920, Y: 0, Primary: false},
				{Name: "Monitor2", X: 0, Y: 0, Primary: true},
				{Name: "Monitor3", X: -1920, Y: 0, Primary: false},
			},
			expectedIndex: 1,
		},
		{
			name: "monitor at 0,0 (no primary)",
			monitors: []*protocol.Monitor{
				{Name: "Monitor1", X: 1920, Y: 0, Primary: false},
				{Name: "Monitor2", X: 0, Y: 0, Primary: false},
				{Name: "Monitor3", X: -1920, Y: 0, Primary: false},
			},
			expectedIndex: 1,
		},
		{
			name: "fallback to first monitor",
			monitors: []*protocol.Monitor{
				{Name: "Monitor1", X: 1920, Y: 100, Primary: false},
				{Name: "Monitor2", X: 100, Y: 100, Primary: false},
			},
			expectedIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cm.findMainMonitor(tt.monitors)
			if tt.expectedIndex == -1 {
				if got != nil {
					t.Errorf("findMainMonitor() = %v, want nil", got)
				}
			} else {
				if got != tt.monitors[tt.expectedIndex] {
					t.Errorf("findMainMonitor() = %v, want %v", got, tt.monitors[tt.expectedIndex])
				}
			}
		})
	}
}