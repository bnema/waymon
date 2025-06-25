package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bnema/waymon/internal/client"
	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/ui"
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
		if err := inputReceiver.Disconnect(); err != nil {
			logger.Errorf("Failed to disconnect input receiver: %v", err)
		}
	}()

	// Note: We'll set up the logger UI notifier AFTER the UI starts
	// to avoid deadlock issues with p.Send() before the program is running

	// Note: In redesigned architecture, client only receives input from server
	// No local input capture needed

	// Set up input receiver callbacks and connection logic
	// This will be handled by the UI runner
	inputReceiver.SetOnReconnectStatus(func(status string) {
		// This will be properly set up once the UI is running
		logger.Infof("Connection status: %s", status)
	})

	// Start connection logic in background
	go func() {
		// Wait for UI to initialize and set up callbacks
		// Increased delay to ensure UI is fully ready before any connection attempts
		// and subsequent logging messages are sent.
		time.Sleep(2 * time.Second)

		// Connection retry logic
		backoff := 1 * time.Second
		maxBackoff := 60 * time.Second
		attempt := 1

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
			err := inputReceiver.Connect(ctx, privateKeyPath)

			if err == nil {
				// Connection successful
				logger.Infof("Successfully connected to server at %s", serverAddr)
				logger.Info("Client ready to receive input from server")
				return
			}

			// Connection failed, log error
			logger.Errorf("Connection attempt %d failed: %v", attempt, err)

			// Wait with exponential backoff
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}

			// Increase backoff
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
		if inputReceiver.IsConnected() {
			logger.Info("Disconnecting from server...")
			if err := inputReceiver.Disconnect(); err != nil {
				logger.Errorf("Failed to disconnect input receiver during cleanup: %v", err)
			}
		}
	}()

	// Run the client UI with the new refactored approach
	if err := ui.RunClientUI(ctx, serverAddr, inputReceiver, Version); err != nil {
		return err
	}

	return nil
}
