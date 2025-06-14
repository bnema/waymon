package ipc

import (
	"testing"

	pb "github.com/bnema/waymon/internal/proto"
)

func TestNewSwitchMessage(t *testing.T) {
	tests := []struct {
		name   string
		action pb.SwitchAction
	}{
		{
			name:   "switch next",
			action: pb.SwitchAction_SWITCH_ACTION_NEXT,
		},
		{
			name:   "switch previous",
			action: pb.SwitchAction_SWITCH_ACTION_PREVIOUS,
		},
		{
			name:   "enable switch",
			action: pb.SwitchAction_SWITCH_ACTION_ENABLE,
		},
		{
			name:   "disable switch",
			action: pb.SwitchAction_SWITCH_ACTION_DISABLE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := NewSwitchMessage(tt.action)
			if err != nil {
				t.Fatalf("NewSwitchMessage() error = %v", err)
			}

			if msg.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_SWITCH {
				t.Errorf("Expected message type %s, got %s", pb.IPCMessageType_IPC_MESSAGE_TYPE_SWITCH, msg.Type)
			}

			// Parse back the command
			cmd, err := GetSwitchCommand(msg)
			if err != nil {
				t.Fatalf("GetSwitchCommand() error = %v", err)
			}

			if cmd.Action != tt.action {
				t.Errorf("Expected Action to be %v, got %v", tt.action, cmd.Action)
			}
		})
	}
}

func TestNewStatusMessage(t *testing.T) {
	msg, err := NewStatusMessage()
	if err != nil {
		t.Fatalf("NewStatusMessage() error = %v", err)
	}

	if msg.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS {
		t.Errorf("Expected message type %s, got %s", pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS, msg.Type)
	}

	// Parse back the query
	_, err = GetStatusQuery(msg)
	if err != nil {
		t.Fatalf("GetStatusQuery() error = %v", err)
	}
}

func TestNewStatusResponseMessage(t *testing.T) {
	tests := []struct {
		name       string
		active     bool
		connected  bool
		serverHost string
	}{
		{
			name:       "active and connected",
			active:     true,
			connected:  true,
			serverHost: "server.local:52525",
		},
		{
			name:       "inactive and disconnected",
			active:     false,
			connected:  false,
			serverHost: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := NewStatusResponseMessageLegacy(tt.active, tt.connected, tt.serverHost)
			if err != nil {
				t.Fatalf("NewStatusResponseMessage() error = %v", err)
			}

			if msg.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS_RESPONSE {
				t.Errorf("Expected message type %s, got %s", pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS_RESPONSE, msg.Type)
			}

			// Parse back the response
			resp, err := GetStatusResponse(msg)
			if err != nil {
				t.Fatalf("GetStatusResponse() error = %v", err)
			}

			if resp.Active != tt.active {
				t.Errorf("Expected Active to be %v, got %v", tt.active, resp.Active)
			}
			if resp.Connected != tt.connected {
				t.Errorf("Expected Connected to be %v, got %v", tt.connected, resp.Connected)
			}
			if resp.ServerHost != tt.serverHost {
				t.Errorf("Expected ServerHost to be %s, got %s", tt.serverHost, resp.ServerHost)
			}
		})
	}
}

func TestNewErrorMessage(t *testing.T) {
	errMsg := "test error message"
	msg, err := NewErrorMessage(errMsg)
	if err != nil {
		t.Fatalf("NewErrorMessage() error = %v", err)
	}

	if msg.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_ERROR {
		t.Errorf("Expected message type %s, got %s", pb.IPCMessageType_IPC_MESSAGE_TYPE_ERROR, msg.Type)
	}

	// Parse back the error
	errResp, err := GetErrorResponse(msg)
	if err != nil {
		t.Fatalf("GetErrorResponse() error = %v", err)
	}

	if errResp.Error != errMsg {
		t.Errorf("Expected Error to be %s, got %s", errMsg, errResp.Error)
	}
}

func TestGetSwitchCommandWrongType(t *testing.T) {
	msg, _ := NewStatusMessage()
	
	_, err := GetSwitchCommand(msg)
	if err == nil {
		t.Error("Expected error when parsing status message as switch command")
	}
}

func TestGetStatusQueryWrongType(t *testing.T) {
	msg, _ := NewSwitchMessage(pb.SwitchAction_SWITCH_ACTION_NEXT)
	
	_, err := GetStatusQuery(msg)
	if err == nil {
		t.Error("Expected error when parsing switch message as status query")
	}
}

func TestGetStatusResponseWrongType(t *testing.T) {
	msg, _ := NewSwitchMessage(pb.SwitchAction_SWITCH_ACTION_NEXT)
	
	_, err := GetStatusResponse(msg)
	if err == nil {
		t.Error("Expected error when parsing switch message as status response")
	}
}

func TestGetErrorResponseWrongType(t *testing.T) {
	msg, _ := NewSwitchMessage(pb.SwitchAction_SWITCH_ACTION_NEXT)
	
	_, err := GetErrorResponse(msg)
	if err == nil {
		t.Error("Expected error when parsing switch message as error response")
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}