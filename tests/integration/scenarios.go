//go:build integration

package integration

import "github.com/bnema/waymon/internal/domain"

// ClientConfig defines a test client configuration.
type ClientConfig struct {
	Name     string
	Position string        // "left", "right", "top", "bottom", etc.
	Layout   MonitorLayout // Monitor configuration for this client
}

// ClientScenario defines a multi-client test scenario.
type ClientScenario struct {
	Name        string
	Description string
	Clients     []ClientConfig
}

// GetClientMonitors returns the monitors for a client in a scenario.
func (s *ClientScenario) GetClientMonitors(clientIndex int) []domain.Monitor {
	if clientIndex < 0 || clientIndex >= len(s.Clients) {
		return nil
	}
	return CreateTestMonitors(s.Clients[clientIndex].Layout)
}

// NumClients returns the number of clients in the scenario.
func (s *ClientScenario) NumClients() int {
	return len(s.Clients)
}

// TestScenarios defines all multi-client test scenarios.
var TestScenarios = []ClientScenario{
	{
		Name:        "TwoClientsHorizontal",
		Description: "Two clients arranged horizontally (left-right)",
		Clients: []ClientConfig{
			{Name: "client-left", Position: "left", Layout: DualHorizontal},
			{Name: "client-right", Position: "right", Layout: SingleMonitor},
		},
	},
	{
		Name:        "TwoClientsVertical",
		Description: "Two clients arranged vertically (top-bottom)",
		Clients: []ClientConfig{
			{Name: "client-top", Position: "top", Layout: SingleMonitor},
			{Name: "client-bottom", Position: "bottom", Layout: SingleMonitor},
		},
	},
	{
		Name:        "ThreeClientsHorizontal",
		Description: "Three clients in a row",
		Clients: []ClientConfig{
			{Name: "client-left", Position: "left", Layout: SingleMonitor},
			{Name: "client-center", Position: "center", Layout: TripleHorizontal},
			{Name: "client-right", Position: "right", Layout: DualVertical},
		},
	},
	{
		Name:        "MixedScalesAndSizes",
		Description: "Clients with different monitor scales and sizes",
		Clients: []ClientConfig{
			{Name: "client-4k", Position: "left", Layout: Monitor4K},
			{Name: "client-1080p", Position: "right", Layout: SingleMonitor},
		},
	},
	{
		Name:        "ComplexOffice",
		Description: "Simulates a complex office setup with varied monitors",
		Clients: []ClientConfig{
			{Name: "workstation", Position: "center", Layout: TripleHorizontal},
			{Name: "laptop", Position: "left", Layout: SingleMonitor},
			{Name: "presentation", Position: "right", Layout: UltraWide},
		},
	},
	{
		Name:        "SingleClientMultiMonitor",
		Description: "Single client with quad monitor setup",
		Clients: []ClientConfig{
			{Name: "quad-setup", Position: "center", Layout: QuadGrid},
		},
	},
	{
		Name:        "MaxClients",
		Description: "Maximum supported clients (stress test)",
		Clients: []ClientConfig{
			{Name: "client-1", Position: "1", Layout: SingleMonitor},
			{Name: "client-2", Position: "2", Layout: SingleMonitor},
			{Name: "client-3", Position: "3", Layout: SingleMonitor},
			{Name: "client-4", Position: "4", Layout: SingleMonitor},
			{Name: "client-5", Position: "5", Layout: SingleMonitor},
		},
	},
}

// GetScenarioByName finds a scenario by name.
func GetScenarioByName(name string) *ClientScenario {
	for i := range TestScenarios {
		if TestScenarios[i].Name == name {
			return &TestScenarios[i]
		}
	}
	return nil
}

// GetAllScenarios returns all test scenarios.
func GetAllScenarios() []ClientScenario {
	return TestScenarios
}

