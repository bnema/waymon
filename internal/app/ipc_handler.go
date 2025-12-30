package app

import (
	"context"
	"os"

	"github.com/bnema/waymon/internal/boundaries/in"
)

// Compile-time interface check
var _ in.IPCHandler = (*IPCHandlerAdapter)(nil)

// IPCHandlerAdapter adapts the server use case to the IPCHandler interface.
// This bridges the gap between the use case layer and the IPC adapter,
// converting between use case types and boundary types.
type IPCHandlerAdapter struct {
	useCase in.ServerUseCase
}

// NewIPCHandlerAdapter creates a new IPC handler adapter.
func NewIPCHandlerAdapter(useCase in.ServerUseCase) *IPCHandlerAdapter {
	return &IPCHandlerAdapter{useCase: useCase}
}

// HandleSwitch handles a switch command (next/previous/enable/disable).
func (a *IPCHandlerAdapter) HandleSwitch(ctx context.Context, action in.SwitchAction) (*in.StatusResponse, error) {
	var err error

	switch action {
	case in.SwitchActionNext:
		err = a.useCase.SwitchToNext(ctx)
	case in.SwitchActionPrevious:
		err = a.useCase.SwitchToPrevious(ctx)
	case in.SwitchActionEnable:
		// Legacy - switch to first available client
		clients := a.useCase.GetConnectedClients(ctx)
		if len(clients) > 0 {
			err = a.useCase.SwitchToClient(ctx, clients[0].ID)
		}
	case in.SwitchActionDisable:
		// Legacy - switch to local
		err = a.useCase.SwitchToLocal(ctx)
	}

	if err != nil {
		return nil, err
	}

	return a.HandleStatus(ctx)
}

// HandleStatus handles a status query.
func (a *IPCHandlerAdapter) HandleStatus(ctx context.Context) (*in.StatusResponse, error) {
	clients := a.useCase.GetConnectedClients(ctx)
	activeClient := a.useCase.GetActiveClient(ctx)
	isLocal := a.useCase.IsControllingLocal(ctx)

	clientNames := make([]string, len(clients))
	for i, c := range clients {
		clientNames[i] = c.Name
	}

	// Calculate current index (0 = local, 1+ = clients)
	var currentIndex int32 = 0
	if !isLocal && activeClient != nil {
		for i, c := range clients {
			if c.ID == activeClient.ID {
				currentIndex = int32(i + 1) // 1-based for clients
				break
			}
		}
	}

	return &in.StatusResponse{
		Active:        true, // Server is running if we can respond
		Connected:     len(clients) > 0,
		ServerHost:    getHostname(),
		CurrentIndex:  currentIndex,
		TotalCount:    int32(len(clients) + 1), // +1 for local
		ComputerNames: clientNames,
	}, nil
}

// HandleRelease handles a release command (switch to local).
func (a *IPCHandlerAdapter) HandleRelease(ctx context.Context) (*in.StatusResponse, error) {
	if err := a.useCase.SwitchToLocal(ctx); err != nil {
		return nil, err
	}
	return a.HandleStatus(ctx)
}

// HandleConnect handles a connect command (switch to specific slot).
func (a *IPCHandlerAdapter) HandleConnect(ctx context.Context, slot int32) (*in.StatusResponse, error) {
	if err := a.useCase.ConnectToSlot(ctx, slot); err != nil {
		return nil, err
	}
	return a.HandleStatus(ctx)
}

// getHostname returns the server hostname.
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "waymon-server"
	}
	return hostname
}
