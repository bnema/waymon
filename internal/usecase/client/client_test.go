package client

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/bnema/waymon/internal/domain"
	mocks "github.com/bnema/waymon/internal/mocks/out"
)

// testCtx returns a context with a nop zerolog logger attached.
func testCtx(t *testing.T) context.Context {
	logger := zerolog.Nop()
	return logger.WithContext(t.Context())
}

// Helper to create a configured UseCaseImpl for tests.
// Returns the concrete type for internal state inspection in tests.
func newTestClientUseCase(t *testing.T) (*UseCaseImpl, *mocks.MockInputInjectionPort, *mocks.MockNetworkClientPort, *mocks.MockDisplayPort, *mocks.MockConfigRepository) {
	inputInjection := mocks.NewMockInputInjectionPort(t)
	network := mocks.NewMockNetworkClientPort(t)
	display := mocks.NewMockDisplayPort(t)
	configRepo := mocks.NewMockConfigRepository(t)

	uc := NewClientUseCase(inputInjection, network, display, configRepo)
	// Type assert to concrete type for test access to internal state
	impl := uc.(*UseCaseImpl)
	return impl, inputInjection, network, display, configRepo
}

func TestNewClientUseCase(t *testing.T) {
	uc, inputInjection, network, display, configRepo := newTestClientUseCase(t)

	assert.NotNil(t, uc)
	assert.Equal(t, inputInjection, uc.inputInjection)
	assert.Equal(t, network, uc.network)
	assert.Equal(t, display, uc.display)
	assert.Equal(t, configRepo, uc.configRepo)
	assert.NotEmpty(t, uc.clientID) // Should have hostname or "unknown-client"
}

func TestUseCaseImpl_Connect(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockInputInjectionPort, *mocks.MockNetworkClientPort, *mocks.MockDisplayPort, *mocks.MockConfigRepository)
		expectedError bool
	}{
		{
			name: "successful connection",
			setupMocks: func(ii *mocks.MockInputInjectionPort, nc *mocks.MockNetworkClientPort, dp *mocks.MockDisplayPort, cr *mocks.MockConfigRepository) {
				cr.On("Load", mock.Anything).Return(&domain.Config{
					Client: domain.ClientCfg{
						ServerAddress: "192.168.1.100:52525",
						SSHPrivateKey: "/path/to/key",
					},
				}, nil)
				ii.On("Start", mock.Anything).Return(nil)
				nc.On("SetOnInputEvent", mock.Anything).Return()
				nc.On("SetOnDisconnected", mock.Anything).Return()
				nc.On("Connect", mock.Anything, "192.168.1.100:52525", "/path/to/key").Return(nil)
				dp.On("GetMonitors", mock.Anything).Return([]domain.Monitor{
					{Name: "Monitor1", Width: 1920, Height: 1080},
				}, nil)
				nc.On("SendEvent", mock.Anything, mock.Anything).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "fails when config load fails",
			setupMocks: func(ii *mocks.MockInputInjectionPort, nc *mocks.MockNetworkClientPort, dp *mocks.MockDisplayPort, cr *mocks.MockConfigRepository) {
				cr.On("Load", mock.Anything).Return(nil, domain.ErrConfigNotFound)
			},
			expectedError: true,
		},
		{
			name: "fails when input injection start fails",
			setupMocks: func(ii *mocks.MockInputInjectionPort, nc *mocks.MockNetworkClientPort, dp *mocks.MockDisplayPort, cr *mocks.MockConfigRepository) {
				cr.On("Load", mock.Anything).Return(&domain.Config{
					Client: domain.ClientCfg{ServerAddress: "192.168.1.100:52525"},
				}, nil)
				ii.On("Start", mock.Anything).Return(domain.ErrInputDeviceUnavailable)
			},
			expectedError: true,
		},
		{
			name: "fails when network connect fails",
			setupMocks: func(ii *mocks.MockInputInjectionPort, nc *mocks.MockNetworkClientPort, dp *mocks.MockDisplayPort, cr *mocks.MockConfigRepository) {
				cr.On("Load", mock.Anything).Return(&domain.Config{
					Client: domain.ClientCfg{
						ServerAddress: "192.168.1.100:52525",
						SSHPrivateKey: "/path/to/key",
					},
				}, nil)
				ii.On("Start", mock.Anything).Return(nil)
				nc.On("SetOnInputEvent", mock.Anything).Return()
				nc.On("SetOnDisconnected", mock.Anything).Return()
				nc.On("Connect", mock.Anything, "192.168.1.100:52525", "/path/to/key").Return(domain.ErrConnectionClosed)
				ii.On("Stop").Return(nil)
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, inputInjection, network, display, configRepo := newTestClientUseCase(t)
			tt.setupMocks(inputInjection, network, display, configRepo)

			ctx := testCtx(t)
			err := uc.Connect(ctx)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.True(t, uc.connected)
				assert.True(t, uc.reconnectEnabled)
			}
		})
	}
}

