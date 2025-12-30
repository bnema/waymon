package ipc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/bnema/waymon/internal/boundaries/in"
)

// MockIPCHandler is a mock implementation of the IPCHandler interface.
type MockIPCHandler struct {
	mock.Mock
}

func (m *MockIPCHandler) HandleSwitch(ctx context.Context, action in.SwitchAction) (*in.StatusResponse, error) {
	args := m.Called(ctx, action)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*in.StatusResponse), args.Error(1)
}

func (m *MockIPCHandler) HandleStatus(ctx context.Context) (*in.StatusResponse, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*in.StatusResponse), args.Error(1)
}

func (m *MockIPCHandler) HandleRelease(ctx context.Context) (*in.StatusResponse, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*in.StatusResponse), args.Error(1)
}

func (m *MockIPCHandler) HandleConnect(ctx context.Context, slot int32) (*in.StatusResponse, error) {
	args := m.Called(ctx, slot)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*in.StatusResponse), args.Error(1)
}

// testContext returns a context with a zerolog logger for testing.
func testContext(t *testing.T) context.Context {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	return logger.WithContext(t.Context())
}

// tempSocketPath returns a unique temporary socket path for testing.
func tempSocketPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(os.TempDir(), "waymon-test-"+t.Name()+".sock")
}

func TestProtocol_SwitchMessage(t *testing.T) {
	tests := []struct {
		name   string
		action in.SwitchAction
		want   string
	}{
		{"next", in.SwitchActionNext, "next"},
		{"previous", in.SwitchActionPrevious, "previous"},
		{"enable", in.SwitchActionEnable, "enable"},
		{"disable", in.SwitchActionDisable, "disable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := NewSwitchMessage(tt.action)
			require.NoError(t, err)
			assert.Equal(t, MessageTypeSwitch, msg.Type)

			action, err := ParseSwitchRequest(msg)
			require.NoError(t, err)
			assert.Equal(t, tt.action, action)
		})
	}
}

func TestProtocol_StatusMessage(t *testing.T) {
	msg := NewStatusMessage()
	assert.Equal(t, MessageTypeStatus, msg.Type)
}

func TestProtocol_ReleaseMessage(t *testing.T) {
	msg := NewReleaseMessage()
	assert.Equal(t, MessageTypeRelease, msg.Type)
}

func TestProtocol_ConnectMessage(t *testing.T) {
	msg, err := NewConnectMessage(3)
	require.NoError(t, err)
	assert.Equal(t, MessageTypeConnect, msg.Type)

	slot, err := ParseConnectRequest(msg)
	require.NoError(t, err)
	assert.Equal(t, int32(3), slot)
}

func TestProtocol_StatusResponseMessage(t *testing.T) {
	resp := &in.StatusResponse{
		Active:        true,
		Connected:     true,
		ServerHost:    "192.168.1.100",
		CurrentIndex:  1,
		TotalCount:    3,
		ComputerNames: []string{"server", "laptop", "desktop"},
	}

	msg, err := NewStatusResponseMessage(resp)
	require.NoError(t, err)
	assert.Equal(t, MessageTypeStatusResponse, msg.Type)

	parsed, err := ParseStatusResponse(msg)
	require.NoError(t, err)
	assert.Equal(t, resp.Active, parsed.Active)
	assert.Equal(t, resp.Connected, parsed.Connected)
	assert.Equal(t, resp.ServerHost, parsed.ServerHost)
	assert.Equal(t, resp.CurrentIndex, parsed.CurrentIndex)
	assert.Equal(t, resp.TotalCount, parsed.TotalCount)
	assert.Equal(t, resp.ComputerNames, parsed.ComputerNames)
}

func TestProtocol_ErrorMessage(t *testing.T) {
	errMsg := "something went wrong"
	msg, err := NewErrorMessage(errMsg)
	require.NoError(t, err)
	assert.Equal(t, MessageTypeError, msg.Type)

	parsed, err := ParseErrorResponse(msg)
	require.NoError(t, err)
	assert.Equal(t, errMsg, parsed)
}

