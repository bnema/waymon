package input

import (
	"testing"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/display"
	"github.com/stretchr/testify/assert"
)

// TestEdgeDetector_GetHostForEdge tests the edge to host mapping logic
func TestEdgeDetector_GetHostForEdgeMapping(t *testing.T) {
	tests := []struct {
		name         string
		edgeMappings []config.EdgeMapping
		monitor      *display.Monitor
		edge         display.Edge
		expectedHost string
	}{
		{
			name: "Primary monitor mapping",
			edgeMappings: []config.EdgeMapping{
				{Edge: "right", MonitorID: "primary", Host: "server1"},
			},
			monitor:      &display.Monitor{ID: "1", Name: "Mon1", Primary: true},
			edge:         display.EdgeRight,
			expectedHost: "server1",
		},
		{
			name: "Monitor ID mapping",
			edgeMappings: []config.EdgeMapping{
				{Edge: "left", MonitorID: "mon2", Host: "server2"},
			},
			monitor:      &display.Monitor{ID: "mon2", Name: "Monitor 2"},
			edge:         display.EdgeLeft,
			expectedHost: "server2",
		},
		{
			name: "Monitor name mapping",
			edgeMappings: []config.EdgeMapping{
				{Edge: "top", MonitorID: "LG Ultra", Host: "server3"},
			},
			monitor:      &display.Monitor{ID: "3", Name: "LG Ultra"},
			edge:         display.EdgeTop,
			expectedHost: "server3",
		},
		{
			name: "Wildcard mapping",
			edgeMappings: []config.EdgeMapping{
				{Edge: "bottom", MonitorID: "*", Host: "server4"},
			},
			monitor:      &display.Monitor{ID: "any", Name: "Any Monitor"},
			edge:         display.EdgeBottom,
			expectedHost: "server4",
		},
		{
			name:         "No mapping found",
			edgeMappings: []config.EdgeMapping{},
			monitor:      &display.Monitor{ID: "1", Name: "Mon1"},
			edge:         display.EdgeRight,
			expectedHost: "",
		},
		{
			name:         "Legacy config fallback",
			edgeMappings: []config.EdgeMapping{},
			monitor:      &display.Monitor{ID: "1", Name: "Mon1"},
			edge:         display.EdgeRight,
			expectedHost: "legacy-server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup config
			cfg := &config.Config{
				Client: config.ClientConfig{
					EdgeMappings: tt.edgeMappings,
				},
			}

			// For legacy test case
			if tt.name == "Legacy config fallback" {
				cfg.Client.ScreenPosition = "right"
				cfg.Client.ServerAddress = "legacy-server"
			}

			config.Set(cfg)

			// Create a minimal EdgeDetector for testing
			ed := &EdgeDetector{
				edgeMappings: tt.edgeMappings,
			}

			// Test the method
			host := ed.getHostForEdge(tt.monitor, tt.edge)
			assert.Equal(t, tt.expectedHost, host)
		})
	}
}

// TestEdgeDetector_Lifecycle tests starting and stopping
func TestEdgeDetector_Lifecycle(t *testing.T) {
	// Create a minimal EdgeDetector
	ed := &EdgeDetector{}

	// Test initial state
	assert.False(t, ed.active)
	assert.False(t, ed.capturing)

	// Start should succeed
	err := ed.Start()
	assert.NoError(t, err)
	assert.True(t, ed.active)

	// Starting again should be a no-op
	err = ed.Start()
	assert.NoError(t, err)

	// Stop should succeed
	ed.Stop()
	assert.False(t, ed.active)

	// Stopping again should be a no-op
	ed.Stop() // Should not panic
}

// TestEdgeDetector_CaptureState tests capture state management
func TestEdgeDetector_CaptureState(t *testing.T) {
	ed := &EdgeDetector{}

	// Initial state
	assert.False(t, ed.IsCapturing())

	// Set capturing state
	ed.mu.Lock()
	ed.capturing = true
	ed.mu.Unlock()

	assert.True(t, ed.IsCapturing())

	// Stop capturing
	ed.mu.Lock()
	ed.capturing = false
	ed.mu.Unlock()

	assert.False(t, ed.IsCapturing())
}
