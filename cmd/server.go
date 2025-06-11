package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/network"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	serverPort int
	bindAddress string
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run Waymon in server mode",
	Long: `Run Waymon in server mode to receive mouse/keyboard events from a client.
The server will inject received events using the uinput kernel module.`,
	RunE: runServer,
}

func init() {
	serverCmd.Flags().IntVarP(&serverPort, "port", "p", 0, "Port to listen on")
	serverCmd.Flags().StringVarP(&bindAddress, "bind", "b", "", "Bind address")
	
	// Bind flags to viper
	viper.BindPFlag("server.port", serverCmd.Flags().Lookup("port"))
	viper.BindPFlag("server.bind_address", serverCmd.Flags().Lookup("bind"))
}

func runServer(cmd *cobra.Command, args []string) error {
	// Check if running with sudo (required for uinput)
	if os.Geteuid() != 0 {
		return fmt.Errorf("server mode requires root privileges for uinput access\nPlease run with: sudo waymon server")
	}

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

	// Initialize input handler
	inputHandler, err := input.NewHandler()
	if err != nil {
		return fmt.Errorf("failed to initialize input handler: %w", err)
	}
	defer inputHandler.Close()

	// Get configuration
	cfg := config.Get()
	
	// Use flag values if provided, otherwise use config
	if serverPort == 0 {
		serverPort = cfg.Server.Port
	}
	if bindAddress == "" {
		bindAddress = cfg.Server.BindAddress
	}
	
	// Show server info
	fmt.Printf("Starting Waymon server '%s' on %s:%d\n", cfg.Server.Name, bindAddress, serverPort)
	if cfg.Server.RequireAuth {
		fmt.Println("Authentication enabled")
	}
	
	// Create server
	server := network.NewServer(serverPort)

	// Start server in background
	errCh := make(chan error)
	go func() {
		if err := server.Start(context.Background()); err != nil {
			errCh <- err
		}
	}()

	// Create TUI
	p := tea.NewProgram(newServerModel(server))

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-sigCh:
			p.Send(tea.Quit())
		case err := <-errCh:
			log.Printf("Server error: %v", err)
			p.Send(tea.Quit())
		}
	}()

	// Run TUI
	if _, err := p.Run(); err != nil {
		return err
	}

	// Cleanup
	server.Stop()
	return nil
}

// serverModel is the TUI model for server mode
type serverModel struct {
	server   *network.Server
	spinner  spinner.Model
	status   string
	clients  []string
	quitting bool
}

func newServerModel(server *network.Server) serverModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	
	return serverModel{
		server:  server,
		spinner: s,
		status:  fmt.Sprintf("Listening on port %d", serverPort),
	}
}

func (m serverModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m serverModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
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

func (m serverModel) View() string {
	if m.quitting {
		return "Shutting down server...\n"
	}

	// Header
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Render("Waymon Server")

	// Status
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
	
	status := fmt.Sprintf("%s %s", m.spinner.View(), m.status)

	// Build view
	view := header + "\n\n" + statusStyle.Render(status) + "\n"

	// Show connected clients
	if len(m.clients) > 0 {
		view += "\nConnected clients:\n"
		for _, client := range m.clients {
			view += fmt.Sprintf("  • %s\n", client)
		}
	} else {
		view += "\nNo clients connected\n"
	}

	view += "\nPress 'q' to quit\n"

	return view
}