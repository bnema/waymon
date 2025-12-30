package in

import "context"

// SwitchAction represents an action for the switch command.
type SwitchAction int

const (
	// SwitchActionNext switches to the next client.
	SwitchActionNext SwitchAction = iota
	// SwitchActionPrevious switches to the previous client.
	SwitchActionPrevious
	// SwitchActionEnable enables mouse sharing (legacy).
	SwitchActionEnable
	// SwitchActionDisable disables mouse sharing (legacy).
	SwitchActionDisable
)

// StatusResponse contains the response to a status query.
type StatusResponse struct {
	Active        bool
	Connected     bool
	ServerHost    string
	CurrentIndex  int32
	TotalCount    int32
	ComputerNames []string
}

// IPCHandler defines the interface for handling IPC commands.
// Implemented by the server usecase to handle CLI commands via Unix socket.
type IPCHandler interface {
	// HandleSwitch handles a switch command (next/previous/enable/disable).
	HandleSwitch(ctx context.Context, action SwitchAction) (*StatusResponse, error)

	// HandleStatus handles a status query.
	HandleStatus(ctx context.Context) (*StatusResponse, error)

	// HandleRelease handles a release command (switch to local).
	HandleRelease(ctx context.Context) (*StatusResponse, error)

	// HandleConnect handles a connect command (switch to specific slot).
	HandleConnect(ctx context.Context, slot int32) (*StatusResponse, error)
}
