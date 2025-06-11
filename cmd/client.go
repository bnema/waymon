package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/network"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	serverAddr string
	edgeSize   int
)

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Run Waymon in client mode",
	Long: `Run Waymon in client mode to send mouse/keyboard events to a server.
The client will capture mouse movement at screen edges and redirect it to the server.`,
	RunE: runClient,
}

func init() {
	clientCmd.Flags().StringVarP(&serverAddr, "host", "H", "", "Server address (host:port)")
	clientCmd.Flags().IntVarP(&edgeSize, "edge", "e", 5, "Edge detection size in pixels")
	clientCmd.MarkFlagRequired("host")
}

func runClient(cmd *cobra.Command, args []string) error {
	// Initialize display detection
	disp, err := display.New()
	if err != nil {
		return fmt.Errorf("failed to initialize display: %w", err)
	}
	defer disp.Close()

	// Show monitor configuration
	monitors := disp.GetMonitors()
	fmt.Printf("Detected %d monitor(s):\n", len(monitors))
	for _, mon := range monitors {
		fmt.Printf("  %s: %dx%d at (%d,%d)\n", mon.Name, mon.Width, mon.Height, mon.X, mon.Y)
	}

	// Create client
	client := network.NewClient()

	// Connect to server
	if err := client.Connect(context.Background(), serverAddr); err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer client.Disconnect()

	// Create edge detector
	edgeDetector := &EdgeDetector{
		display:   disp,
		client:    client,
		threshold: int32(edgeSize),
		active:    false,
	}

	// Create TUI
	p := tea.NewProgram(newClientModel(client, edgeDetector))

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		p.Send(tea.Quit())
	}()

	// Run TUI
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

// EdgeDetector monitors cursor position and triggers edge events
type EdgeDetector struct {
	display   *display.Display
	client    *network.Client
	threshold int32
	active    bool
	lastEdge  display.Edge
	capturing bool
}

func (e *EdgeDetector) Start() {
	e.active = true
	// TODO: Start monitoring cursor position
	// This would use a separate goroutine to poll cursor position
	// and detect when it hits screen edges
}

func (e *EdgeDetector) Stop() {
	e.active = false
}

// clientModel is the TUI model for client mode
type clientModel struct {
	client       *network.Client
	edgeDetector *EdgeDetector
	spinner      spinner.Model
	status       string
	connected    bool
	capturing    bool
	quitting     bool
}

func newClientModel(client *network.Client, edgeDetector *EdgeDetector) clientModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	
	return clientModel{
		client:       client,
		edgeDetector: edgeDetector,
		spinner:      s,
		status:       fmt.Sprintf("Connected to %s", serverAddr),
		connected:    true,
	}
}

func (m clientModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			// Start edge detection
			m.edgeDetector.Start()
			return nil
		},
	)
}

func (m clientModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			m.edgeDetector.Stop()
			return m, tea.Quit
		case "space":
			// Toggle capture
			m.capturing = !m.capturing
			if m.capturing {
				m.status = "Mouse capture enabled"
			} else {
				m.status = "Mouse capture disabled"
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		// Handle resize if needed
	}

	return m, nil
}

func (m clientModel) View() string {
	if m.quitting {
		return "Disconnecting from server...\n"
	}

	// Header
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Render("Waymon Client")

	// Status
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
	
	var statusIcon string
	if m.connected {
		statusIcon = "●"
	} else {
		statusIcon = "○"
	}
	
	status := fmt.Sprintf("%s %s %s", statusIcon, m.spinner.View(), m.status)

	// Build view
	view := header + "\n\n" + statusStyle.Render(status) + "\n"

	// Show capture state
	if m.capturing {
		captureStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))
		view += "\n" + captureStyle.Render("✓ Mouse capture active") + "\n"
	} else {
		view += "\n○ Mouse capture inactive\n"
	}

	// Controls
	view += "\nControls:\n"
	view += "  Space - Toggle mouse capture\n"
	view += "  q     - Quit\n"

	// Edge detection hint
	if m.capturing {
		hintStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true)
		view += "\n" + hintStyle.Render("Move mouse to screen edge to switch computers")
	}

	return view
}