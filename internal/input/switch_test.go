package input

import (
	"testing"

	pb "github.com/bnema/waymon/internal/proto"
)

func TestSwitchManager(t *testing.T) {
	t.Run("creates new switch manager with default state", func(t *testing.T) {
		sm := NewSwitchManager()

		if len(sm.computers) != 1 {
			t.Errorf("Expected 1 computer, got %d", len(sm.computers))
		}

		if sm.computers[0] != "server" {
			t.Errorf("Expected server as first computer, got %s", sm.computers[0])
		}

		if sm.currentComputer != 0 {
			t.Errorf("Expected current computer to be 0, got %d", sm.currentComputer)
		}

		if sm.active {
			t.Error("Expected active to be false")
		}

		if sm.connected {
			t.Error("Expected connected to be false")
		}
	})

	t.Run("adds and removes computers", func(t *testing.T) {
		sm := NewSwitchManager()

		// Add a computer
		sm.AddComputer("client1")
		if len(sm.computers) != 2 {
			t.Errorf("Expected 2 computers, got %d", len(sm.computers))
		}

		// Add duplicate computer (should be ignored)
		sm.AddComputer("client1")
		if len(sm.computers) != 2 {
			t.Errorf("Expected 2 computers after duplicate add, got %d", len(sm.computers))
		}

		// Add another computer
		sm.AddComputer("client2")
		if len(sm.computers) != 3 {
			t.Errorf("Expected 3 computers, got %d", len(sm.computers))
		}

		// Remove a computer
		sm.RemoveComputer("client1")
		if len(sm.computers) != 2 {
			t.Errorf("Expected 2 computers after removal, got %d", len(sm.computers))
		}

		// Check remaining computers
		expected := []string{"server", "client2"}
		for i, computer := range sm.computers {
			if computer != expected[i] {
				t.Errorf("Expected computer %d to be %s, got %s", i, expected[i], computer)
			}
		}
	})

	t.Run("switches to next computer", func(t *testing.T) {
		sm := NewSwitchManager()
		sm.AddComputer("client1")
		sm.AddComputer("client2")

		// Should start at server (index 0)
		if sm.currentComputer != 0 {
			t.Errorf("Expected to start at computer 0, got %d", sm.currentComputer)
		}

		// Switch to next (client1)
		err := sm.SwitchNext()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if sm.currentComputer != 1 {
			t.Errorf("Expected current computer to be 1, got %d", sm.currentComputer)
		}

		// Switch to next (client2)
		err = sm.SwitchNext()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if sm.currentComputer != 2 {
			t.Errorf("Expected current computer to be 2, got %d", sm.currentComputer)
		}

		// Switch to next (should wrap to server)
		err = sm.SwitchNext()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if sm.currentComputer != 0 {
			t.Errorf("Expected current computer to wrap to 0, got %d", sm.currentComputer)
		}
	})

	t.Run("switches to previous computer", func(t *testing.T) {
		sm := NewSwitchManager()
		sm.AddComputer("client1")
		sm.AddComputer("client2")

		// Switch to previous (should wrap to client2)
		err := sm.SwitchPrevious()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if sm.currentComputer != 2 {
			t.Errorf("Expected current computer to wrap to 2, got %d", sm.currentComputer)
		}

		// Switch to previous (client1)
		err = sm.SwitchPrevious()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if sm.currentComputer != 1 {
			t.Errorf("Expected current computer to be 1, got %d", sm.currentComputer)
		}

		// Switch to previous (server)
		err = sm.SwitchPrevious()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if sm.currentComputer != 0 {
			t.Errorf("Expected current computer to be 0, got %d", sm.currentComputer)
		}
	})

	t.Run("fails to switch with only one computer", func(t *testing.T) {
		sm := NewSwitchManager()

		err := sm.SwitchNext()
		if err == nil {
			t.Error("Expected error when switching with only one computer")
		}

		err = sm.SwitchPrevious()
		if err == nil {
			t.Error("Expected error when switching with only one computer")
		}
	})

	t.Run("handles computer removal while current", func(t *testing.T) {
		sm := NewSwitchManager()
		sm.AddComputer("client1")
		sm.AddComputer("client2")

		// Switch to client1
		sm.SwitchNext()
		if sm.currentComputer != 1 {
			t.Errorf("Expected current computer to be 1, got %d", sm.currentComputer)
		}

		// Remove current computer (client1)
		sm.RemoveComputer("client1")

		// Should switch back to server
		if sm.currentComputer != 0 {
			t.Errorf("Expected current computer to reset to 0, got %d", sm.currentComputer)
		}
	})

	t.Run("adjusts index when computer before current is removed", func(t *testing.T) {
		sm := NewSwitchManager()
		sm.AddComputer("client1")
		sm.AddComputer("client2")

		// Switch to client2 (index 2)
		sm.SwitchNext() // to client1
		sm.SwitchNext() // to client2
		if sm.currentComputer != 2 {
			t.Errorf("Expected current computer to be 2, got %d", sm.currentComputer)
		}

		// Remove client1 (index 1)
		sm.RemoveComputer("client1")

		// Current index should be decremented to 1
		if sm.currentComputer != 1 {
			t.Errorf("Expected current computer to be decremented to 1, got %d", sm.currentComputer)
		}

		// Should still be pointing to client2
		if sm.CurrentComputerName() != "client2" {
			t.Errorf("Expected current computer name to be client2, got %s", sm.CurrentComputerName())
		}
	})
}