func TestServer_StartStop(t *testing.T) {
	ctx := testContext(t)
	socketPath := tempSocketPath(t)
	t.Cleanup(func() {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			t.Logf("failed to remove socket %s: %v", socketPath, err)
		}
	})

	handler := new(MockIPCHandler)
	server := NewServerWithPath(handler, socketPath)

	// Start server
	err := server.Start(ctx)
	require.NoError(t, err)

	// Verify socket exists
	_, err = os.Stat(socketPath)
	require.NoError(t, err)

	// Stop server
	server.Stop(ctx)

	// Verify socket is removed
	_, err = os.Stat(socketPath)
	assert.True(t, os.IsNotExist(err))
}

func TestServer_DoubleStart(t *testing.T) {
	ctx := testContext(t)
	socketPath := tempSocketPath(t)
	t.Cleanup(func() {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			t.Logf("failed to remove socket %s: %v", socketPath, err)
		}
	})

	handler := new(MockIPCHandler)
	server := NewServerWithPath(handler, socketPath)

	// Start server twice should be safe
	require.NoError(t, server.Start(ctx))
	require.NoError(t, server.Start(ctx))

	server.Stop(ctx)
}

func TestServer_DoubleStop(t *testing.T) {
	ctx := testContext(t)
	socketPath := tempSocketPath(t)
	t.Cleanup(func() {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			t.Logf("failed to remove socket %s: %v", socketPath, err)
		}
	})

	handler := new(MockIPCHandler)
	server := NewServerWithPath(handler, socketPath)

	require.NoError(t, server.Start(ctx))

	// Stop twice should be safe
	server.Stop(ctx)
	server.Stop(ctx)
}

func TestClientServer_Status(t *testing.T) {
	ctx := testContext(t)
	socketPath := tempSocketPath(t)
	t.Cleanup(func() {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			t.Logf("failed to remove socket %s: %v", socketPath, err)
		}
	})

	expectedResp := &in.StatusResponse{
		Active:        true,
		Connected:     true,
		ServerHost:    "test-host",
		CurrentIndex:  0,
		TotalCount:    2,
		ComputerNames: []string{"server", "client1"},
	}

	handler := new(MockIPCHandler)
	handler.On("HandleStatus", mock.Anything).Return(expectedResp, nil)

	server := NewServerWithPath(handler, socketPath)
	require.NoError(t, server.Start(ctx))
	defer server.Stop(ctx)

	// Give server time to start
	time.Sleep(10 * time.Millisecond)

	client := NewClientWithPath(socketPath)
	resp, err := client.SendStatus()
	require.NoError(t, err)

	assert.Equal(t, expectedResp.Active, resp.Active)
	assert.Equal(t, expectedResp.Connected, resp.Connected)
	assert.Equal(t, expectedResp.ServerHost, resp.ServerHost)
	assert.Equal(t, expectedResp.CurrentIndex, resp.CurrentIndex)
	assert.Equal(t, expectedResp.TotalCount, resp.TotalCount)

	handler.AssertExpectations(t)
}

func TestClientServer_Switch(t *testing.T) {
	ctx := testContext(t)
	socketPath := tempSocketPath(t)
	t.Cleanup(func() {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			t.Logf("failed to remove socket %s: %v", socketPath, err)
		}
	})

	expectedResp := &in.StatusResponse{
		Active:       true,
		CurrentIndex: 1,
		TotalCount:   2,
	}

	handler := new(MockIPCHandler)
	handler.On("HandleSwitch", mock.Anything, in.SwitchActionNext).Return(expectedResp, nil)

	server := NewServerWithPath(handler, socketPath)
	require.NoError(t, server.Start(ctx))
	defer server.Stop(ctx)

	time.Sleep(10 * time.Millisecond)

	client := NewClientWithPath(socketPath)
	resp, err := client.SendSwitchNext()
	require.NoError(t, err)

	assert.Equal(t, int32(1), resp.CurrentIndex)

	handler.AssertExpectations(t)
}

