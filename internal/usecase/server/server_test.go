package server

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/bnema/waymon/internal/domain"
	mocks "github.com/bnema/waymon/internal/mocks/out"
)

// testCtx returns a context with a nop zerolog logger attached.
// Uses t.Context() as the base context for proper test lifecycle management.
func testCtx(t *testing.T) context.Context {
	logger := zerolog.Nop()
	return logger.WithContext(t.Context())
}

// Helper to create a configured UseCaseImpl for tests.
// Returns the concrete type for internal state inspection in tests.
func newTestServerUseCase(t *testing.T) (*UseCaseImpl, *mocks.MockInputCapturePort, *mocks.MockNetworkServerPort, *mocks.MockConfigRepository) {
	inputCapture := mocks.NewMockInputCapturePort(t)
	network := mocks.NewMockNetworkServerPort(t)
	configRepo := mocks.NewMockConfigRepository(t)

	uc := NewServerUseCase(inputCapture, network, configRepo)
	// Type assert to concrete type for test access to internal state
	impl := uc.(*UseCaseImpl)
	return impl, inputCapture, network, configRepo
}

func TestNewServerUseCase(t *testing.T) {
	inputCapture := mocks.NewMockInputCapturePort(t)
	network := mocks.NewMockNetworkServerPort(t)
	configRepo := mocks.NewMockConfigRepository(t)

	uc := NewServerUseCase(inputCapture, network, configRepo)
	require.NotNil(t, uc)

	// Type assert to concrete type for internal state verification
	impl, ok := uc.(*UseCaseImpl)
	require.True(t, ok, "NewServerUseCase should return *UseCaseImpl")

	assert.Equal(t, inputCapture, impl.inputCapture)
	assert.Equal(t, network, impl.network)
	assert.Equal(t, configRepo, impl.configRepo)
	assert.True(t, impl.controllingLocal)
	assert.NotNil(t, impl.clients)
	assert.NotNil(t, impl.clientCursors)
	assert.Equal(t, 5*time.Second, impl.emergencyCooldown)
}

func TestUseCaseImpl_Start(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockInputCapturePort, *mocks.MockNetworkServerPort, *mocks.MockConfigRepository)
		expectedError bool
	}{
		{
			name: "successful start with config",
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort, cr *mocks.MockConfigRepository) {
				cr.On("Load", mock.Anything).Return(&domain.Config{
					Server: domain.ServerCfg{
						Port:            52525,
						MaxClients:      5,
						SSHHostKeyPath:  "/path/to/host/key",
						SSHAuthKeysPath: "/path/to/auth/keys",
					},
				}, nil)
				ns.On("SetMaxClients", 5).Return()
				ic.On("SetEventCallback", mock.Anything).Return()
				ic.On("Start", mock.Anything).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "start with config load error uses defaults",
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort, cr *mocks.MockConfigRepository) {
				cr.On("Load", mock.Anything).Return(nil, domain.ErrConfigNotFound)
				ns.On("SetMaxClients", 5).Return()
				ic.On("SetEventCallback", mock.Anything).Return()
				ic.On("Start", mock.Anything).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "start fails when input capture fails",
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort, cr *mocks.MockConfigRepository) {
				cr.On("Load", mock.Anything).Return(&domain.Config{
					Server: domain.ServerCfg{MaxClients: 5},
				}, nil)
				ns.On("SetMaxClients", 5).Return()
				ic.On("SetEventCallback", mock.Anything).Return()
				ic.On("Start", mock.Anything).Return(domain.ErrInputDeviceUnavailable)
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, inputCapture, network, configRepo := newTestServerUseCase(t)
			tt.setupMocks(inputCapture, network, configRepo)

			ctx := testCtx(t)
			err := uc.Start(ctx)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.True(t, uc.running)
			}
		})
	}
}

