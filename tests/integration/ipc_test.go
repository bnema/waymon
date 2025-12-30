//go:build integration

package integration

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/bnema/waymon/internal/adapters/in/ipc"
	"github.com/bnema/waymon/internal/boundaries/in"
)

// mockIPCHandler is a mock implementation of the IPCHandler interface for integration tests.
type mockIPCHandler struct {
	mock.Mock
}

func (m *mockIPCHandler) HandleSwitch(ctx context.Context, action in.SwitchAction) (*in.StatusResponse, error) {
	args := m.Called(ctx, action)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*in.StatusResponse), args.Error(1)
}

func (m *mockIPCHandler) HandleStatus(ctx context.Context) (*in.StatusResponse, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*in.StatusResponse), args.Error(1)
}

func (m *mockIPCHandler) HandleRelease(ctx context.Context) (*in.StatusResponse, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*in.StatusResponse), args.Error(1)
}

func (m *mockIPCHandler) HandleConnect(ctx context.Context, slot int32) (*in.StatusResponse, error) {
	args := m.Called(ctx, slot)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*in.StatusResponse), args.Error(1)
}

// TestIPC_StatusQuery tests querying status via IPC.
func TestIPC_StatusQuery(t *testing.T) {
	tests := []struct {
		name          string
		response      *in.StatusResponse
		expectSuccess bool
	}{
		{
			name: "empty_client_list",
			response: &in.StatusResponse{
				Active:        false,
				Connected:     true,
				ServerHost:    "localhost",
				CurrentIndex:  0,
				TotalCount:    0,
				ComputerNames: nil, // Use nil instead of empty slice for proper comparison
			},
			expectSuccess: true,
		},
		{
			name: "single_client",
			response: &in.StatusResponse{
				Active:        true,
				Connected:     true,
				ServerHost:    "192.168.1.100",
				CurrentIndex:  1,
				TotalCount:    1,
				ComputerNames: []string{"server", "laptop"},
			},
			expectSuccess: true,
		},
		{
			name: "many_clients",
			response: &in.StatusResponse{
				Active:        true,
				Connected:     true,
				ServerHost:    "192.168.1.100",
				CurrentIndex:  3,
				TotalCount:    5,
				ComputerNames: []string{"server", "laptop", "desktop", "workstation", "tablet"},
			},
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testContext(t)
			socketPath := tempSocketPath(t)

			handler := new(mockIPCHandler)
			handler.On("HandleStatus", mock.Anything).Return(tt.response, nil)

			server := ipc.NewServerWithPath(handler, socketPath)
			require.NoError(t, server.Start(ctx))
			defer server.Stop(ctx)

			time.Sleep(10 * time.Millisecond)

			client := ipc.NewClientWithPath(socketPath)
			resp, err := client.SendStatus()

			if tt.expectSuccess {
				require.NoError(t, err)
				assert.Equal(t, tt.response.Active, resp.Active)
				assert.Equal(t, tt.response.Connected, resp.Connected)
				assert.Equal(t, tt.response.ServerHost, resp.ServerHost)
				assert.Equal(t, tt.response.CurrentIndex, resp.CurrentIndex)
				assert.Equal(t, tt.response.TotalCount, resp.TotalCount)
				assert.Equal(t, tt.response.ComputerNames, resp.ComputerNames)
			} else {
				require.Error(t, err)
			}

			handler.AssertExpectations(t)
		})
	}
}

// TestIPC_SwitchNext tests switching to the next client.
func TestIPC_SwitchNext(t *testing.T) {
	tests := []struct {
		name         string
		initialIndex int32
		totalCount   int32
		expectIndex  int32
		description  string
	}{
		{
			name:         "basic_next",
			initialIndex: 0,
			totalCount:   3,
			expectIndex:  1,
			description:  "Switch from local to first client",
		},
		{
			name:         "wrap_around",
			initialIndex: 2,
			totalCount:   3,
			expectIndex:  0,
			description:  "Wrap around from last client to local",
		},
		{
			name:         "single_client",
			initialIndex: 0,
			totalCount:   1,
			expectIndex:  1,
			description:  "Switch with only one client",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testContext(t)
			socketPath := tempSocketPath(t)

			response := &in.StatusResponse{
				Active:       true,
				CurrentIndex: tt.expectIndex,
				TotalCount:   tt.totalCount,
			}

			handler := new(mockIPCHandler)
			handler.On("HandleSwitch", mock.Anything, in.SwitchActionNext).Return(response, nil)

			server := ipc.NewServerWithPath(handler, socketPath)
			require.NoError(t, server.Start(ctx))
			defer server.Stop(ctx)

			time.Sleep(10 * time.Millisecond)

			client := ipc.NewClientWithPath(socketPath)
			resp, err := client.SendSwitchNext()

			require.NoError(t, err)
			assert.Equal(t, tt.expectIndex, resp.CurrentIndex)
			handler.AssertExpectations(t)
		})
	}
}