func TestUseCaseImpl_Connect_AlreadyConnected(t *testing.T) {
	uc, _, _, _, _ := newTestClientUseCase(t)
	uc.connected = true

	ctx := testCtx(t)
	err := uc.Connect(ctx)

	assert.ErrorIs(t, err, domain.ErrAlreadyConnected)
}

func TestUseCaseImpl_Disconnect(t *testing.T) {
	tests := []struct {
		name       string
		setupState func(*UseCaseImpl)
		setupMocks func(*mocks.MockInputInjectionPort, *mocks.MockNetworkClientPort)
	}{
		{
			name: "disconnect when connected",
			setupState: func(uc *UseCaseImpl) {
				uc.connected = true
				uc.reconnectEnabled = true
				ctx, cancel := context.WithCancel(context.Background())
				uc.reconnectCtx = ctx
				uc.reconnectCancel = cancel
				uc.controlStatus = domain.ControlStatus{BeingControlled: true}
			},
			setupMocks: func(ii *mocks.MockInputInjectionPort, nc *mocks.MockNetworkClientPort) {
				nc.On("Disconnect").Return(nil)
				ii.On("Stop").Return(nil)
			},
		},
		{
			name: "disconnect when not connected is noop",
			setupState: func(uc *UseCaseImpl) {
				uc.connected = false
			},
			setupMocks: func(ii *mocks.MockInputInjectionPort, nc *mocks.MockNetworkClientPort) {
				// No calls expected
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, inputInjection, network, _, _ := newTestClientUseCase(t)
			tt.setupState(uc)
			tt.setupMocks(inputInjection, network)

			ctx := testCtx(t)
			err := uc.Disconnect(ctx)

			assert.NoError(t, err)
			assert.False(t, uc.connected)
			assert.False(t, uc.reconnectEnabled)
			assert.False(t, uc.controlStatus.BeingControlled)
		})
	}
}

