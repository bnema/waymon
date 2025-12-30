package server

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bnema/waymon/internal/domain"
)

func TestCalculateTotalDisplayBounds(t *testing.T) {
	tests := []struct {
		name     string
		monitors []domain.Monitor
		want     domain.DisplayBounds
	}{
		{
			name:     "empty monitors returns default 1080p",
			monitors: []domain.Monitor{},
			want: domain.DisplayBounds{
				MinX: 0,
				MinY: 0,
				MaxX: 1920,
				MaxY: 1080,
			},
		},
		{
			name: "single monitor at origin",
			monitors: []domain.Monitor{
				{X: 0, Y: 0, Width: 1920, Height: 1080},
			},
			want: domain.DisplayBounds{
				MinX: 0,
				MinY: 0,
				MaxX: 1920,
				MaxY: 1080,
			},
		},
		{
			name: "single monitor with offset",
			monitors: []domain.Monitor{
				{X: 100, Y: 50, Width: 1920, Height: 1080},
			},
			want: domain.DisplayBounds{
				MinX: 100,
				MinY: 50,
				MaxX: 2020,
				MaxY: 1130,
			},
		},
		{
			name: "two monitors side by side",
			monitors: []domain.Monitor{
				{X: 0, Y: 0, Width: 1920, Height: 1080},
				{X: 1920, Y: 0, Width: 1920, Height: 1080},
			},
			want: domain.DisplayBounds{
				MinX: 0,
				MinY: 0,
				MaxX: 3840,
				MaxY: 1080,
			},
		},
		{
			name: "two monitors stacked vertically",
			monitors: []domain.Monitor{
				{X: 0, Y: 0, Width: 1920, Height: 1080},
				{X: 0, Y: 1080, Width: 1920, Height: 1080},
			},
			want: domain.DisplayBounds{
				MinX: 0,
				MinY: 0,
				MaxX: 1920,
				MaxY: 2160,
			},
		},
		{
			name: "three monitors L-shape",
			monitors: []domain.Monitor{
				{X: 0, Y: 0, Width: 1920, Height: 1080},
				{X: 1920, Y: 0, Width: 1920, Height: 1080},
				{X: 0, Y: 1080, Width: 1920, Height: 1080},
			},
			want: domain.DisplayBounds{
				MinX: 0,
				MinY: 0,
				MaxX: 3840,
				MaxY: 2160,
			},
		},
		{
			name: "monitors with negative coordinates",
			monitors: []domain.Monitor{
				{X: -1920, Y: 0, Width: 1920, Height: 1080},
				{X: 0, Y: 0, Width: 1920, Height: 1080},
			},
			want: domain.DisplayBounds{
				MinX: -1920,
				MinY: 0,
				MaxX: 1920,
				MaxY: 1080,
			},
		},
		{
			name: "monitors with different sizes",
			monitors: []domain.Monitor{
				{X: 0, Y: 0, Width: 2560, Height: 1440},
				{X: 2560, Y: 200, Width: 1920, Height: 1080},
			},
			want: domain.DisplayBounds{
				MinX: 0,
				MinY: 0,
				MaxX: 4480,
				MaxY: 1440,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateTotalDisplayBounds(tt.monitors)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFindMainMonitor(t *testing.T) {
	tests := []struct {
		name     string
		monitors []domain.Monitor
		want     *domain.Monitor
	}{
		{
			name:     "empty monitors returns nil",
			monitors: []domain.Monitor{},
			want:     nil,
		},
		{
			name: "single monitor returns that monitor",
			monitors: []domain.Monitor{
				{ID: "1", Name: "Monitor1", X: 0, Y: 0, Width: 1920, Height: 1080},
			},
			want: &domain.Monitor{ID: "1", Name: "Monitor1", X: 0, Y: 0, Width: 1920, Height: 1080},
		},
		{
			name: "primary monitor is returned first",
			monitors: []domain.Monitor{
				{ID: "1", Name: "Secondary", X: 1920, Y: 0, Width: 1920, Height: 1080, Primary: false},
				{ID: "2", Name: "Primary", X: 0, Y: 0, Width: 2560, Height: 1440, Primary: true},
			},
			want: &domain.Monitor{ID: "2", Name: "Primary", X: 0, Y: 0, Width: 2560, Height: 1440, Primary: true},
		},
		{
			name: "monitor at origin if no primary",
			monitors: []domain.Monitor{
				{ID: "1", Name: "Right", X: 1920, Y: 0, Width: 1920, Height: 1080, Primary: false},
				{ID: "2", Name: "Origin", X: 0, Y: 0, Width: 1920, Height: 1080, Primary: false},
			},
			want: &domain.Monitor{ID: "2", Name: "Origin", X: 0, Y: 0, Width: 1920, Height: 1080, Primary: false},
		},
		{
			name: "first monitor if no primary and none at origin",
			monitors: []domain.Monitor{
				{ID: "1", Name: "First", X: 100, Y: 100, Width: 1920, Height: 1080, Primary: false},
				{ID: "2", Name: "Second", X: 2020, Y: 100, Width: 1920, Height: 1080, Primary: false},
			},
			want: &domain.Monitor{ID: "1", Name: "First", X: 100, Y: 100, Width: 1920, Height: 1080, Primary: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindMainMonitor(tt.monitors)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.want.ID, got.ID)
				assert.Equal(t, tt.want.Name, got.Name)
				assert.Equal(t, tt.want.Primary, got.Primary)
			}
		})
	}
}

func TestFindPrimaryMonitor(t *testing.T) {
	tests := []struct {
		name     string
		monitors []domain.Monitor
		want     *domain.Monitor
	}{
		{
			name:     "empty monitors returns nil",
			monitors: []domain.Monitor{},
			want:     nil,
		},
		{
			name: "no primary monitor returns nil",
			monitors: []domain.Monitor{
				{ID: "1", Name: "Monitor1", Primary: false},
				{ID: "2", Name: "Monitor2", Primary: false},
			},
			want: nil,
		},
		{
			name: "returns primary monitor",
			monitors: []domain.Monitor{
				{ID: "1", Name: "Secondary", Primary: false},
				{ID: "2", Name: "Primary", Primary: true},
			},
			want: &domain.Monitor{ID: "2", Name: "Primary", Primary: true},
		},
		{
			name: "returns first primary if multiple",
			monitors: []domain.Monitor{
				{ID: "1", Name: "Primary1", Primary: true},
				{ID: "2", Name: "Primary2", Primary: true},
			},
			want: &domain.Monitor{ID: "1", Name: "Primary1", Primary: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindPrimaryMonitor(tt.monitors)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.want.ID, got.ID)
			}
		})
	}
}