func TestClientServer_Release(t *testing.T) {
	ctx := testContext(t)
	socketPath := tempSocketPath(t)
	t.Cleanup(func() {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			t.Logf("failed to remove socket %s: %v", socketPath, err)
		}
	})

	expectedResp := &in.StatusResponse{
		Active:       false,
		CurrentIndex: 0,
		TotalCount:   2,
	}

	handler := new(MockIPCHandler)
	handler.On("HandleRelease", mock.Anything).Return(expectedResp, nil)

	server := NewServerWithPath(handler, socketPath)
	require.NoError(t, server.Start(ctx))
	defer server.Stop(ctx)

	time.Sleep(10 * time.Millisecond)

	client := NewClientWithPath(socketPath)
	resp, err := client.SendRelease()
	require.NoError(t, err)

	assert.False(t, resp.Active)
	assert.Equal(t, int32(0), resp.CurrentIndex)

	handler.AssertExpectations(t)
}

func TestClientServer_Connect(t *testing.T) {
	ctx := testContext(t)
	socketPath := tempSocketPath(t)
	t.Cleanup(func() {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			t.Logf("failed to remove socket %s: %v", socketPath, err)
		}
	})

	expectedResp := &in.StatusResponse{
		Active:       true,
		CurrentIndex: 2,
		TotalCount:   3,
	}

	handler := new(MockIPCHandler)
	handler.On("HandleConnect", mock.Anything, int32(2)).Return(expectedResp, nil)

	server := NewServerWithPath(handler, socketPath)
	require.NoError(t, server.Start(ctx))
	defer server.Stop(ctx)

	time.Sleep(10 * time.Millisecond)

	client := NewClientWithPath(socketPath)
	resp, err := client.SendConnect(2)
	require.NoError(t, err)

	assert.Equal(t, int32(2), resp.CurrentIndex)

	handler.AssertExpectations(t)
}

func TestClientServer_Stop(t *testing.T) {
	ctx := testContext(t)
	socketPath := tempSocketPath(t)
	t.Cleanup(func() {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			t.Logf("failed to remove socket %s: %v", socketPath, err)
		}
	})

	stopCalled := make(chan struct{})

	handler := new(MockIPCHandler)
	server := NewServerWithPath(handler, socketPath)
	server.OnStop = func() {
		close(stopCalled)
	}

	require.NoError(t, server.Start(ctx))
	defer server.Stop(ctx)

	time.Sleep(10 * time.Millisecond)

	client := NewClientWithPath(socketPath)
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

func TestClient_NotRunning(t *testing.T) {
	socketPath := tempSocketPath(t)
	// Don't create a server

	client := NewClientWithPath(socketPath)
	client.SetTimeout(100 * time.Millisecond)

	_, err := client.SendStatus()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "waymon is not running")
}

func TestClient_IsRunning(t *testing.T) {
	ctx := testContext(t)
	socketPath := tempSocketPath(t)
	t.Cleanup(func() {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			t.Logf("failed to remove socket %s: %v", socketPath, err)
		}
	})

	client := NewClientWithPath(socketPath)
	client.SetTimeout(100 * time.Millisecond)

	// Not running
	assert.False(t, client.IsRunning())

	// Start server
	handler := new(MockIPCHandler)
	handler.On("HandleStatus", mock.Anything).Return(&in.StatusResponse{}, nil)

	server := NewServerWithPath(handler, socketPath)
	require.NoError(t, server.Start(ctx))
	defer server.Stop(ctx)

	time.Sleep(10 * time.Millisecond)

	// Now running
	assert.True(t, client.IsRunning())
}

func TestClient_InvalidSlot(t *testing.T) {
	client := NewClientWithPath("/tmp/nonexistent.sock")

	_, err := client.SendConnect(-1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid slot number")
}

func TestGetSocketPath(t *testing.T) {
	path, err := GetSocketPath()
	require.NoError(t, err)
	assert.NotEmpty(t, path)
	assert.True(t, filepath.IsAbs(path))
}