func TestUseCaseImpl_IsConnected(t *testing.T) {
	tests := []struct {
		name       string
		setupState func(*UseCaseImpl)
		want       bool
	}{
		{
			name:       "not connected by default",
			setupState: func(uc *UseCaseImpl) {},
			want:       false,
		},
		{
			name: "connected when flag is true",
			setupState: func(uc *UseCaseImpl) {
				uc.connected = true
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, _, _, _, _ := newTestClientUseCase(t)
			tt.setupState(uc)

			got := uc.IsConnected()

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUseCaseImpl_GetControlStatus(t *testing.T) {
	tests := []struct {
		name       string
		setupState func(*UseCaseImpl)
		want       domain.ControlStatus
	}{
		{
			name:       "default status",
			setupState: func(uc *UseCaseImpl) {},
			want:       domain.ControlStatus{},
		},
		{
			name: "being controlled",
			setupState: func(uc *UseCaseImpl) {
				uc.controlStatus = domain.ControlStatus{
					BeingControlled: true,
					ControllerName:  "server1",
				}
			},
			want: domain.ControlStatus{
				BeingControlled: true,
				ControllerName:  "server1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, _, _, _, _ := newTestClientUseCase(t)
			tt.setupState(uc)

			ctx := testCtx(t)
			got := uc.GetControlStatus(ctx)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUseCaseImpl_SetOnControlChanged(t *testing.T) {
	uc, _, _, _, _ := newTestClientUseCase(t)

	var receivedStatus domain.ControlStatus
	callback := func(status domain.ControlStatus) {
		receivedStatus = status
	}

	uc.SetOnControlChanged(callback)
	require.NotNil(t, uc.onControlChanged)

	// Trigger the callback
	uc.onControlChanged(domain.ControlStatus{BeingControlled: true})

	assert.True(t, receivedStatus.BeingControlled)
}

func TestUseCaseImpl_SetOnConnectionStateChanged(t *testing.T) {
	uc, _, _, _, _ := newTestClientUseCase(t)

	var receivedConnected bool
	var receivedServerName string
	callback := func(connected bool, serverName string) {
		receivedConnected = connected
		receivedServerName = serverName
	}

	uc.SetOnConnectionStateChanged(callback)
	require.NotNil(t, uc.onConnectionStateChange)

	// Trigger the callback
	uc.onConnectionStateChange(true, "server1")

	assert.True(t, receivedConnected)
	assert.Equal(t, "server1", receivedServerName)
}

func TestUseCaseImpl_handleControlEvent(t *testing.T) {
	tests := []struct {
		name           string
		controlEvent   *domain.ControlEvent
		setupMocks     func(*mocks.MockInputInjectionPort)
		wantControlled bool
		wantController string
	}{
		{
			name: "request control enables control",
			controlEvent: &domain.ControlEvent{
				Type:     domain.ControlRequestControl,
				TargetID: "server1",
			},
			setupMocks: func(ii *mocks.MockInputInjectionPort) {
				ii.On("SetExclusiveCapture", mock.Anything, true).Return(nil)
			},
			wantControlled: true,
			wantController: "server1",
		},
		{
			name: "release control disables control",
			controlEvent: &domain.ControlEvent{
				Type: domain.ControlReleaseControl,
			},
			setupMocks: func(ii *mocks.MockInputInjectionPort) {
				ii.On("SetExclusiveCapture", mock.Anything, false).Return(nil)
			},
			wantControlled: false,
			wantController: "",
		},
		{
			name: "switch to local disables control",
			controlEvent: &domain.ControlEvent{
				Type: domain.ControlSwitchToLocal,
			},
			setupMocks: func(ii *mocks.MockInputInjectionPort) {
				ii.On("SetExclusiveCapture", mock.Anything, false).Return(nil)
			},
			wantControlled: false,
			wantController: "",
		},
		{
			name: "server shutdown clears control",
			controlEvent: &domain.ControlEvent{
				Type: domain.ControlServerShutdown,
			},
			setupMocks: func(ii *mocks.MockInputInjectionPort) {
				// No calls expected - just clears state
			},
			wantControlled: false,
			wantController: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, inputInjection, _, _, _ := newTestClientUseCase(t)
			tt.setupMocks(inputInjection)

			// Set initial state for release/switch tests
			if tt.controlEvent.Type == domain.ControlReleaseControl || tt.controlEvent.Type == domain.ControlSwitchToLocal {
				uc.controlStatus = domain.ControlStatus{BeingControlled: true, ControllerName: "server1"}
			}

			ctx := testCtx(t)
			uc.handleControlEvent(ctx, tt.controlEvent)

			assert.Equal(t, tt.wantControlled, uc.controlStatus.BeingControlled)
			assert.Equal(t, tt.wantController, uc.controlStatus.ControllerName)
		})
	}
}

func TestUseCaseImpl_injectEvent(t *testing.T) {
	tests := []struct {
		name       string
		event      *domain.InputEvent
		setupMocks func(*mocks.MockInputInjectionPort)
		wantError  bool
	}{
		{
			name: "inject mouse move",
			event: &domain.InputEvent{
				MouseMove: &domain.MouseMoveEvent{DX: 10.0, DY: 20.0},
			},
			setupMocks: func(ii *mocks.MockInputInjectionPort) {
				ii.On("InjectMouseMove", mock.Anything, 10.0, 20.0).Return(nil)
			},
			wantError: false,
		},
		{
			name: "inject mouse position",
			event: &domain.InputEvent{
				MousePosition: &domain.MousePositionEvent{X: 100, Y: 200},
			},
			setupMocks: func(ii *mocks.MockInputInjectionPort) {
				ii.On("InjectMousePosition", mock.Anything, int32(100), int32(200)).Return(nil)
			},
			wantError: false,
		},
		{
			name: "inject mouse button",
			event: &domain.InputEvent{
				MouseButton: &domain.MouseButtonEvent{Button: 1, Pressed: true},
			},
			setupMocks: func(ii *mocks.MockInputInjectionPort) {
				ii.On("InjectMouseButton", mock.Anything, uint32(1), true).Return(nil)
			},
			wantError: false,
		},
		{
			name: "inject mouse scroll",
			event: &domain.InputEvent{
				MouseScroll: &domain.MouseScrollEvent{DX: 0, DY: 5.0, Type: domain.ScrollWheel},
			},
			setupMocks: func(ii *mocks.MockInputInjectionPort) {
				ii.On("InjectMouseScroll", mock.Anything, 0.0, 5.0, domain.ScrollWheel).Return(nil)
			},
			wantError: false,
		},
		{
			name: "inject keyboard event",
			event: &domain.InputEvent{
				Keyboard: &domain.KeyboardEvent{Key: 30, Pressed: true, Modifiers: 0},
			},
			setupMocks: func(ii *mocks.MockInputInjectionPort) {
				ii.On("InjectKeyEvent", mock.Anything, uint32(30), true, uint32(0)).Return(nil)
			},
			wantError: false,
		},
		{
			name:  "invalid event returns error",
			event: &domain.InputEvent{
				// No event type set
			},
			setupMocks: func(ii *mocks.MockInputInjectionPort) {
				// No calls expected
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, inputInjection, _, _, _ := newTestClientUseCase(t)
			tt.setupMocks(inputInjection)

			ctx := testCtx(t)
			err := uc.injectEvent(ctx, tt.event)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUseCaseImpl_processInputEvent(t *testing.T) {
	tests := []struct {
		name       string
		setupState func(*UseCaseImpl)
		event      *domain.InputEvent
		setupMocks func(*mocks.MockInputInjectionPort)
	}{
		{
			name: "control event handled",
			setupState: func(uc *UseCaseImpl) {
				uc.connected = true
			},
			event: &domain.InputEvent{
				Control: &domain.ControlEvent{
					Type:     domain.ControlRequestControl,
					TargetID: "server1",
				},
			},
			setupMocks: func(ii *mocks.MockInputInjectionPort) {
				ii.On("SetExclusiveCapture", mock.Anything, true).Return(nil)
			},
		},
		{
			name: "input ignored when not being controlled",
			setupState: func(uc *UseCaseImpl) {
				uc.connected = true
				uc.controlStatus.BeingControlled = false
			},
			event: &domain.InputEvent{
				MouseMove: &domain.MouseMoveEvent{DX: 10, DY: 20},
			},
			setupMocks: func(ii *mocks.MockInputInjectionPort) {
				// No injection calls expected
			},
		},
		{
			name: "input injected when being controlled",
			setupState: func(uc *UseCaseImpl) {
				uc.connected = true
				uc.controlStatus.BeingControlled = true
			},
			event: &domain.InputEvent{
				MouseMove: &domain.MouseMoveEvent{DX: 10, DY: 20},
			},
			setupMocks: func(ii *mocks.MockInputInjectionPort) {
				ii.On("InjectMouseMove", mock.Anything, 10.0, 20.0).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, inputInjection, _, _, _ := newTestClientUseCase(t)
			tt.setupState(uc)
			tt.setupMocks(inputInjection)

			ctx := testCtx(t)
			uc.processInputEvent(ctx, tt.event)
		})
	}
}

func TestGetWaylandCompositor(t *testing.T) {
	// This is a simple function that reads environment variables
	// We can't easily mock env vars, so just test that it returns something
	compositor := getWaylandCompositor()
	assert.NotEmpty(t, compositor)
}