func TestFindMonitorAtOrigin(t *testing.T) {
	tests := []struct {
		name     string
		monitors []domain.Monitor
		want     *domain.Monitor
	}{
		{
			name:     "empty monitors returns nil",
			monitors: []domain.Monitor{},
			want:     nil,
		},
		{
			name: "no monitor at origin returns nil",
			monitors: []domain.Monitor{
				{ID: "1", Name: "Monitor1", X: 100, Y: 100},
				{ID: "2", Name: "Monitor2", X: 1920, Y: 0},
			},
			want: nil,
		},
		{
			name: "returns monitor at origin",
			monitors: []domain.Monitor{
				{ID: "1", Name: "Right", X: 1920, Y: 0},
				{ID: "2", Name: "Origin", X: 0, Y: 0},
			},
			want: &domain.Monitor{ID: "2", Name: "Origin", X: 0, Y: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindMonitorAtOrigin(tt.monitors)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.want.ID, got.ID)
			}
		})
	}
}

func TestConstrainCursorPosition(t *testing.T) {
	bounds := domain.DisplayBounds{
		MinX: 0,
		MinY: 0,
		MaxX: 1920,
		MaxY: 1080,
	}

	tests := []struct {
		name   string
		x, y   float64
		bounds domain.DisplayBounds
		wantX  float64
		wantY  float64
	}{
		{
			name:   "position within bounds unchanged",
			x:      960,
			y:      540,
			bounds: bounds,
			wantX:  960,
			wantY:  540,
		},
		{
			name:   "position at origin unchanged",
			x:      0,
			y:      0,
			bounds: bounds,
			wantX:  0,
			wantY:  0,
		},
		{
			name:   "position at max unchanged",
			x:      1920,
			y:      1080,
			bounds: bounds,
			wantX:  1920,
			wantY:  1080,
		},
		{
			name:   "negative X constrained to MinX",
			x:      -100,
			y:      540,
			bounds: bounds,
			wantX:  0,
			wantY:  540,
		},
		{
			name:   "negative Y constrained to MinY",
			x:      960,
			y:      -50,
			bounds: bounds,
			wantX:  960,
			wantY:  0,
		},
		{
			name:   "X beyond MaxX constrained",
			x:      2000,
			y:      540,
			bounds: bounds,
			wantX:  1920,
			wantY:  540,
		},
		{
			name:   "Y beyond MaxY constrained",
			x:      960,
			y:      1200,
			bounds: bounds,
			wantX:  960,
			wantY:  1080,
		},
		{
			name:   "both X and Y beyond bounds constrained",
			x:      3000,
			y:      2000,
			bounds: bounds,
			wantX:  1920,
			wantY:  1080,
		},
		{
			name:   "both X and Y negative constrained",
			x:      -500,
			y:      -200,
			bounds: bounds,
			wantX:  0,
			wantY:  0,
		},
		{
			name: "works with negative bounds",
			x:    -2000,
			y:    100,
			bounds: domain.DisplayBounds{
				MinX: -1920,
				MinY: 0,
				MaxX: 1920,
				MaxY: 1080,
			},
			wantX: -1920,
			wantY: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotX, gotY := ConstrainCursorPosition(tt.x, tt.y, tt.bounds)
			assert.Equal(t, tt.wantX, gotX)
			assert.Equal(t, tt.wantY, gotY)
		})
	}
}

