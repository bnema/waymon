package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
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

	// Get configuration first to check logging settings
	cfg := config.Get()

	// Apply log level from config if set
	if cfg.Logging.LogLevel != "" {
		logger.SetLevel(cfg.Logging.LogLevel)
	}

	// Set up file logging if enabled
	var logFile *os.File
	if cfg.Logging.FileLogging {
		// Set up file logging since Bubble Tea will hide terminal output
		// This MUST be done before any log output to avoid TUI corruption
		var err error
		logFile, err = logger.SetupFileLogging("CLIENT")
		if err != nil {
			return fmt.Errorf("failed to setup file logging: %w", err)
		}
		defer func() {
			if logFile != nil && logFile.Fd() != 0 {
				if err := logFile.Close(); err != nil {
					logger.Errorf("Failed to close log file: %v", err)
				}
			}
		}()
	}

	// Setup verification is no longer needed - libei handles permissions automatically

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
		inputReceiver.StopEmergencyRelease()
		if err := inputReceiver.Disconnect(); err != nil {
			logger.Errorf("Failed to disconnect input receiver: %v", err)
		}
	}()

	// Create redesigned client TUI model
	model := ui.NewClientModel(serverAddr, inputReceiver, Version)
	logger.Debug("Created redesigned client UI model")

	// Create the program with alt screen mode for proper full-screen UI
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Set up the program reference for status updates
	model.SetProgram(p)

	// Note: We'll set up the logger UI notifier AFTER p.Run() starts
	// to avoid deadlock issues with p.Send() before the program is running

	// Note: In redesigned architecture, client only receives input from server
	// No local input capture needed

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

		// Initial connection with retry logic
		backoff := 1 * time.Second
		maxBackoff := 60 * time.Second
		attempt := 1

		// Set up logger to send to UI now that TUI is running
		logger.SetUINotifier(func(level, message string) {
			logEntry := ui.LogEntry{
				Timestamp: time.Now(),
				Level:     level,
				Message:   message,
			}
			p.Send(ui.LogMsg{Entry: logEntry})
		})

		logger.Info("Starting connection to server with automatic retry")

		for {
			select {
			case <-ctx.Done():
				logger.Info("Connection attempts cancelled")
				return
			default:
			}

			logger.Infof("Connection attempt %d to %s", attempt, serverAddr)

			// Try to connect
			connectionStart := time.Now()

			// Create a timer to show approval message if connection takes too long
			approvalTimer := time.AfterFunc(3*time.Second, func() {
				// If connection is still in progress after 3 seconds, likely waiting for approval
				p.Send(ui.WaitingApprovalMsg{})
			})

			// Use the parent context for the connection - it needs to stay alive for receiving
			err := inputReceiver.Connect(ctx, privateKeyPath)
			approvalTimer.Stop() // Ensure timer is cleaned up

			if err == nil {
				// Connection successful
				connectionDuration := time.Since(connectionStart)
				logger.Infof("Successfully connected to server at %s in %v", serverAddr, connectionDuration)
				p.Send(ui.ConnectedMsg{})

				// In redesigned architecture, client is now ready to receive input from server
				logger.Info("Client ready to receive input from server")

				// Start emergency release monitoring
				inputReceiver.StartEmergencyRelease(ctx, func(reason string) {
					logger.Warnf("Emergency release triggered: %s", reason)
					p.Send(ui.DisconnectedMsg{})
					// Cancel context to initiate shutdown
					cancel()
				})

				return // Exit the retry loop
			}

			// Connection failed, handle error
			errStr := err.Error()
			switch {
			case strings.Contains(errStr, "waiting for server approval"):
				// Keep showing the waiting message
				logger.Info("Connection pending server approval")
			case strings.Contains(errStr, "timed out"):
				logger.Errorf("Connection attempt %d timed out: %v", attempt, err)
				p.Send(ui.DisconnectedMsg{})
				p.Send(ui.ReconnectingMsg{Status: fmt.Sprintf("Connection timed out, retrying in %v...", backoff)})
			case strings.Contains(errStr, "connection refused"):
				logger.Warnf("Connection attempt %d refused: %v", attempt, err)
				p.Send(ui.DisconnectedMsg{})
				p.Send(ui.ReconnectingMsg{Status: fmt.Sprintf("Server not ready, retrying in %v...", backoff)})
			default:
				logger.Errorf("Connection attempt %d failed: %v", attempt, err)
				p.Send(ui.DisconnectedMsg{})
				p.Send(ui.ReconnectingMsg{Status: fmt.Sprintf("Connection failed, retrying in %v...", backoff)})
			}

			// Wait with exponential backoff
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}

			// Increase backoff, but cap it
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			attempt++
		}
	}()

	// Ensure cleanup happens on any exit path
	defer func() {
		logger.Info("Cleaning up client resources...")
		cancel() // Cancel context
		inputReceiver.StopEmergencyRelease()
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

	// Track if we're already shutting down
	shuttingDown := false
	shutdownMutex := &sync.Mutex{}

	go func() {
		for sig := range sigCh {
			shutdownMutex.Lock()
			if !shuttingDown {
				shuttingDown = true
				shutdownMutex.Unlock()
				logger.Info("Received %v signal, initiating graceful shutdown...", sig)

				// Cancel context to signal shutdown to all components
				cancel()

				// Send client shutdown message for proper cleanup
				p.Send(func() tea.Msg { return ui.ClientShutdownMsg{} })
			} else {
				shutdownMutex.Unlock()
				logger.Warn("Received second interrupt signal, forcing exit...")
				fmt.Println("\nForce quitting...")
				os.Exit(1)
			}
		}
	}()

	// Run TUI
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
