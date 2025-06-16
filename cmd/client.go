package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bnema/waymon/internal/client"
	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	serverAddr string
	hostName   string
)

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Run Waymon in client mode",
	Long: `Run Waymon in client mode to receive mouse/keyboard events from a server.
The client will inject received input events locally using uinput.`,
	RunE: runClient,
}

func init() {
	clientCmd.Flags().StringVarP(&serverAddr, "host", "H", "", "Server address (host:port)")
	clientCmd.Flags().StringVarP(&hostName, "name", "n", "", "Host name from config")

	// Bind flags to viper
	if err := viper.BindPFlag("client.server_address", clientCmd.Flags().Lookup("host")); err != nil {
		logger.Errorf("Failed to bind host flag: %v", err)
	}
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
	defer func() {
		if err := logFile.Close(); err != nil {
			logger.Errorf("Failed to close log file: %v", err)
		}
	}()

	// Setup verification is no longer needed - libei handles permissions automatically

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

	// Note: Edge detection no longer needed in redesigned architecture

	// Initialize display detection
	disp, err := display.New()
	if err != nil {
		return fmt.Errorf("failed to initialize display: %w", err)
	}
	defer func() {
		if err := disp.Close(); err != nil {
			logger.Errorf("Failed to close display: %v", err)
		}
	}()

	// Show monitor configuration
	monitors := disp.GetMonitors()
	logger.Infof("Detected %d monitor(s):", len(monitors))
	for _, mon := range monitors {
		logger.Infof("  %s: %dx%d at (%d,%d)", mon.Name, mon.Width, mon.Height, mon.X, mon.Y)
	}

	// Get private key path for InputReceiver
	privateKeyPath := cfg.Client.SSHPrivateKey

	// Create input receiver for the redesigned architecture
	inputReceiver, err := client.NewInputReceiver(serverAddr)
	if err != nil {
		return fmt.Errorf("failed to create input receiver: %w", err)
	}
	defer func() {
		if err := inputReceiver.Disconnect(); err != nil {
			logger.Errorf("Failed to disconnect input receiver: %v", err)
		}
	}()

	// Create redesigned client TUI model
	model := ui.NewClientModel(serverAddr, inputReceiver, Version)
	logger.Debug("Created redesigned client UI model")

	// Create the program with alt screen mode for proper full-screen UI
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Note: We'll set up the logger UI notifier AFTER p.Run() starts
	// to avoid deadlock issues with p.Send() before the program is running

	// Note: In redesigned architecture, client only receives input from server
	// No local input capture needed

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

		logger.Infof("Attempting to connect to server at %s", serverAddr)

		// Start connection with smart approval detection
		// Only show "waiting for approval" if connection takes longer than expected
		connectionStart := time.Now()

		// Create a timer to show approval message if connection takes too long
		approvalTimer := time.AfterFunc(3*time.Second, func() {
			// If connection is still in progress after 3 seconds, likely waiting for approval
			p.Send(ui.WaitingApprovalMsg{})
		})
		defer approvalTimer.Stop() // Ensure timer is cleaned up

		// Use the parent context for the actual connection
		// The input backend and SSH receive loop need to stay alive after connection
		if err := inputReceiver.Connect(ctx, privateKeyPath); err != nil {
			errStr := err.Error()
			switch {
			case strings.Contains(errStr, "waiting for server approval"):
				// Keep showing the waiting message
				logger.Info("Connection pending server approval")
			case strings.Contains(errStr, "timed out"):
				logger.Errorf("Connection timed out: %v", err)
				p.Send(ui.DisconnectedMsg{})
			default:
				logger.Errorf("Failed to connect to server: %v", err)
				p.Send(ui.DisconnectedMsg{})
			}
		} else {
			connectionDuration := time.Since(connectionStart)
			logger.Infof("Successfully connected to server at %s in %v", serverAddr, connectionDuration)
			p.Send(ui.ConnectedMsg{})

			// In redesigned architecture, client is now ready to receive input from server
			logger.Info("Client ready to receive input from server")
		}
	}

	// Set up reconnection status callback now that TUI program is created
	inputReceiver.SetOnReconnectStatus(func(status string) {
		logEntry := ui.LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   status,
		}
		p.Send(ui.LogMsg{Entry: logEntry})

		// Also send appropriate UI messages based on status
		switch {
		case strings.Contains(status, "Reconnection attempt") ||
			strings.Contains(status, "Reconnecting") ||
			strings.Contains(status, "retrying") ||
			strings.Contains(status, "attempting to reconnect"):
			p.Send(ui.ReconnectingMsg{Status: status})
		case strings.Contains(status, "Reconnected successfully"):
			p.Send(ui.ConnectedMsg{})
		case strings.Contains(status, "Server shutdown") ||
			strings.Contains(status, "Connection lost"):
			p.Send(ui.DisconnectedMsg{})
			p.Send(ui.ReconnectingMsg{Status: status})
		}
	})

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
		if inputReceiver.IsConnected() {
			logger.Info("Disconnecting from server...")
			if err := inputReceiver.Disconnect(); err != nil {
				logger.Errorf("Failed to disconnect input receiver during cleanup: %v", err)
			}
		}
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