// TestIPC_SwitchPrevious tests switching to the previous client.
func TestIPC_SwitchPrevious(t *testing.T) {
	tests := []struct {
		name         string
		initialIndex int32
		totalCount   int32
		expectIndex  int32
		description  string
	}{
		{
			name:         "basic_previous",
			initialIndex: 2,
			totalCount:   3,
			expectIndex:  1,
			description:  "Switch from client 2 to client 1",
		},
		{
			name:         "wrap_around",
			initialIndex: 0,
			totalCount:   3,
			expectIndex:  2,
			description:  "Wrap around from local to last client",
		},
		{
			name:         "at_first_client",
			initialIndex: 1,
			totalCount:   3,
			expectIndex:  0,
			description:  "Switch from first client to local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testContext(t)
			socketPath := tempSocketPath(t)

			response := &in.StatusResponse{
				Active:       tt.expectIndex != 0,
				CurrentIndex: tt.expectIndex,
				TotalCount:   tt.totalCount,
			}

			handler := new(mockIPCHandler)
			handler.On("HandleSwitch", mock.Anything, in.SwitchActionPrevious).Return(response, nil)

			server := ipc.NewServerWithPath(handler, socketPath)
			require.NoError(t, server.Start(ctx))
			defer server.Stop(ctx)

			time.Sleep(10 * time.Millisecond)

			client := ipc.NewClientWithPath(socketPath)
			resp, err := client.SendSwitchPrevious()

			require.NoError(t, err)
			assert.Equal(t, tt.expectIndex, resp.CurrentIndex)
			handler.AssertExpectations(t)
		})
	}
}

// TestIPC_SwitchToSlot tests switching to a specific slot.
func TestIPC_SwitchToSlot(t *testing.T) {
	tests := []struct {
		name        string
		slot        int32
		expectError bool
		description string
	}{
		{
			name:        "switch_to_local",
			slot:        0,
			expectError: false,
			description: "Switch to local (slot 0)",
		},
		{
			name:        "switch_to_client_1",
			slot:        1,
			expectError: false,
			description: "Switch to first client",
		},
		{
			name:        "switch_to_client_3",
			slot:        3,
			expectError: false,
			description: "Switch to third client",
		},
		{
			name:        "invalid_negative",
			slot:        -1,
			expectError: true,
			description: "Invalid negative slot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testContext(t)
			socketPath := tempSocketPath(t)

			handler := new(mockIPCHandler)
			if !tt.expectError {
				response := &in.StatusResponse{
					Active:       tt.slot != 0,
					CurrentIndex: tt.slot,
					TotalCount:   5,
				}
				handler.On("HandleConnect", mock.Anything, tt.slot).Return(response, nil)
			}

			server := ipc.NewServerWithPath(handler, socketPath)
			require.NoError(t, server.Start(ctx))
			defer server.Stop(ctx)

			time.Sleep(10 * time.Millisecond)

			client := ipc.NewClientWithPath(socketPath)
			resp, err := client.SendConnect(tt.slot)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.slot, resp.CurrentIndex)
			}
		})
	}
}

// TestIPC_ReleaseControl tests releasing control to local.
func TestIPC_ReleaseControl(t *testing.T) {
	tests := []struct {
		name         string
		wasActive    bool
		wasControlOn int32
		description  string
	}{
		{
			name:         "release_from_client",
			wasActive:    true,
			wasControlOn: 2,
			description:  "Release control from a client",
		},
		{
			name:         "already_local",
			wasActive:    false,
			wasControlOn: 0,
			description:  "Release when already on local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testContext(t)
			socketPath := tempSocketPath(t)

			response := &in.StatusResponse{
				Active:       false,
				CurrentIndex: 0,
				TotalCount:   3,
			}

			handler := new(mockIPCHandler)
			handler.On("HandleRelease", mock.Anything).Return(response, nil)

			server := ipc.NewServerWithPath(handler, socketPath)
			require.NoError(t, server.Start(ctx))
			defer server.Stop(ctx)

			time.Sleep(10 * time.Millisecond)

			client := ipc.NewClientWithPath(socketPath)
			resp, err := client.SendRelease()

			require.NoError(t, err)
			assert.False(t, resp.Active)
			assert.Equal(t, int32(0), resp.CurrentIndex)
			handler.AssertExpectations(t)
		})
	}
}

// TestIPC_ConcurrentRequests tests multiple IPC clients making concurrent requests.
func TestIPC_ConcurrentRequests(t *testing.T) {
	ctx := testContext(t)
	socketPath := tempSocketPath(t)

	// Track call counts
	var statusCount int32
	var switchCount int32

	handler := new(mockIPCHandler)
	handler.On("HandleStatus", mock.Anything).Run(func(_ mock.Arguments) {
		atomic.AddInt32(&statusCount, 1)
	}).Return(&in.StatusResponse{
		Active:       true,
		CurrentIndex: 1,
		TotalCount:   3,
	}, nil)

	handler.On("HandleSwitch", mock.Anything, mock.Anything).Run(func(_ mock.Arguments) {
		atomic.AddInt32(&switchCount, 1)
	}).Return(&in.StatusResponse{
		Active:       true,
		CurrentIndex: 2,
		TotalCount:   3,
	}, nil)

	server := ipc.NewServerWithPath(handler, socketPath)
	require.NoError(t, server.Start(ctx))
	defer server.Stop(ctx)

	time.Sleep(10 * time.Millisecond)

	// Run 10 concurrent requests
	const numRequests = 10
	var wg sync.WaitGroup
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			client := ipc.NewClientWithPath(socketPath)
			var err error
			if idx%2 == 0 {
				_, err = client.SendStatus()
			} else {
				_, err = client.SendSwitchNext()
			}
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("concurrent request error: %v", err)
	}

	// Verify all requests were processed
	totalCalls := atomic.LoadInt32(&statusCount) + atomic.LoadInt32(&switchCount)
	assert.Equal(t, int32(numRequests), totalCalls, "not all requests were processed")
}

