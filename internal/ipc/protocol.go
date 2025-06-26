package ipc

import (
	"fmt"

	pb "github.com/bnema/waymon/internal/proto"
)

// NewSwitchMessage creates a new switch message with rotation action
func NewSwitchMessage(action pb.SwitchAction) (*pb.IPCMessage, error) {
	cmd := &pb.SwitchCommand{
		Action: action,
	}

	return &pb.IPCMessage{
		Type: pb.IPCMessageType_IPC_MESSAGE_TYPE_SWITCH,
		Payload: &pb.IPCMessage_SwitchCommand{
			SwitchCommand: cmd,
		},
	}, nil
}

// NewSwitchMessageLegacy creates a switch message using the legacy enable/disable pattern
func NewSwitchMessageLegacy(enable *bool) (*pb.IPCMessage, error) {
	cmd := &pb.SwitchCommand{}
	if enable != nil {
		cmd.Enable = enable
		if *enable {
			cmd.Action = pb.SwitchAction_SWITCH_ACTION_ENABLE
		} else {
			cmd.Action = pb.SwitchAction_SWITCH_ACTION_DISABLE
		}
	} else {
		// Default to "next" for toggle behavior
		cmd.Action = pb.SwitchAction_SWITCH_ACTION_NEXT
	}

	return &pb.IPCMessage{
		Type: pb.IPCMessageType_IPC_MESSAGE_TYPE_SWITCH,
		Payload: &pb.IPCMessage_SwitchCommand{
			SwitchCommand: cmd,
		},
	}, nil
}

// NewStatusMessage creates a new status query message
func NewStatusMessage() (*pb.IPCMessage, error) {
	return &pb.IPCMessage{
		Type: pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS,
		Payload: &pb.IPCMessage_StatusQuery{
			StatusQuery: &pb.StatusQuery{},
		},
	}, nil
}

// NewStatusResponseMessage creates a new status response message
func NewStatusResponseMessage(active, connected bool, serverHost string, currentComputer, totalComputers int32, computerNames []string) (*pb.IPCMessage, error) {
	return &pb.IPCMessage{
		Type: pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS_RESPONSE,
		Payload: &pb.IPCMessage_StatusResponse{
			StatusResponse: &pb.StatusResponse{
				Active:          active,
				Connected:       connected,
				ServerHost:      serverHost,
				CurrentComputer: currentComputer,
				TotalComputers:  totalComputers,
				ComputerNames:   computerNames,
			},
		},
	}, nil
}

// NewStatusResponseMessageLegacy creates a status response with legacy fields only
func NewStatusResponseMessageLegacy(active, connected bool, serverHost string) (*pb.IPCMessage, error) {
	return NewStatusResponseMessage(active, connected, serverHost, 0, 1, []string{"server"})
}

// NewErrorMessage creates a new error message
func NewErrorMessage(errMsg string) (*pb.IPCMessage, error) {
	return &pb.IPCMessage{
		Type: pb.IPCMessageType_IPC_MESSAGE_TYPE_ERROR,
		Payload: &pb.IPCMessage_ErrorResponse{
			ErrorResponse: &pb.ErrorResponse{
				Error: errMsg,
			},
		},
	}, nil
}

// GetSwitchCommand extracts switch command from message
func GetSwitchCommand(msg *pb.IPCMessage) (*pb.SwitchCommand, error) {
	if msg.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_SWITCH {
		return nil, fmt.Errorf("message is not a switch command")
	}

	cmd, ok := msg.Payload.(*pb.IPCMessage_SwitchCommand)
	if !ok {
		return nil, fmt.Errorf("invalid switch command payload")
	}

	return cmd.SwitchCommand, nil
}

// GetStatusQuery extracts status query from message
func GetStatusQuery(msg *pb.IPCMessage) (*pb.StatusQuery, error) {
	if msg.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS {
		return nil, fmt.Errorf("message is not a status query")
	}

	query, ok := msg.Payload.(*pb.IPCMessage_StatusQuery)
	if !ok {
		return nil, fmt.Errorf("invalid status query payload")
	}

	return query.StatusQuery, nil
}

// GetStatusResponse extracts status response from message
func GetStatusResponse(msg *pb.IPCMessage) (*pb.StatusResponse, error) {
	if msg.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS_RESPONSE {
		return nil, fmt.Errorf("message is not a status response")
	}

	resp, ok := msg.Payload.(*pb.IPCMessage_StatusResponse)
	if !ok {
		return nil, fmt.Errorf("invalid status response payload")
	}

	return resp.StatusResponse, nil
}

// GetErrorResponse extracts error response from message
func GetErrorResponse(msg *pb.IPCMessage) (*pb.ErrorResponse, error) {
	if msg.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_ERROR {
		return nil, fmt.Errorf("message is not an error response")
	}

	errResp, ok := msg.Payload.(*pb.IPCMessage_ErrorResponse)
	if !ok {
		return nil, fmt.Errorf("invalid error response payload")
	}

	return errResp.ErrorResponse, nil
}

// NewReleaseMessage creates a new release command message
func NewReleaseMessage() (*pb.IPCMessage, error) {
	return &pb.IPCMessage{
		Type: pb.IPCMessageType_IPC_MESSAGE_TYPE_RELEASE,
		Payload: &pb.IPCMessage_ReleaseCommand{
			ReleaseCommand: &pb.ReleaseCommand{},
		},
	}, nil
}

// NewConnectMessage creates a new connect command message
func NewConnectMessage(slot int32) (*pb.IPCMessage, error) {
	return &pb.IPCMessage{
		Type: pb.IPCMessageType_IPC_MESSAGE_TYPE_CONNECT,
		Payload: &pb.IPCMessage_ConnectCommand{
			ConnectCommand: &pb.ConnectCommand{
				Slot: slot,
			},
		},
	}, nil
}

// GetReleaseCommand extracts release command from message
func GetReleaseCommand(msg *pb.IPCMessage) (*pb.ReleaseCommand, error) {
	if msg.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_RELEASE {
		return nil, fmt.Errorf("message is not a release command")
	}

	cmd, ok := msg.Payload.(*pb.IPCMessage_ReleaseCommand)
	if !ok {
		return nil, fmt.Errorf("invalid release command payload")
	}

	return cmd.ReleaseCommand, nil
}

// GetConnectCommand extracts connect command from message
func GetConnectCommand(msg *pb.IPCMessage) (*pb.ConnectCommand, error) {
	if msg.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_CONNECT {
		return nil, fmt.Errorf("message is not a connect command")
	}

	cmd, ok := msg.Payload.(*pb.IPCMessage_ConnectCommand)
	if !ok {
		return nil, fmt.Errorf("invalid connect command payload")
	}

	return cmd.ConnectCommand, nil
}