func TestUseCaseImpl_Stop(t *testing.T) {
	tests := []struct {
		name       string
		setupState func(*UseCaseImpl)
		setupMocks func(*mocks.MockInputCapturePort, *mocks.MockNetworkServerPort)
	}{
		{
			name: "stop when running",
			setupState: func(uc *UseCaseImpl) {
				uc.running = true
				uc.clients["client1"] = &domain.Client{ID: "client1"}
				uc.clientCursors["client1"] = &domain.CursorState{}
				uc.activeClientID = "client1"
				uc.controllingLocal = false
			},
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				ic.On("Stop").Return(nil)
				ns.On("Stop").Return()
			},
		},
		{
			name: "stop when not running is noop",
			setupState: func(uc *UseCaseImpl) {
				uc.running = false
			},
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				// No calls expected
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, inputCapture, network, _ := newTestServerUseCase(t)
			tt.setupState(uc)
			tt.setupMocks(inputCapture, network)

			ctx := testCtx(t)
			err := uc.Stop(ctx)

			assert.NoError(t, err)
			assert.False(t, uc.running)
			assert.Empty(t, uc.clients)
			assert.Empty(t, uc.clientCursors)
			assert.Empty(t, uc.activeClientID)
			assert.True(t, uc.controllingLocal)
		})
	}
}

func TestUseCaseImpl_RegisterClient(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		clientName  string
		address     string
		wantClients int
	}{
		{
			name:        "register first client",
			id:          "client1",
			clientName:  "TestClient",
			address:     "192.168.1.100:52525",
			wantClients: 1,
		},
		{
			name:        "register second client",
			id:          "client2",
			clientName:  "TestClient2",
			address:     "192.168.1.101:52525",
			wantClients: 1, // Each test runs independently
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, _, _, _ := newTestServerUseCase(t)
			ctx := testCtx(t)

			uc.RegisterClient(ctx, tt.id, tt.clientName, tt.address)

			assert.Len(t, uc.clients, tt.wantClients)
			client, exists := uc.clients[tt.id]
			require.True(t, exists)
			assert.Equal(t, tt.id, client.ID)
			assert.Equal(t, tt.clientName, client.Name)
			assert.Equal(t, tt.address, client.Address)
			assert.Equal(t, domain.ClientIdle, client.Status)
		})
	}
}

func TestUseCaseImpl_UnregisterClient(t *testing.T) {
	tests := []struct {
		name             string
		setupState       func(*UseCaseImpl)
		clientID         string
		setupMocks       func(*mocks.MockInputCapturePort, *mocks.MockNetworkServerPort)
		wantClients      int
		wantLocal        bool
		wantActiveClient string
	}{
		{
			name: "unregister inactive client",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["client1"] = &domain.Client{ID: "client1", Name: "Client1"}
				uc.clients["client2"] = &domain.Client{ID: "client2", Name: "Client2"}
				uc.clientCursors["client1"] = &domain.CursorState{}
				uc.activeClientID = "client2"
				uc.controllingLocal = false
			},
			clientID: "client1",
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				// No input release needed for inactive client
			},
			wantClients:      1,
			wantLocal:        false,
			wantActiveClient: "client2",
		},
		{
			name: "unregister active client switches to local",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["client1"] = &domain.Client{ID: "client1", Name: "Client1"}
				uc.clientCursors["client1"] = &domain.CursorState{}
				uc.activeClientID = "client1"
				uc.controllingLocal = false
			},
			clientID: "client1",
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				ic.On("SetTarget", "").Return(nil)
			},
			wantClients:      0,
			wantLocal:        true,
			wantActiveClient: "",
		},
		{
			name: "unregister non-existent client is noop",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["client1"] = &domain.Client{ID: "client1", Name: "Client1"}
			},
			clientID: "nonexistent",
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				// No calls expected
			},
			wantClients:      1,
			wantLocal:        true, // Default state
			wantActiveClient: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, inputCapture, network, _ := newTestServerUseCase(t)
			tt.setupState(uc)
			tt.setupMocks(inputCapture, network)

			ctx := testCtx(t)
			uc.UnregisterClient(ctx, tt.clientID)

			assert.Len(t, uc.clients, tt.wantClients)
			assert.Equal(t, tt.wantLocal, uc.controllingLocal)
			assert.Equal(t, tt.wantActiveClient, uc.activeClientID)
		})
	}
}

