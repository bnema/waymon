package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/network"
	"github.com/bnema/waymon/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	serverAddr string
	edgeSize   int
	hostName   string
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
	clientCmd.Flags().IntVarP(&edgeSize, "edge", "e", 0, "Edge detection size in pixels")
	clientCmd.Flags().StringVarP(&hostName, "name", "n", "", "Host name from config")

	// Bind flags to viper
	viper.BindPFlag("client.server_address", clientCmd.Flags().Lookup("host"))
	viper.BindPFlag("client.edge_threshold", clientCmd.Flags().Lookup("edge"))
}

func runClient(cmd *cobra.Command, args []string) error {
	// Verify uinput setup has been completed
	if err := VerifyUinputSetup(); err != nil {
		return fmt.Errorf("uinput setup verification failed: %w", err)
	}

	// Get configuration
	cfg := config.Get()

	// Determine server address
	if hostName != "" {
		// Look up host from config
		host, err := config.GetHost(hostName)
		if err != nil {
			return fmt.Errorf("host '%s' not found in config", hostName)
		}
		serverAddr = host.Address
	} else if serverAddr == "" {
		// Use default from config
		serverAddr = cfg.Client.ServerAddress
	}

	// Validate we have a server address
	if serverAddr == "" {
		return fmt.Errorf("no server address specified (use --host or configure a default)")
	}

	// Use edge size from config if not specified
	if edgeSize == 0 {
		edgeSize = cfg.Client.EdgeThreshold
	}

	// Initialize display detection
	disp, err := display.New()
	if err != nil {
		return fmt.Errorf("failed to initialize display: %w", err)
	}
	defer disp.Close()

	// Show monitor configuration
	monitors := disp.GetMonitors()
	logger.Infof("Detected %d monitor(s):", len(monitors))
	for _, mon := range monitors {
		logger.Infof("  %s: %dx%d at (%d,%d)", mon.Name, mon.Width, mon.Height, mon.X, mon.Y)
	}

	// Create SSH client (will connect later)
	privateKeyPath := cfg.Client.SSHPrivateKey
	client := network.NewSSHClient(privateKeyPath)
	defer client.Disconnect()

	// Create edge detector
	edgeDetector := input.NewEdgeDetector(disp, client, int32(edgeSize))

	// Create inline TUI model
	model := ui.NewInlineClientModel(serverAddr)

	// Create the program first
	p := tea.NewProgram(model)

	// Connect to server in background
	go func() {
		ctx := context.Background()
		if err := client.Connect(ctx, serverAddr); err != nil {
			if strings.Contains(err.Error(), "waiting for server approval") {
				logger.Info("Waiting for server approval...")
				p.Send(ui.WaitingApprovalMsg{})
			} else {
				logger.Errorf("Failed to connect to server: %v", err)
				p.Send(ui.DisconnectedMsg{})
			}
		} else {
			p.Send(ui.ConnectedMsg{})
		}
	}()

	// Set up edge callbacks
	edgeDetector.SetCallbacks(
		func(edge display.Edge, x, y int32) {
			// Edge entered - start capture
			if client.IsConnected() {
				edgeDetector.StartCapture(edge)
				p.Send(ui.CaptureStartMsg{})
			}
		},
		func() {
			// Edge left - stop capture
			edgeDetector.StopCapture()
			p.Send(ui.CaptureStopMsg{})
		},
	)

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