func TestIPCHandler(t *testing.T) {
	t.Run("handles switch next command", func(t *testing.T) {
		sm := NewSwitchManager()
		sm.AddComputer("client1")
		handler := NewIPCHandler(sm)

		cmd := &pb.SwitchCommand{
			Action: pb.SwitchAction_SWITCH_ACTION_NEXT,
		}

		resp, err := handler.HandleSwitchCommand(cmd)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if resp.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS_RESPONSE {
			t.Errorf("Expected status response, got %s", resp.Type)
		}

		// Should have switched to client1
		if sm.currentComputer != 1 {
			t.Errorf("Expected current computer to be 1, got %d", sm.currentComputer)
		}
	})

	t.Run("handles switch previous command", func(t *testing.T) {
		sm := NewSwitchManager()
		sm.AddComputer("client1")
		handler := NewIPCHandler(sm)

		cmd := &pb.SwitchCommand{
			Action: pb.SwitchAction_SWITCH_ACTION_PREVIOUS,
		}

		resp, err := handler.HandleSwitchCommand(cmd)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if resp.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS_RESPONSE {
			t.Errorf("Expected status response, got %s", resp.Type)
		}

		// Should have switched to client1 (wrapped)
		if sm.currentComputer != 1 {
			t.Errorf("Expected current computer to be 1, got %d", sm.currentComputer)
		}
	})

	t.Run("handles enable command", func(t *testing.T) {
		sm := NewSwitchManager()
		handler := NewIPCHandler(sm)

		cmd := &pb.SwitchCommand{
			Action: pb.SwitchAction_SWITCH_ACTION_ENABLE,
		}

		resp, err := handler.HandleSwitchCommand(cmd)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if resp.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS_RESPONSE {
			t.Errorf("Expected status response, got %s", resp.Type)
		}

		if !sm.active {
			t.Error("Expected active to be true")
		}
	})

	t.Run("handles disable command", func(t *testing.T) {
		sm := NewSwitchManager()
		sm.SetActiveState(true)
		handler := NewIPCHandler(sm)

		cmd := &pb.SwitchCommand{
			Action: pb.SwitchAction_SWITCH_ACTION_DISABLE,
		}

		resp, err := handler.HandleSwitchCommand(cmd)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if resp.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS_RESPONSE {
			t.Errorf("Expected status response, got %s", resp.Type)
		}

		if sm.active {
			t.Error("Expected active to be false")
		}
	})

	t.Run("handles unknown action", func(t *testing.T) {
		sm := NewSwitchManager()
		handler := NewIPCHandler(sm)

		cmd := &pb.SwitchCommand{
			Action: pb.SwitchAction_SWITCH_ACTION_UNSPECIFIED,
		}

		resp, err := handler.HandleSwitchCommand(cmd)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if resp.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_ERROR {
			t.Errorf("Expected error response, got %s", resp.Type)
		}
	})

	t.Run("handles status query", func(t *testing.T) {
		sm := NewSwitchManager()
		sm.AddComputer("client1")
		sm.SetActiveState(true)
		sm.SetConnectionState(true, "server.local:52525")
		handler := NewIPCHandler(sm)

		query := &pb.StatusQuery{}

		resp, err := handler.HandleStatusQuery(query)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if resp.Type != pb.IPCMessageType_IPC_MESSAGE_TYPE_STATUS_RESPONSE {
			t.Errorf("Expected status response, got %s", resp.Type)
		}
	})
}

func TestSwitchCallback(t *testing.T) {
	t.Run("calls callback on switch", func(t *testing.T) {
		sm := NewSwitchManager()
		sm.AddComputer("client1")

		var callbackCalled bool
		var callbackComputer int32
		var callbackActive bool

		sm.SetOnSwitchCallback(func(computer int32, active bool) {
			callbackCalled = true
			callbackComputer = computer
			callbackActive = active
		})

		sm.SetActiveState(true)
		err := sm.SwitchNext()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if !callbackCalled {
			t.Error("Expected callback to be called")
		}

		if callbackComputer != 1 {
			t.Errorf("Expected callback computer to be 1, got %d", callbackComputer)
		}

		if !callbackActive {
			t.Error("Expected callback active to be true")
		}
	})
}
