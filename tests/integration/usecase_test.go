//go:build integration

package integration

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/bnema/waymon/internal/domain"
	mocks "github.com/bnema/waymon/internal/mocks/out"
	clientuc "github.com/bnema/waymon/internal/usecase/client"
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

// TestServer_FullLifecycle tests server start/stop cycle with client registration.
func TestServer_FullLifecycle(t *testing.T) {
	ctx := testContext(t)

	// Create mocks
	mockInputCapture := mocks.NewMockInputCapturePort(t)
	mockNetwork := mocks.NewMockNetworkServerPort(t)
	mockConfig := mocks.NewMockConfigRepository(t)

	// Setup expectations for Start
	mockConfig.EXPECT().Load(mock.Anything).Return(&domain.Config{
		Server: domain.ServerCfg{
			Port:       52525,
			MaxClients: 5,
		},
	}, nil)
	mockNetwork.EXPECT().SetMaxClients(5).Return()
	mockInputCapture.EXPECT().SetEventCallback(mock.Anything).Return()
	mockInputCapture.EXPECT().Start(mock.Anything).Return(nil)

	// Setup expectations for Stop
	mockInputCapture.EXPECT().Stop().Return(nil)
	mockNetwork.EXPECT().Stop().Return()

	// Create server use case
	server := serveruc.NewServerUseCase(mockInputCapture, mockNetwork, mockConfig)

	// Test lifecycle
	t.Run("start", func(t *testing.T) {
		err := server.Start(ctx)
		require.NoError(t, err)
	})

	t.Run("initial_state", func(t *testing.T) {
		clients := server.GetConnectedClients(ctx)
		assert.Empty(t, clients)
		assert.True(t, server.IsControllingLocal(ctx))
		assert.Nil(t, server.GetActiveClient(ctx))
	})

	t.Run("stop", func(t *testing.T) {
		err := server.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("post_stop_state", func(t *testing.T) {
		clients := server.GetConnectedClients(ctx)
		assert.Empty(t, clients)
		assert.True(t, server.IsControllingLocal(ctx))
	})
}

// TestServer_ClientRegistration tests client registration and unregistration.
func TestServer_ClientRegistration(t *testing.T) {
	ctx := testContext(t)

	// Create mocks
	mockInputCapture := mocks.NewMockInputCapturePort(t)
	mockNetwork := mocks.NewMockNetworkServerPort(t)
	mockConfig := mocks.NewMockConfigRepository(t)

	// Setup expectations
	mockConfig.EXPECT().Load(mock.Anything).Return(&domain.Config{
		Server: domain.ServerCfg{Port: 52525, MaxClients: 5},
	}, nil)
	mockNetwork.EXPECT().SetMaxClients(5).Return()
	mockInputCapture.EXPECT().SetEventCallback(mock.Anything).Return()
	mockInputCapture.EXPECT().Start(mock.Anything).Return(nil)
	mockInputCapture.EXPECT().Stop().Return(nil)
	mockNetwork.EXPECT().Stop().Return()

	server := serveruc.NewServerUseCase(mockInputCapture, mockNetwork, mockConfig)
	require.NoError(t, server.Start(ctx))
	defer func() { _ = server.Stop(ctx) }()

	t.Run("register_single_client", func(t *testing.T) {
		server.RegisterClient(ctx, "client-1", "laptop", "192.168.1.10")
		clients := server.GetConnectedClients(ctx)
		require.Len(t, clients, 1)
		assert.Equal(t, "client-1", clients[0].ID)
		assert.Equal(t, "laptop", clients[0].Name)
		assert.Equal(t, "192.168.1.10", clients[0].Address)
	})

	t.Run("register_multiple_clients", func(t *testing.T) {
		server.RegisterClient(ctx, "client-2", "desktop", "192.168.1.20")
		server.RegisterClient(ctx, "client-3", "workstation", "192.168.1.30")
		clients := server.GetConnectedClients(ctx)
		assert.Len(t, clients, 3)
	})

	t.Run("unregister_client", func(t *testing.T) {
		server.UnregisterClient(ctx, "client-2")
		clients := server.GetConnectedClients(ctx)
		assert.Len(t, clients, 2)

		// Verify remaining clients
		ids := make([]string, len(clients))
		for i, c := range clients {
			ids[i] = c.ID
		}
		assert.Contains(t, ids, "client-1")
		assert.Contains(t, ids, "client-3")
		assert.NotContains(t, ids, "client-2")
	})

	t.Run("unregister_nonexistent", func(t *testing.T) {
		// Should not panic or error
		server.UnregisterClient(ctx, "nonexistent")
		clients := server.GetConnectedClients(ctx)
		assert.Len(t, clients, 2)
	})

	t.Run("duplicate_registration", func(t *testing.T) {
		// Registering same ID again should update the client
		server.RegisterClient(ctx, "client-1", "laptop-updated", "192.168.1.11")
		clients := server.GetConnectedClients(ctx)
		assert.Len(t, clients, 2) // Still 2 clients (client-1 updated, client-3 unchanged)
	})
}

// TestServer_ClientSwitching tests switching between clients.
func TestServer_ClientSwitching(t *testing.T) {
	ctx := testContext(t)

	// Create mocks
	mockInputCapture := mocks.NewMockInputCapturePort(t)
	mockNetwork := mocks.NewMockNetworkServerPort(t)
	mockConfig := mocks.NewMockConfigRepository(t)

	// Setup expectations
	mockConfig.EXPECT().Load(mock.Anything).Return(&domain.Config{
		Server: domain.ServerCfg{Port: 52525, MaxClients: 5},
	}, nil)
	mockNetwork.EXPECT().SetMaxClients(5).Return()
	mockInputCapture.EXPECT().SetEventCallback(mock.Anything).Return()
	mockInputCapture.EXPECT().Start(mock.Anything).Return(nil)
	mockInputCapture.EXPECT().Stop().Return(nil)
	mockNetwork.EXPECT().Stop().Return()

	// Allow SetTarget to be called multiple times
	mockInputCapture.EXPECT().SetTarget(mock.Anything).Return(nil).Maybe()
	// Allow SendEventToClient to be called
	mockNetwork.EXPECT().SendEventToClient(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	server := serveruc.NewServerUseCase(mockInputCapture, mockNetwork, mockConfig)
	require.NoError(t, server.Start(ctx))
	defer func() { _ = server.Stop(ctx) }()

	// Register clients
	server.RegisterClient(ctx, "client-a", "laptop-a", "192.168.1.10")
	server.RegisterClient(ctx, "client-b", "laptop-b", "192.168.1.20")
	server.RegisterClient(ctx, "client-c", "laptop-c", "192.168.1.30")

	t.Run("switch_to_client", func(t *testing.T) {
		err := server.SwitchToClient(ctx, "client-a")
		require.NoError(t, err)
		assert.False(t, server.IsControllingLocal(ctx))

		active := server.GetActiveClient(ctx)
		require.NotNil(t, active)
		assert.Equal(t, "client-a", active.ID)
	})

	t.Run("switch_to_another_client", func(t *testing.T) {
		err := server.SwitchToClient(ctx, "client-b")
		require.NoError(t, err)

		active := server.GetActiveClient(ctx)
		require.NotNil(t, active)
		assert.Equal(t, "client-b", active.ID)
	})

	t.Run("switch_to_nonexistent_client", func(t *testing.T) {
		err := server.SwitchToClient(ctx, "nonexistent")
		assert.ErrorIs(t, err, domain.ErrClientNotFound)
	})

	t.Run("switch_next", func(t *testing.T) {
		// Start from local
		err := server.SwitchToLocal(ctx)
		require.NoError(t, err)

		// Next should go to first client (alphabetically sorted)
		err = server.SwitchToNext(ctx)
		require.NoError(t, err)
		assert.False(t, server.IsControllingLocal(ctx))
	})

	t.Run("switch_previous_to_local", func(t *testing.T) {
		// From first client, previous should go to local
		// First ensure we're at first client
		err := server.SwitchToLocal(ctx)
		require.NoError(t, err)
		err = server.SwitchToNext(ctx)
		require.NoError(t, err)

		err = server.SwitchToPrevious(ctx)
		require.NoError(t, err)
		assert.True(t, server.IsControllingLocal(ctx))
	})

	t.Run("connect_to_slot", func(t *testing.T) {
		// Slot 0 = local
		err := server.ConnectToSlot(ctx, 0)
		require.NoError(t, err)
		assert.True(t, server.IsControllingLocal(ctx))

		// Slot 1 = first client
		err = server.ConnectToSlot(ctx, 1)
		require.NoError(t, err)
		assert.False(t, server.IsControllingLocal(ctx))
	})

	t.Run("connect_to_invalid_slot", func(t *testing.T) {
		err := server.ConnectToSlot(ctx, 10)
		assert.Error(t, err)
	})
}

// TestServer_LocalControl tests local control switching.
func TestServer_LocalControl(t *testing.T) {
	ctx := testContext(t)

	// Create mocks
	mockInputCapture := mocks.NewMockInputCapturePort(t)
	mockNetwork := mocks.NewMockNetworkServerPort(t)
	mockConfig := mocks.NewMockConfigRepository(t)

	// Setup expectations
	mockConfig.EXPECT().Load(mock.Anything).Return(&domain.Config{
		Server: domain.ServerCfg{Port: 52525, MaxClients: 5},
	}, nil)
	mockNetwork.EXPECT().SetMaxClients(5).Return()
	mockInputCapture.EXPECT().SetEventCallback(mock.Anything).Return()
	mockInputCapture.EXPECT().Start(mock.Anything).Return(nil)
	mockInputCapture.EXPECT().Stop().Return(nil)
	mockNetwork.EXPECT().Stop().Return()
	mockInputCapture.EXPECT().SetTarget(mock.Anything).Return(nil).Maybe()
	mockNetwork.EXPECT().SendEventToClient(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	server := serveruc.NewServerUseCase(mockInputCapture, mockNetwork, mockConfig)
	require.NoError(t, server.Start(ctx))
	defer func() { _ = server.Stop(ctx) }()

	// Register a client
	server.RegisterClient(ctx, "client-1", "laptop", "192.168.1.10")

	t.Run("initially_local", func(t *testing.T) {
		assert.True(t, server.IsControllingLocal(ctx))
		assert.Nil(t, server.GetActiveClient(ctx))
	})

	t.Run("switch_to_client_then_back", func(t *testing.T) {
		err := server.SwitchToClient(ctx, "client-1")
		require.NoError(t, err)
		assert.False(t, server.IsControllingLocal(ctx))

		err = server.SwitchToLocal(ctx)
		require.NoError(t, err)
		assert.True(t, server.IsControllingLocal(ctx))
		assert.Nil(t, server.GetActiveClient(ctx))
	})

	t.Run("switch_to_local_when_already_local", func(t *testing.T) {
		assert.True(t, server.IsControllingLocal(ctx))
		err := server.SwitchToLocal(ctx)
		require.NoError(t, err)
		assert.True(t, server.IsControllingLocal(ctx))
	})

	t.Run("active_client_disconnect_returns_to_local", func(t *testing.T) {
		// Switch to client
		err := server.SwitchToClient(ctx, "client-1")
		require.NoError(t, err)
		assert.False(t, server.IsControllingLocal(ctx))

		// Unregister the active client
		server.UnregisterClient(ctx, "client-1")

		// Should return to local
		assert.True(t, server.IsControllingLocal(ctx))
		assert.Nil(t, server.GetActiveClient(ctx))
	})
}

// TestServer_NoClientsEdgeCases tests edge cases with no clients connected.
func TestServer_NoClientsEdgeCases(t *testing.T) {
	ctx := testContext(t)

	mockInputCapture := mocks.NewMockInputCapturePort(t)
	mockNetwork := mocks.NewMockNetworkServerPort(t)
	mockConfig := mocks.NewMockConfigRepository(t)

	mockConfig.EXPECT().Load(mock.Anything).Return(&domain.Config{
		Server: domain.ServerCfg{Port: 52525, MaxClients: 5},
	}, nil)
	mockNetwork.EXPECT().SetMaxClients(5).Return()
	mockInputCapture.EXPECT().SetEventCallback(mock.Anything).Return()
	mockInputCapture.EXPECT().Start(mock.Anything).Return(nil)
	mockInputCapture.EXPECT().Stop().Return(nil)
	mockNetwork.EXPECT().Stop().Return()
	mockInputCapture.EXPECT().SetTarget(mock.Anything).Return(nil).Maybe()

	server := serveruc.NewServerUseCase(mockInputCapture, mockNetwork, mockConfig)
	require.NoError(t, server.Start(ctx))
	defer func() { _ = server.Stop(ctx) }()

	t.Run("switch_next_with_no_clients", func(t *testing.T) {
		err := server.SwitchToNext(ctx)
		require.NoError(t, err)
		assert.True(t, server.IsControllingLocal(ctx))
	})

	t.Run("switch_previous_with_no_clients", func(t *testing.T) {
		err := server.SwitchToPrevious(ctx)
		require.NoError(t, err)
		assert.True(t, server.IsControllingLocal(ctx))
	})

	t.Run("connect_to_slot_with_no_clients", func(t *testing.T) {
		// Slot 0 should work (local)
		err := server.ConnectToSlot(ctx, 0)
		require.NoError(t, err)

		// Slot 1 should fail (no clients)
		err = server.ConnectToSlot(ctx, 1)
		assert.ErrorIs(t, err, domain.ErrClientNotFound)
	})
}

// TestServer_RapidSwitching tests rapid switching between clients.
func TestServer_RapidSwitching(t *testing.T) {
	ctx := testContext(t)

	mockInputCapture := mocks.NewMockInputCapturePort(t)
	mockNetwork := mocks.NewMockNetworkServerPort(t)
	mockConfig := mocks.NewMockConfigRepository(t)

	mockConfig.EXPECT().Load(mock.Anything).Return(&domain.Config{
		Server: domain.ServerCfg{Port: 52525, MaxClients: 5},
	}, nil)
	mockNetwork.EXPECT().SetMaxClients(5).Return()
	mockInputCapture.EXPECT().SetEventCallback(mock.Anything).Return()
	mockInputCapture.EXPECT().Start(mock.Anything).Return(nil)
	mockInputCapture.EXPECT().Stop().Return(nil)
	mockNetwork.EXPECT().Stop().Return()
	mockInputCapture.EXPECT().SetTarget(mock.Anything).Return(nil).Maybe()
	mockNetwork.EXPECT().SendEventToClient(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	server := serveruc.NewServerUseCase(mockInputCapture, mockNetwork, mockConfig)
	require.NoError(t, server.Start(ctx))
	defer func() { _ = server.Stop(ctx) }()

	// Register multiple clients
	for i := 1; i <= 5; i++ {
		server.RegisterClient(ctx, fmt.Sprintf("client-%d", i), fmt.Sprintf("laptop-%d", i), fmt.Sprintf("192.168.1.%d", i))
	}

	t.Run("rapid_next_switching", func(t *testing.T) {
		// Rapid switching should not cause issues
		for i := 0; i < 20; i++ {
			err := server.SwitchToNext(ctx)
			require.NoError(t, err)
		}
	})

	t.Run("rapid_slot_switching", func(t *testing.T) {
		// Rapid slot switching
		for i := 0; i < 10; i++ {
			for slot := int32(0); slot <= 5; slot++ {
				err := server.ConnectToSlot(ctx, slot)
				require.NoError(t, err)
			}
		}
	})

	t.Run("switch_to_same_client_repeatedly", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			err := server.SwitchToClient(ctx, "client-1")
			require.NoError(t, err)
			assert.False(t, server.IsControllingLocal(ctx))
		}
	})
}

// =============================================================================
// Client Use Case Tests
// =============================================================================

// TestClient_ConnectDisconnect tests client connection and disconnection.
func TestClient_ConnectDisconnect(t *testing.T) {
	ctx := testContext(t)

	mockInjection := mocks.NewMockInputInjectionPort(t)
	mockNetwork := mocks.NewMockNetworkClientPort(t)
	mockDisplay := mocks.NewMockDisplayPort(t)
	mockConfig := mocks.NewMockConfigRepository(t)

	t.Run("successful_connection", func(t *testing.T) {
		// Setup expectations
		mockConfig.EXPECT().Load(mock.Anything).Return(&domain.Config{
			Client: domain.ClientCfg{
				ServerAddress: "192.168.1.100:52525",
				SSHPrivateKey: "/tmp/test_key",
			},
		}, nil).Once()
		mockInjection.EXPECT().Start(mock.Anything).Return(nil).Once()
		mockNetwork.EXPECT().SetOnInputEvent(mock.Anything).Return().Once()
		mockNetwork.EXPECT().SetOnDisconnected(mock.Anything).Return().Once()
		mockNetwork.EXPECT().Connect(mock.Anything, "192.168.1.100:52525", "/tmp/test_key").Return(nil).Once()
		mockDisplay.EXPECT().GetMonitors(mock.Anything).Return(CreateTestMonitors(SingleMonitor), nil).Once()
		mockNetwork.EXPECT().SendEvent(mock.Anything, mock.Anything).Return(nil).Once()

		client := clientuc.NewClientUseCase(mockInjection, mockNetwork, mockDisplay, mockConfig)

		err := client.Connect(ctx)
		require.NoError(t, err)
		assert.True(t, client.IsConnected())
	})
}

// TestClient_ConnectAlreadyConnected tests connecting when already connected.
func TestClient_ConnectAlreadyConnected(t *testing.T) {
	ctx := testContext(t)

	mockInjection := mocks.NewMockInputInjectionPort(t)
	mockNetwork := mocks.NewMockNetworkClientPort(t)
	mockDisplay := mocks.NewMockDisplayPort(t)
	mockConfig := mocks.NewMockConfigRepository(t)

	// Setup for first connection
	mockConfig.EXPECT().Load(mock.Anything).Return(&domain.Config{
		Client: domain.ClientCfg{
			ServerAddress: "192.168.1.100:52525",
			SSHPrivateKey: "/tmp/test_key",
		},
	}, nil).Once()
	mockInjection.EXPECT().Start(mock.Anything).Return(nil).Once()
	mockNetwork.EXPECT().SetOnInputEvent(mock.Anything).Return().Once()
	mockNetwork.EXPECT().SetOnDisconnected(mock.Anything).Return().Once()
	mockNetwork.EXPECT().Connect(mock.Anything, "192.168.1.100:52525", "/tmp/test_key").Return(nil).Once()
	mockDisplay.EXPECT().GetMonitors(mock.Anything).Return(CreateTestMonitors(SingleMonitor), nil).Once()
	mockNetwork.EXPECT().SendEvent(mock.Anything, mock.Anything).Return(nil).Once()

	client := clientuc.NewClientUseCase(mockInjection, mockNetwork, mockDisplay, mockConfig)

	// First connect
	err := client.Connect(ctx)
	require.NoError(t, err)

	// Second connect should fail
	err = client.Connect(ctx)
	assert.ErrorIs(t, err, domain.ErrAlreadyConnected)
}

// TestClient_DisconnectCycle tests the full connect/disconnect cycle.
func TestClient_DisconnectCycle(t *testing.T) {
	ctx := testContext(t)

	mockInjection := mocks.NewMockInputInjectionPort(t)
	mockNetwork := mocks.NewMockNetworkClientPort(t)
	mockDisplay := mocks.NewMockDisplayPort(t)
	mockConfig := mocks.NewMockConfigRepository(t)

	// Setup for connection
	mockConfig.EXPECT().Load(mock.Anything).Return(&domain.Config{
		Client: domain.ClientCfg{
			ServerAddress: "192.168.1.100:52525",
			SSHPrivateKey: "/tmp/test_key",
		},
	}, nil).Once()
	mockInjection.EXPECT().Start(mock.Anything).Return(nil).Once()
	mockNetwork.EXPECT().SetOnInputEvent(mock.Anything).Return().Once()
	mockNetwork.EXPECT().SetOnDisconnected(mock.Anything).Return().Once()
	mockNetwork.EXPECT().Connect(mock.Anything, "192.168.1.100:52525", "/tmp/test_key").Return(nil).Once()
	mockDisplay.EXPECT().GetMonitors(mock.Anything).Return(CreateTestMonitors(SingleMonitor), nil).Once()
	mockNetwork.EXPECT().SendEvent(mock.Anything, mock.Anything).Return(nil).Once()

	// Setup for disconnection
	mockNetwork.EXPECT().Disconnect().Return(nil).Once()
	mockInjection.EXPECT().Stop().Return(nil).Once()

	client := clientuc.NewClientUseCase(mockInjection, mockNetwork, mockDisplay, mockConfig)

	// Connect
	err := client.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	// Disconnect
	err = client.Disconnect(ctx)
	require.NoError(t, err)
	assert.False(t, client.IsConnected())
}

// TestClient_DisconnectWhenNotConnected tests disconnecting when not connected.
func TestClient_DisconnectWhenNotConnected(t *testing.T) {
	ctx := testContext(t)

	mockInjection := mocks.NewMockInputInjectionPort(t)
	mockNetwork := mocks.NewMockNetworkClientPort(t)
	mockDisplay := mocks.NewMockDisplayPort(t)
	mockConfig := mocks.NewMockConfigRepository(t)

	client := clientuc.NewClientUseCase(mockInjection, mockNetwork, mockDisplay, mockConfig)

	// Should not error when not connected
	err := client.Disconnect(ctx)
	require.NoError(t, err)
	assert.False(t, client.IsConnected())
}

// TestClient_ControlStatus tests control status management.
func TestClient_ControlStatus(t *testing.T) {
	ctx := testContext(t)

	mockInjection := mocks.NewMockInputInjectionPort(t)
	mockNetwork := mocks.NewMockNetworkClientPort(t)
	mockDisplay := mocks.NewMockDisplayPort(t)
	mockConfig := mocks.NewMockConfigRepository(t)

	client := clientuc.NewClientUseCase(mockInjection, mockNetwork, mockDisplay, mockConfig)

	t.Run("initial_status", func(t *testing.T) {
		status := client.GetControlStatus(ctx)
		assert.False(t, status.BeingControlled)
		assert.Empty(t, status.ControllerName)
	})

	t.Run("set_control_callback", func(t *testing.T) {
		var receivedStatus domain.ControlStatus
		client.SetOnControlChanged(func(status domain.ControlStatus) {
			receivedStatus = status
		})
		// Callback is set but not invoked until a control event happens
		_ = receivedStatus // Used in real scenario
	})

	t.Run("set_connection_callback", func(t *testing.T) {
		var receivedConnected bool
		var receivedServer string
		client.SetOnConnectionStateChanged(func(connected bool, serverName string) {
			receivedConnected = connected
			receivedServer = serverName
		})
		// Callback is set but not invoked until connection state changes
		_ = receivedConnected
		_ = receivedServer
	})
}

// TestClient_ConnectionFailed tests handling of failed connections.
func TestClient_ConnectionFailed(t *testing.T) {
	ctx := testContext(t)

	mockInjection := mocks.NewMockInputInjectionPort(t)
	mockNetwork := mocks.NewMockNetworkClientPort(t)
	mockDisplay := mocks.NewMockDisplayPort(t)
	mockConfig := mocks.NewMockConfigRepository(t)

	t.Run("config_load_fails", func(t *testing.T) {
		mockConfig.EXPECT().Load(mock.Anything).Return(nil, fmt.Errorf("config not found")).Once()

		client := clientuc.NewClientUseCase(mockInjection, mockNetwork, mockDisplay, mockConfig)

		err := client.Connect(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config")
		assert.False(t, client.IsConnected())
	})

	t.Run("input_injection_start_fails", func(t *testing.T) {
		mockConfig.EXPECT().Load(mock.Anything).Return(&domain.Config{
			Client: domain.ClientCfg{
				ServerAddress: "192.168.1.100:52525",
				SSHPrivateKey: "/tmp/test_key",
			},
		}, nil).Once()
		mockInjection.EXPECT().Start(mock.Anything).Return(fmt.Errorf("uinput not available")).Once()

		client := clientuc.NewClientUseCase(mockInjection, mockNetwork, mockDisplay, mockConfig)

		err := client.Connect(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "input injection")
		assert.False(t, client.IsConnected())
	})

	t.Run("network_connect_fails", func(t *testing.T) {
		mockConfig.EXPECT().Load(mock.Anything).Return(&domain.Config{
			Client: domain.ClientCfg{
				ServerAddress: "192.168.1.100:52525",
				SSHPrivateKey: "/tmp/test_key",
			},
		}, nil).Once()
		mockInjection.EXPECT().Start(mock.Anything).Return(nil).Once()
		mockNetwork.EXPECT().SetOnInputEvent(mock.Anything).Return().Once()
		mockNetwork.EXPECT().SetOnDisconnected(mock.Anything).Return().Once()
		mockNetwork.EXPECT().Connect(mock.Anything, "192.168.1.100:52525", "/tmp/test_key").Return(fmt.Errorf("connection refused")).Once()
		mockInjection.EXPECT().Stop().Return(nil).Once()

		client := clientuc.NewClientUseCase(mockInjection, mockNetwork, mockDisplay, mockConfig)

		err := client.Connect(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connect")
		assert.False(t, client.IsConnected())
	})
}