func TestIsWithinBounds(t *testing.T) {
	bounds := domain.DisplayBounds{
		MinX: 0,
		MinY: 0,
		MaxX: 1920,
		MaxY: 1080,
	}

	tests := []struct {
		name   string
		x, y   float64
		bounds domain.DisplayBounds
		want   bool
	}{
		{
			name:   "center is within bounds",
			x:      960,
			y:      540,
			bounds: bounds,
			want:   true,
		},
		{
			name:   "origin is within bounds",
			x:      0,
			y:      0,
			bounds: bounds,
			want:   true,
		},
		{
			name:   "max corner is within bounds",
			x:      1920,
			y:      1080,
			bounds: bounds,
			want:   true,
		},
		{
			name:   "negative X is outside bounds",
			x:      -1,
			y:      540,
			bounds: bounds,
			want:   false,
		},
		{
			name:   "negative Y is outside bounds",
			x:      960,
			y:      -1,
			bounds: bounds,
			want:   false,
		},
		{
			name:   "X beyond max is outside bounds",
			x:      1921,
			y:      540,
			bounds: bounds,
			want:   false,
		},
		{
			name:   "Y beyond max is outside bounds",
			x:      960,
			y:      1081,
			bounds: bounds,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWithinBounds(tt.x, tt.y, tt.bounds)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCalculateMonitorCenter(t *testing.T) {
	tests := []struct {
		name    string
		monitor *domain.Monitor
		wantX   float64
		wantY   float64
	}{
		{
			name:    "1080p monitor at origin",
			monitor: &domain.Monitor{X: 0, Y: 0, Width: 1920, Height: 1080},
			wantX:   960,
			wantY:   540,
		},
		{
			name:    "1440p monitor at origin",
			monitor: &domain.Monitor{X: 0, Y: 0, Width: 2560, Height: 1440},
			wantX:   1280,
			wantY:   720,
		},
		{
			name:    "monitor with offset",
			monitor: &domain.Monitor{X: 1920, Y: 100, Width: 1920, Height: 1080},
			wantX:   2880, // 1920 + 960
			wantY:   640,  // 100 + 540
		},
		{
			name:    "monitor with negative offset",
			monitor: &domain.Monitor{X: -1920, Y: 0, Width: 1920, Height: 1080},
			wantX:   -960, // -1920 + 960
			wantY:   540,
		},
		{
			name:    "small monitor",
			monitor: &domain.Monitor{X: 0, Y: 0, Width: 800, Height: 600},
			wantX:   400,
			wantY:   300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotX, gotY := CalculateMonitorCenter(tt.monitor)
			assert.Equal(t, tt.wantX, gotX)
			assert.Equal(t, tt.wantY, gotY)
		})
	}
}