// TestIPC_ServerShutdown tests behavior when server stops while IPC is active.
func TestIPC_ServerShutdown(t *testing.T) {
	ctx := testContext(t)
	socketPath := tempSocketPath(t)

	handler := new(mockIPCHandler)
	handler.On("HandleStatus", mock.Anything).Return(&in.StatusResponse{
		Active:     true,
		TotalCount: 1,
	}, nil)

	server := ipc.NewServerWithPath(handler, socketPath)
	require.NoError(t, server.Start(ctx))

	time.Sleep(10 * time.Millisecond)

	// First request should succeed
	client := ipc.NewClientWithPath(socketPath)
	_, err := client.SendStatus()
	require.NoError(t, err)

	// Stop server
	server.Stop(ctx)

	// Give time for cleanup
	time.Sleep(50 * time.Millisecond)

	// Next request should fail
	client.SetTimeout(100 * time.Millisecond)
	_, err = client.SendStatus()
	require.Error(t, err)
}

// TestIPC_ReconnectAfterRestart tests IPC reconnection after server restart.
func TestIPC_ReconnectAfterRestart(t *testing.T) {
	ctx := testContext(t)
	socketPath := tempSocketPath(t)

	handler := new(mockIPCHandler)
	handler.On("HandleStatus", mock.Anything).Return(&in.StatusResponse{
		Active:     true,
		TotalCount: 1,
	}, nil)

	// Start server
	server1 := ipc.NewServerWithPath(handler, socketPath)
	require.NoError(t, server1.Start(ctx))

	time.Sleep(10 * time.Millisecond)

	client := ipc.NewClientWithPath(socketPath)
	_, err := client.SendStatus()
	require.NoError(t, err, "first request should succeed")

	// Stop server
	server1.Stop(ctx)
	time.Sleep(50 * time.Millisecond)

	// Start new server
	server2 := ipc.NewServerWithPath(handler, socketPath)
	require.NoError(t, server2.Start(ctx))
	defer server2.Stop(ctx)

	time.Sleep(10 * time.Millisecond)

	// Reconnect should succeed
	_, err = client.SendStatus()
	require.NoError(t, err, "reconnect should succeed")
}

// TestIPC_InvalidCommands tests malformed IPC messages.
func TestIPC_InvalidCommands(t *testing.T) {
	ctx := testContext(t)
	socketPath := tempSocketPath(t)

	handler := new(mockIPCHandler)

	server := ipc.NewServerWithPath(handler, socketPath)
	require.NoError(t, server.Start(ctx))
	defer server.Stop(ctx)

	time.Sleep(10 * time.Millisecond)

	client := ipc.NewClientWithPath(socketPath)

	// Invalid slot number
	_, err := client.SendConnect(-1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid slot")
}

// TestIPC_IsRunning tests the IsRunning helper.
func TestIPC_IsRunning(t *testing.T) {
	ctx := testContext(t)
	socketPath := tempSocketPath(t)

	client := ipc.NewClientWithPath(socketPath)
	client.SetTimeout(100 * time.Millisecond)

	// Not running yet
	assert.False(t, client.IsRunning(), "should not be running before server starts")

	// Start server
	handler := new(mockIPCHandler)
	handler.On("HandleStatus", mock.Anything).Return(&in.StatusResponse{}, nil)

	server := ipc.NewServerWithPath(handler, socketPath)
	require.NoError(t, server.Start(ctx))

	time.Sleep(10 * time.Millisecond)

	// Now running
	assert.True(t, client.IsRunning(), "should be running after server starts")

	// Stop server
	server.Stop(ctx)
	time.Sleep(50 * time.Millisecond)

	// Not running anymore
	assert.False(t, client.IsRunning(), "should not be running after server stops")
}

// TestIPC_StopCommand tests the stop command via IPC.
func TestIPC_StopCommand(t *testing.T) {
	ctx := testContext(t)
	socketPath := tempSocketPath(t)

	stopCalled := make(chan struct{})

	handler := new(mockIPCHandler)
	server := ipc.NewServerWithPath(handler, socketPath)
	server.OnStop = func() {
		close(stopCalled)
	}

	require.NoError(t, server.Start(ctx))
	defer server.Stop(ctx)

	time.Sleep(10 * time.Millisecond)

	client := ipc.NewClientWithPath(socketPath)
	err := client.SendStop()
	require.NoError(t, err)

	// Verify OnStop was called
	select {
	case <-stopCalled:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("OnStop was not called")
	}
}
