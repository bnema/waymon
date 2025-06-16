package input

import (
	"fmt"
	"sync"

	"github.com/bnema/waymon/internal/ipc"
	"github.com/bnema/waymon/internal/logger"
	pb "github.com/bnema/waymon/internal/proto"
)

// SwitchManager manages computer switching and rotation
type SwitchManager struct {
	mu               sync.RWMutex
	computers        []string          // List of computer names in rotation order
	currentComputer  int32             // Index of currently active computer (0 = server)
	active           bool              // Whether mouse sharing is currently active
	connected        bool              // Whether connected to server
	serverHost       string            // Server address if connected
	onSwitchCallback func(int32, bool) // Callback when switch occurs
}

// NewSwitchManager creates a new switch manager
func NewSwitchManager() *SwitchManager {
	return &SwitchManager{
		computers:       []string{"server"}, // Start with just server
		currentComputer: 0,                  // Default to server
		active:          false,
		connected:       false,
	}
}

// SetOnSwitchCallback sets the callback function for switch events
func (sm *SwitchManager) SetOnSwitchCallback(callback func(int32, bool)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onSwitchCallback = callback
}

// AddComputer adds a computer to the rotation
func (sm *SwitchManager) AddComputer(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if computer already exists
	for _, existing := range sm.computers {
		if existing == name {
			return
		}
	}

	sm.computers = append(sm.computers, name)
	logger.Infof("Added computer to rotation: %s (total: %d)", name, len(sm.computers))
}

// RemoveComputer removes a computer from the rotation
func (sm *SwitchManager) RemoveComputer(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var filtered []string
	removedIndex := -1

	for i, computer := range sm.computers {
		if computer == name {
			removedIndex = i
		} else {
			filtered = append(filtered, computer)
		}
	}

	if removedIndex == -1 {
		return // Computer not found
	}

	sm.computers = filtered

	// Adjust current computer index if necessary
	if int32(removedIndex) == sm.currentComputer { //nolint:gosec // index within slice bounds
		// If the current computer was removed, switch to server (index 0)
		sm.currentComputer = 0
	} else if int32(removedIndex) < sm.currentComputer { //nolint:gosec // index within slice bounds
		// If a computer before the current one was removed, decrement index
		sm.currentComputer--
	}

	logger.Infof("Removed computer from rotation: %s (total: %d)", name, len(sm.computers))
}

// SetConnectionState updates the connection state
func (sm *SwitchManager) SetConnectionState(connected bool, serverHost string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.connected = connected
	sm.serverHost = serverHost
}

// SetActiveState updates the active state
func (sm *SwitchManager) SetActiveState(active bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.active = active
}

// SwitchNext switches to the next computer in rotation
func (sm *SwitchManager) SwitchNext() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.computers) <= 1 {
		return fmt.Errorf("only one computer in rotation")
	}

	previousComputer := sm.currentComputer
	sm.currentComputer = (sm.currentComputer + 1) % int32(len(sm.computers)) //nolint:gosec // slice length within int32 range

	logger.Infof("Switched from %s to %s",
		sm.computers[previousComputer],
		sm.computers[sm.currentComputer])

	// Trigger callback if set
	if sm.onSwitchCallback != nil {
		sm.onSwitchCallback(sm.currentComputer, sm.active)
	}

	return nil
}

// SwitchPrevious switches to the previous computer in rotation
func (sm *SwitchManager) SwitchPrevious() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.computers) <= 1 {
		return fmt.Errorf("only one computer in rotation")
	}

	previousComputer := sm.currentComputer
	sm.currentComputer = (sm.currentComputer - 1 + int32(len(sm.computers))) % int32(len(sm.computers))

	logger.Infof("Switched from %s to %s",
		sm.computers[previousComputer],
		sm.computers[sm.currentComputer])

	// Trigger callback if set
	if sm.onSwitchCallback != nil {
		sm.onSwitchCallback(sm.currentComputer, sm.active)
	}

	return nil
}

// GetStatus returns the current status for IPC responses
func (sm *SwitchManager) GetStatus() (bool, bool, string, int32, int32, []string) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.active, sm.connected, sm.serverHost,
		sm.currentComputer, int32(len(sm.computers)), sm.computers
}

// CurrentComputerName returns the name of the currently active computer
func (sm *SwitchManager) CurrentComputerName() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.currentComputer >= 0 && int(sm.currentComputer) < len(sm.computers) {
		return sm.computers[sm.currentComputer]
	}
	return "unknown"
}

// IPCHandler implements the IPC MessageHandler interface
type IPCHandler struct {
	switchManager *SwitchManager
}

// NewIPCHandler creates a new IPC handler
func NewIPCHandler(switchManager *SwitchManager) *IPCHandler {
	return &IPCHandler{
		switchManager: switchManager,
	}
}

// HandleSwitchCommand handles switch commands from IPC
func (h *IPCHandler) HandleSwitchCommand(cmd *pb.SwitchCommand) (*pb.IPCMessage, error) {
	logger.Debugf("Handling switch command: %s", cmd.Action)

	switch cmd.Action {
	case pb.SwitchAction_SWITCH_ACTION_NEXT:
		if err := h.switchManager.SwitchNext(); err != nil {
			return ipc.NewErrorMessage(err.Error())
		}

	case pb.SwitchAction_SWITCH_ACTION_PREVIOUS:
		if err := h.switchManager.SwitchPrevious(); err != nil {
			return ipc.NewErrorMessage(err.Error())
		}

	case pb.SwitchAction_SWITCH_ACTION_ENABLE:
		h.switchManager.SetActiveState(true)
		logger.Info("Mouse sharing enabled via IPC")

	case pb.SwitchAction_SWITCH_ACTION_DISABLE:
		h.switchManager.SetActiveState(false)
		logger.Info("Mouse sharing disabled via IPC")

	default:
		return ipc.NewErrorMessage(fmt.Sprintf("unknown switch action: %s", cmd.Action))
	}

	// Return current status
	active, connected, serverHost, currentComputer, totalComputers, computerNames := h.switchManager.GetStatus()
	return ipc.NewStatusResponseMessage(active, connected, serverHost, currentComputer, totalComputers, computerNames)
}

// HandleStatusQuery handles status queries from IPC
func (h *IPCHandler) HandleStatusQuery(query *pb.StatusQuery) (*pb.IPCMessage, error) {
	logger.Debug("Handling status query")

	active, connected, serverHost, currentComputer, totalComputers, computerNames := h.switchManager.GetStatus()
	return ipc.NewStatusResponseMessage(active, connected, serverHost, currentComputer, totalComputers, computerNames)
}
