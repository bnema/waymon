package cli

import (
	"github.com/bnema/waymon/internal/adapters/in/ipc"
	"github.com/bnema/waymon/internal/boundaries/in"
)

// IPCClientAdapter wraps the IPC client to implement the IPCClient interface.
// This adapter converts between the IPC client's types and the CLI's interface types.
type IPCClientAdapter struct {
	client *ipc.Client
}

// NewIPCClientAdapter creates a new IPC client adapter.
func NewIPCClientAdapter(client *ipc.Client) *IPCClientAdapter {
	return &IPCClientAdapter{client: client}
}

// SendSwitch sends a switch command.
func (a *IPCClientAdapter) SendSwitch(action SwitchAction) (*StatusResponse, error) {
	// Convert CLI SwitchAction to boundaries SwitchAction
	var boundaryAction in.SwitchAction
	switch action {
	case SwitchActionNext:
		boundaryAction = in.SwitchActionNext
	case SwitchActionPrevious:
		boundaryAction = in.SwitchActionPrevious
	case SwitchActionEnable:
		boundaryAction = in.SwitchActionEnable
	case SwitchActionDisable:
		boundaryAction = in.SwitchActionDisable
	default:
		boundaryAction = in.SwitchActionNext
	}

	resp, err := a.client.SendSwitch(boundaryAction)
	if err != nil {
		return nil, err
	}

	return convertStatusResponse(resp), nil
}

// SendStatus sends a status query.
func (a *IPCClientAdapter) SendStatus() (*StatusResponse, error) {
	resp, err := a.client.SendStatus()
	if err != nil {
		return nil, err
	}

	return convertStatusResponse(resp), nil
}

// SendRelease sends a release command.
func (a *IPCClientAdapter) SendRelease() (*StatusResponse, error) {
	resp, err := a.client.SendRelease()
	if err != nil {
		return nil, err
	}

	return convertStatusResponse(resp), nil
}

// SendConnect sends a connect command.
func (a *IPCClientAdapter) SendConnect(slot int32) (*StatusResponse, error) {
	resp, err := a.client.SendConnect(slot)
	if err != nil {
		return nil, err
	}

	return convertStatusResponse(resp), nil
}

// SendStop sends a stop command.
func (a *IPCClientAdapter) SendStop() error {
	return a.client.SendStop()
}

// IsRunning checks if waymon is running.
func (a *IPCClientAdapter) IsRunning() bool {
	return a.client.IsRunning()
}

// convertStatusResponse converts from boundaries StatusResponse to CLI StatusResponse.
func convertStatusResponse(resp *in.StatusResponse) *StatusResponse {
	if resp == nil {
		return nil
	}

	return &StatusResponse{
		Active:        resp.Active,
		Connected:     resp.Connected,
		ServerHost:    resp.ServerHost,
		CurrentIndex:  resp.CurrentIndex,
		TotalCount:    resp.TotalCount,
		ComputerNames: resp.ComputerNames,
	}
}