func TestUseCaseImpl_GetConnectedClients(t *testing.T) {
	tests := []struct {
		name        string
		setupState  func(*UseCaseImpl)
		wantClients int
	}{
		{
			name:        "no clients",
			setupState:  func(uc *UseCaseImpl) {},
			wantClients: 0,
		},
		{
			name: "one client",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["client1"] = &domain.Client{ID: "client1", Name: "Client1"}
			},
			wantClients: 1,
		},
		{
			name: "multiple clients",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["client1"] = &domain.Client{ID: "client1", Name: "Client1"}
				uc.clients["client2"] = &domain.Client{ID: "client2", Name: "Client2"}
				uc.clients["client3"] = &domain.Client{ID: "client3", Name: "Client3"}
			},
			wantClients: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, _, _, _ := newTestServerUseCase(t)
			tt.setupState(uc)

			ctx := testCtx(t)
			clients := uc.GetConnectedClients(ctx)

			assert.Len(t, clients, tt.wantClients)
		})
	}
}

func TestUseCaseImpl_GetActiveClient(t *testing.T) {
	tests := []struct {
		name       string
		setupState func(*UseCaseImpl)
		wantNil    bool
		wantID     string
	}{
		{
			name:       "no active client",
			setupState: func(uc *UseCaseImpl) {},
			wantNil:    true,
		},
		{
			name: "active client exists",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["client1"] = &domain.Client{ID: "client1", Name: "Client1"}
				uc.activeClientID = "client1"
			},
			wantNil: false,
			wantID:  "client1",
		},
		{
			name: "active client ID set but client removed",
			setupState: func(uc *UseCaseImpl) {
				uc.activeClientID = "nonexistent"
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, _, _, _ := newTestServerUseCase(t)
			tt.setupState(uc)

			ctx := testCtx(t)
			client := uc.GetActiveClient(ctx)

			if tt.wantNil {
				assert.Nil(t, client)
			} else {
				require.NotNil(t, client)
				assert.Equal(t, tt.wantID, client.ID)
			}
		})
	}
}

func TestUseCaseImpl_IsControllingLocal(t *testing.T) {
	tests := []struct {
		name       string
		setupState func(*UseCaseImpl)
		want       bool
	}{
		{
			name:       "default is controlling local",
			setupState: func(uc *UseCaseImpl) {},
			want:       true,
		},
		{
			name: "not controlling local when active client",
			setupState: func(uc *UseCaseImpl) {
				uc.controllingLocal = false
				uc.activeClientID = "client1"
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, _, _, _ := newTestServerUseCase(t)
			tt.setupState(uc)

			ctx := testCtx(t)
			got := uc.IsControllingLocal(ctx)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUseCaseImpl_SwitchToClient(t *testing.T) {
	tests := []struct {
		name          string
		setupState    func(*UseCaseImpl)
		clientID      string
		setupMocks    func(*mocks.MockInputCapturePort, *mocks.MockNetworkServerPort)
		expectedError error
		wantActive    string
		wantLocal     bool
	}{
		{
			name: "switch to existing client",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["client1"] = &domain.Client{
					ID:      "client1",
					Name:    "Client1",
					Address: "192.168.1.100:52525",
				}
			},
			clientID: "client1",
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				ic.On("SetTarget", "client1").Return(nil)
				ns.On("SendEventToClient", mock.Anything, "192.168.1.100:52525", mock.Anything).Return(nil)
			},
			expectedError: nil,
			wantActive:    "client1",
			wantLocal:     false,
		},
		{
			name: "switch to non-existent client fails",
			setupState: func(uc *UseCaseImpl) {
				// No clients
			},
			clientID: "nonexistent",
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				// No calls expected
			},
			expectedError: domain.ErrClientNotFound,
			wantActive:    "",
			wantLocal:     true,
		},
		{
			name: "switch to already active client is noop",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["client1"] = &domain.Client{
					ID:      "client1",
					Name:    "Client1",
					Address: "192.168.1.100:52525",
				}
				uc.activeClientID = "client1"
				uc.controllingLocal = false
			},
			clientID: "client1",
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				// No calls expected - already controlling this client
			},
			expectedError: nil,
			wantActive:    "client1",
			wantLocal:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, inputCapture, network, _ := newTestServerUseCase(t)
			tt.setupState(uc)
			tt.setupMocks(inputCapture, network)

			ctx := testCtx(t)
			err := uc.SwitchToClient(ctx, tt.clientID)

			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantActive, uc.activeClientID)
			assert.Equal(t, tt.wantLocal, uc.controllingLocal)
		})
	}
}

