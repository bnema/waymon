package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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
	// Create root context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up file logging since Bubble Tea will hide terminal output
	// This MUST be done before any log output to avoid TUI corruption
	logFile, err := logger.SetupFileLogging("CLIENT")
	if err != nil {
		return fmt.Errorf("failed to setup file logging: %w", err)
	}
	defer logFile.Close()

	// Verify waymon client setup has been completed
	if err := VerifyClientSetup(); err != nil {
		return fmt.Errorf("waymon client setup verification failed: %w", err)
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

	// Create edge detector for mouse capture
	edgeDetector := input.NewEdgeDetector(disp, client, int32(edgeSize))

	// Create mouse capture system
	mouseCapture := input.NewMouseCapture(edgeDetector)

	// Create hotkey system for manual switching
	modifiers := uint32(0)
	if cfg.Client.HotkeyModifier == "ctrl+alt" {
		modifiers = input.ModCtrl | input.ModAlt
	}
	hotkeyCapture := input.NewHotkeyCapture(modifiers, input.KEY_S, func() {
		if edgeDetector.IsCapturing() {
			edgeDetector.StopCapture()
		} else {
			// Toggle to primary monitor right edge as default
			primary := disp.GetPrimaryMonitor()
			if primary != nil {
				edgeDetector.StartCapture(display.EdgeRight, primary)
			} else {
				logger.Warn("No primary monitor found for hotkey activation")
			}
		}
	})

	// Try to start hotkey capture (optional feature)
	if err := hotkeyCapture.Start(); err != nil {
		logger.Warnf("Hotkey capture not available: %v", err)
		logger.Info("Edge detection will be the primary switching method")
	}
	defer hotkeyCapture.Stop()

	// Create inline TUI model
	model := ui.NewInlineClientModel(serverAddr)

	// Create the program first
	p := tea.NewProgram(model)

	// Note: We'll set up the logger UI notifier AFTER p.Run() starts
	// to avoid deadlock issues with p.Send() before the program is running

	// Set up edge detector callbacks
	edgeDetector.SetCallbacks(
		func(edge display.Edge, x, y int32) {
			// Edge activated
			if client.IsConnected() {
				p.Send(ui.CaptureStartMsg{})
			}
		},
		func() {
			// Edge deactivated
			p.Send(ui.CaptureStopMsg{})
		},
	)

	// Start server connection after TUI is running
	// We need to delay this to avoid logger deadlock
	connectToServer := func() {
		// Now that TUI is running, set up logger to send to UI
		logger.SetUINotifier(func(level, message string) {
			logEntry := ui.LogEntry{
				Timestamp: time.Now(),
				Level:     level,
				Message:   message,
			}
			p.Send(ui.LogMsg{Entry: logEntry})
		})

		logger.Info("Starting connection to server")

		// Small delay to ensure TUI is fully ready
		time.Sleep(100 * time.Millisecond)

		// Create a context with a 30-second timeout for the connection
		connCtx, connCancel := context.WithTimeout(ctx, 30*time.Second)
		defer connCancel()

		logger.Infof("Attempting to connect to server at %s", serverAddr)

		// Immediately show waiting for approval since SSH auth can take time
		p.Send(ui.WaitingApprovalMsg{})

		if err := client.Connect(connCtx, serverAddr); err != nil {
			if strings.Contains(err.Error(), "waiting for server approval") {
				// Keep showing the waiting message
				logger.Info("Connection pending server approval")
			} else if strings.Contains(err.Error(), "timed out") {
				logger.Errorf("Connection timed out: %v", err)
				p.Send(ui.DisconnectedMsg{})
			} else {
				logger.Errorf("Failed to connect to server: %v", err)
				p.Send(ui.DisconnectedMsg{})
			}
		} else {
			logger.Infof("Successfully connected to server at %s", serverAddr)
			p.Send(ui.ConnectedMsg{})

			// Start edge detector and mouse capture after successful connection
			logger.Info("Starting edge detector and mouse capture systems")
			if err := edgeDetector.Start(); err != nil {
				logger.Errorf("Failed to start edge detector: %v", err)
			}
			if err := mouseCapture.Start(); err != nil {
				logger.Errorf("Failed to start mouse capture: %v", err)
			}
		}
	}

	// Start connection in background AFTER TUI starts
	go func() {
		// Wait a bit for TUI to initialize
		time.Sleep(200 * time.Millisecond)
		connectToServer()
	}()

	// Ensure cleanup happens on any exit path
	defer func() {
		logger.Info("Cleaning up client resources...")
		cancel() // Cancel context
		if client.IsConnected() {
			logger.Info("Disconnecting from server...")
			client.Disconnect()
		}
		edgeDetector.Stop()
		mouseCapture.Stop()
		hotkeyCapture.Stop()
	}()

	// Handle graceful shutdown with proper cleanup
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("Received shutdown signal, initiating graceful shutdown...")

		// Cancel context to signal shutdown to all components
		cancel()

		// Quit TUI immediately - defer cleanup will handle the rest
		p.Send(tea.Quit())
	}()

	// Run TUI
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
