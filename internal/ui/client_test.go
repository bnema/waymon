package ui

import (
	"testing"
	"time"

	"github.com/bnema/waymon/internal/client"
)

func TestClientUIControlStatusUpdate(t *testing.T) {
	// Create a mock input receiver
	inputReceiver, err := client.NewInputReceiver("test-server:52525")
	if err != nil {
		t.Fatalf("Failed to create input receiver: %v", err)
	}

	// Create client model
	model := NewClientModel("test-server:52525", inputReceiver, "test-version")

	// Test initial state
	if model.controlStatus.BeingControlled {
		t.Error("Expected initial control status to be false")
	}

	// Test handling control status message - being controlled
	controlledStatus := client.ControlStatus{
		BeingControlled: true,
		ControllerName:  "test-server",
		ControllerID:    "server-id",
		ConnectedAt:     time.Now().Unix(),
	}

	updatedModel, cmd := model.Update(ControlStatusMsg{Status: controlledStatus})
	clientModel := updatedModel.(*ClientModel)

	// Verify status was updated
	if !clientModel.controlStatus.BeingControlled {
		t.Error("Expected control status to be updated to being controlled")
	}
	if clientModel.controlStatus.ControllerName != "test-server" {
		t.Errorf("Expected controller name to be 'test-server', got '%s'", clientModel.controlStatus.ControllerName)
	}

	// Verify that a command was returned to trigger UI update
	if cmd == nil {
		t.Error("Expected a command to be returned to trigger UI update")
	}

	// Test handling control release
	releasedStatus := client.ControlStatus{
		BeingControlled: false,
		ControllerName:  "",
		ControllerID:    "",
		ConnectedAt:     0,
	}

	updatedModel2, cmd2 := clientModel.Update(ControlStatusMsg{Status: releasedStatus})
	clientModel2 := updatedModel2.(*ClientModel)

	// Verify status was updated
	if clientModel2.controlStatus.BeingControlled {
		t.Error("Expected control status to be updated to not being controlled")
	}
	if clientModel2.controlStatus.ControllerName != "" {
		t.Errorf("Expected controller name to be empty, got '%s'", clientModel2.controlStatus.ControllerName)
	}

	// Verify that a command was returned to trigger UI update
	if cmd2 == nil {
		t.Error("Expected a command to be returned to trigger UI update on release")
	}
}

// TestClientUIRenderControlStatus tests that the control status is rendered correctly
func TestClientUIRenderControlStatus(t *testing.T) {
	// Create a mock input receiver
	inputReceiver, err := client.NewInputReceiver("test-server:52525")
	if err != nil {
		t.Fatalf("Failed to create input receiver: %v", err)
	}

	// Create client model
	model := NewClientModel("test-server:52525", inputReceiver, "test-version")
	model.connected = true

	// Test rendering when idle
	view := model.renderControlStatus()
	if !contains(view, "Idle - Waiting for server control") {
		t.Error("Expected idle status to be shown")
	}

	// Test rendering when being controlled
	model.controlStatus = client.ControlStatus{
		BeingControlled: true,
		ControllerName:  "test-server",
		ControllerID:    "server-id",
		ConnectedAt:     time.Now().Unix(),
	}
	view = model.renderControlStatus()
	if !contains(view, "BEING CONTROLLED BY test-server") {
		t.Error("Expected being controlled status to be shown")
	}
	if !contains(view, "[r] Release control") {
		t.Error("Expected release control option to be shown")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:] != "" && len(substr) > 0 && len(s) > 0 && ContainsSubstring(s, substr)
}

func ContainsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