func TestUseCaseImpl_SwitchToLocal(t *testing.T) {
	tests := []struct {
		name       string
		setupState func(*UseCaseImpl)
		setupMocks func(*mocks.MockInputCapturePort, *mocks.MockNetworkServerPort)
	}{
		{
			name: "switch to local from client",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["client1"] = &domain.Client{
					ID:      "client1",
					Name:    "Client1",
					Address: "192.168.1.100:52525",
				}
				uc.activeClientID = "client1"
				uc.controllingLocal = false
			},
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				ic.On("SetTarget", "").Return(nil)
				ns.On("SendEventToClient", mock.Anything, "192.168.1.100:52525", mock.Anything).Return(nil)
			},
		},
		{
			name: "switch to local when already local is noop",
			setupState: func(uc *UseCaseImpl) {
				// Already controlling local
			},
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				// No calls expected
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, inputCapture, network, _ := newTestServerUseCase(t)
			tt.setupState(uc)
			tt.setupMocks(inputCapture, network)

			ctx := testCtx(t)
			err := uc.SwitchToLocal(ctx)

			assert.NoError(t, err)
			assert.True(t, uc.controllingLocal)
			assert.Empty(t, uc.activeClientID)
		})
	}
}

func TestUseCaseImpl_SwitchToNext(t *testing.T) {
	tests := []struct {
		name       string
		setupState func(*UseCaseImpl)
		setupMocks func(*mocks.MockInputCapturePort, *mocks.MockNetworkServerPort)
		wantActive string
		wantLocal  bool
	}{
		{
			name: "from local to first client",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["client1"] = &domain.Client{ID: "client1", Name: "C1", Address: "addr1"}
			},
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				ic.On("SetTarget", "client1").Return(nil)
				ns.On("SendEventToClient", mock.Anything, "addr1", mock.Anything).Return(nil)
			},
			wantActive: "client1",
			wantLocal:  false,
		},
		{
			name: "from client to next client",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["aaa"] = &domain.Client{ID: "aaa", Name: "C1", Address: "addr1"}
				uc.clients["bbb"] = &domain.Client{ID: "bbb", Name: "C2", Address: "addr2"}
				uc.activeClientID = "aaa"
				uc.controllingLocal = false
			},
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				ic.On("SetTarget", "bbb").Return(nil)
				ns.On("SendEventToClient", mock.Anything, "addr2", mock.Anything).Return(nil)
			},
			wantActive: "bbb",
			wantLocal:  false,
		},
		{
			name: "from last client to local",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["client1"] = &domain.Client{ID: "client1", Name: "C1", Address: "addr1"}
				uc.activeClientID = "client1"
				uc.controllingLocal = false
			},
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				ic.On("SetTarget", "").Return(nil)
				ns.On("SendEventToClient", mock.Anything, "addr1", mock.Anything).Return(nil)
			},
			wantActive: "",
			wantLocal:  true,
		},
		{
			name:       "no clients stays local",
			setupState: func(uc *UseCaseImpl) {},
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				// No calls - already local, no clients
			},
			wantActive: "",
			wantLocal:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, inputCapture, network, _ := newTestServerUseCase(t)
			tt.setupState(uc)
			tt.setupMocks(inputCapture, network)

			ctx := testCtx(t)
			err := uc.SwitchToNext(ctx)

			assert.NoError(t, err)
			assert.Equal(t, tt.wantActive, uc.activeClientID)
			assert.Equal(t, tt.wantLocal, uc.controllingLocal)
		})
	}
}