// SwitchTestCase defines a test case for control switching.
type SwitchTestCase struct {
	Name           string
	StartIndex     int  // Index to start control at (0 = server/local)
	SwitchToIndex  int  // Index to switch to
	ExpectSuccess  bool // Whether the switch should succeed
	ExpectWrapNext bool // True if switch next should wrap
	ExpectWrapPrev bool // True if switch prev should wrap
}

// GetSwitchTestCases returns test cases for switching based on scenario.
func GetSwitchTestCases(scenario *ClientScenario) []SwitchTestCase {
	numClients := scenario.NumClients()
	if numClients == 0 {
		return nil
	}

	cases := []SwitchTestCase{
		// Basic switching
		{Name: "local_to_first_client", StartIndex: 0, SwitchToIndex: 1, ExpectSuccess: true},
		{Name: "first_client_to_local", StartIndex: 1, SwitchToIndex: 0, ExpectSuccess: true},
	}

	// Add multi-client switching cases
	if numClients >= 2 {
		cases = append(cases,
			SwitchTestCase{Name: "client_to_client", StartIndex: 1, SwitchToIndex: 2, ExpectSuccess: true},
		)
	}

	// Wrap-around cases
	cases = append(cases,
		SwitchTestCase{Name: "wrap_next_from_last", StartIndex: numClients, SwitchToIndex: 0, ExpectWrapNext: true},
		SwitchTestCase{Name: "wrap_prev_from_first", StartIndex: 0, SwitchToIndex: numClients, ExpectWrapPrev: true},
	)

	// Invalid slot cases
	cases = append(cases,
		SwitchTestCase{Name: "invalid_slot_too_high", StartIndex: 0, SwitchToIndex: numClients + 10, ExpectSuccess: false},
	)

	return cases
}

// EventTestCase defines a test case for event transmission.
type EventTestCase struct {
	Name        string
	EventType   string // "mouse_move", "mouse_button", "keyboard", etc.
	Count       int    // Number of events to send
	Description string
}

// GetEventTestCases returns test cases for event transmission.
func GetEventTestCases() []EventTestCase {
	return []EventTestCase{
		{Name: "single_mouse_move", EventType: "mouse_move", Count: 1, Description: "Single mouse movement event"},
		{Name: "multiple_mouse_moves", EventType: "mouse_move", Count: 100, Description: "Multiple mouse movement events"},
		{Name: "rapid_mouse_moves", EventType: "mouse_move", Count: 1000, Description: "Rapid mouse movement (stress test)"},
		{Name: "mouse_buttons", EventType: "mouse_button", Count: 10, Description: "Mouse button events"},
		{Name: "scroll_events", EventType: "scroll", Count: 20, Description: "Mouse scroll events"},
		{Name: "keyboard_single", EventType: "keyboard", Count: 1, Description: "Single key press"},
		{Name: "keyboard_typing", EventType: "keyboard", Count: 50, Description: "Simulated typing"},
		{Name: "mixed_events", EventType: "mixed", Count: 100, Description: "Mix of mouse and keyboard events"},
	}
}

// ConnectionTestCase defines a test case for connection behavior.
type ConnectionTestCase struct {
	Name            string
	Action          string // "connect", "disconnect", "reconnect", "timeout"
	ClientCount     int
	ExpectConnected int
	Description     string
}

// GetConnectionTestCases returns test cases for connection handling.
func GetConnectionTestCases() []ConnectionTestCase {
	return []ConnectionTestCase{
		{Name: "single_client_connect", Action: "connect", ClientCount: 1, ExpectConnected: 1, Description: "Single client connects"},
		{Name: "multiple_clients_connect", Action: "connect", ClientCount: 3, ExpectConnected: 3, Description: "Multiple clients connect"},
		{Name: "client_disconnect", Action: "disconnect", ClientCount: 2, ExpectConnected: 1, Description: "Client gracefully disconnects"},
		{Name: "client_reconnect", Action: "reconnect", ClientCount: 1, ExpectConnected: 1, Description: "Client reconnects after disconnect"},
		{Name: "max_clients_limit", Action: "connect", ClientCount: 10, ExpectConnected: 5, Description: "Exceed max client limit (5)"},
	}
}