func TestUseCaseImpl_SwitchToPrevious(t *testing.T) {
	tests := []struct {
		name       string
		setupState func(*UseCaseImpl)
		setupMocks func(*mocks.MockInputCapturePort, *mocks.MockNetworkServerPort)
		wantActive string
		wantLocal  bool
	}{
		{
			name: "from local to last client",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["aaa"] = &domain.Client{ID: "aaa", Name: "C1", Address: "addr1"}
				uc.clients["bbb"] = &domain.Client{ID: "bbb", Name: "C2", Address: "addr2"}
			},
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				ic.On("SetTarget", "bbb").Return(nil)
				ns.On("SendEventToClient", mock.Anything, "addr2", mock.Anything).Return(nil)
			},
			wantActive: "bbb",
			wantLocal:  false,
		},
		{
			name: "from first client to local",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["aaa"] = &domain.Client{ID: "aaa", Name: "C1", Address: "addr1"}
				uc.clients["bbb"] = &domain.Client{ID: "bbb", Name: "C2", Address: "addr2"}
				uc.activeClientID = "aaa"
				uc.controllingLocal = false
			},
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				ic.On("SetTarget", "").Return(nil)
				ns.On("SendEventToClient", mock.Anything, "addr1", mock.Anything).Return(nil)
			},
			wantActive: "",
			wantLocal:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, inputCapture, network, _ := newTestServerUseCase(t)
			tt.setupState(uc)
			tt.setupMocks(inputCapture, network)

			ctx := testCtx(t)
			err := uc.SwitchToPrevious(ctx)

			assert.NoError(t, err)
			assert.Equal(t, tt.wantActive, uc.activeClientID)
			assert.Equal(t, tt.wantLocal, uc.controllingLocal)
		})
	}
}

func TestUseCaseImpl_ConnectToSlot(t *testing.T) {
	tests := []struct {
		name          string
		setupState    func(*UseCaseImpl)
		slot          int32
		setupMocks    func(*mocks.MockInputCapturePort, *mocks.MockNetworkServerPort)
		expectedError error
		wantLocal     bool
	}{
		{
			name: "slot 0 switches to local",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["client1"] = &domain.Client{ID: "client1", Address: "addr1"}
				uc.activeClientID = "client1"
				uc.controllingLocal = false
			},
			slot: 0,
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				ic.On("SetTarget", "").Return(nil)
				ns.On("SendEventToClient", mock.Anything, "addr1", mock.Anything).Return(nil)
			},
			expectedError: nil,
			wantLocal:     true,
		},
		{
			name: "slot 1 switches to first client",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["client1"] = &domain.Client{ID: "client1", Address: "addr1"}
			},
			slot: 1,
			setupMocks: func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {
				ic.On("SetTarget", "client1").Return(nil)
				ns.On("SendEventToClient", mock.Anything, "addr1", mock.Anything).Return(nil)
			},
			expectedError: nil,
			wantLocal:     false,
		},
		{
			name:          "invalid slot returns error",
			setupState:    func(uc *UseCaseImpl) {},
			slot:          6,
			setupMocks:    func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {},
			expectedError: domain.ErrInvalidSlot,
			wantLocal:     true,
		},
		{
			name: "slot with no client returns error",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["client1"] = &domain.Client{ID: "client1", Address: "addr1"}
			},
			slot:          3, // No client in slot 3
			setupMocks:    func(ic *mocks.MockInputCapturePort, ns *mocks.MockNetworkServerPort) {},
			expectedError: domain.ErrClientNotFound,
			wantLocal:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, inputCapture, network, _ := newTestServerUseCase(t)
			tt.setupState(uc)
			tt.setupMocks(inputCapture, network)

			ctx := testCtx(t)
			err := uc.ConnectToSlot(ctx, tt.slot)

			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantLocal, uc.controllingLocal)
		})
	}
}

func TestUseCaseImpl_UpdateClientConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		setupState func(*UseCaseImpl)
		config     *domain.ClientConfig
		sourceID   string
		wantName   string
	}{
		{
			name: "update existing client by ID",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["client1"] = &domain.Client{ID: "client1", Name: "OldName", Address: "addr1"}
			},
			config: &domain.ClientConfig{
				ClientID:   "client1",
				ClientName: "NewName",
				Monitors: []domain.Monitor{
					{Name: "Monitor1", Width: 1920, Height: 1080},
				},
			},
			sourceID: "client1",
			wantName: "NewName",
		},
		{
			name: "update client found by source ID",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["addr1"] = &domain.Client{ID: "addr1", Name: "addr1", Address: "addr1"}
			},
			config: &domain.ClientConfig{
				ClientID:   "different",
				ClientName: "NewName",
			},
			sourceID: "addr1",
			wantName: "NewName",
		},
		{
			name: "update single client even with mismatched IDs",
			setupState: func(uc *UseCaseImpl) {
				uc.clients["only-client"] = &domain.Client{ID: "only-client", Name: "OldName", Address: "addr1"}
			},
			config: &domain.ClientConfig{
				ClientID:   "different-id",
				ClientName: "NewName",
			},
			sourceID: "unknown-source",
			wantName: "NewName",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, _, _, _ := newTestServerUseCase(t)
			tt.setupState(uc)

			ctx := testCtx(t)
			uc.UpdateClientConfiguration(ctx, tt.config, tt.sourceID)

			// Verify the name was updated for some client
			found := false
			for _, client := range uc.clients {
				if client.Name == tt.wantName {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected to find client with name %s", tt.wantName)
		})
	}
}

func TestUseCaseImpl_SetOnActivity(t *testing.T) {
	uc, _, _, _ := newTestServerUseCase(t)

	var receivedLevel, receivedMessage string
	callback := func(level, message string) {
		receivedLevel = level
		receivedMessage = message
	}

	uc.SetOnActivity(callback)
	uc.notifyActivity("INFO", "test message")

	assert.Equal(t, "INFO", receivedLevel)
	assert.Equal(t, "test message", receivedMessage)
}

func TestUseCaseImpl_GetSSHPaths(t *testing.T) {
	uc, _, _, _ := newTestServerUseCase(t)
	uc.sshHostKeyPath = "/path/to/host/key"
	uc.sshAuthKeyPath = "/path/to/auth/keys"

	assert.Equal(t, "/path/to/host/key", uc.GetSSHHostKeyPath())
	assert.Equal(t, "/path/to/auth/keys", uc.GetSSHAuthKeysPath())
}

func TestUseCaseImpl_MarkEmergencyRelease(t *testing.T) {
	uc, _, _, _ := newTestServerUseCase(t)

	before := time.Now()
	ctx := testCtx(t)
	uc.MarkEmergencyRelease(ctx)
	after := time.Now()

	assert.True(t, uc.emergencyReleaseTime.After(before) || uc.emergencyReleaseTime.Equal(before))
	assert.True(t, uc.emergencyReleaseTime.Before(after) || uc.emergencyReleaseTime.Equal(after))
}
