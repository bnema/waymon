package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/server"
	"github.com/bnema/waymon/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	serverPort  int
	bindAddress string
	noTUI       bool
	debugTUI    bool
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
	serverCmd.Flags().BoolVar(&noTUI, "no-tui", false, "Run without TUI (useful for non-interactive environments)")
	serverCmd.Flags().BoolVar(&debugTUI, "debug-tui", false, "Use minimal debug TUI")

	// Bind flags to viper
	viper.BindPFlag("server.port", serverCmd.Flags().Lookup("port"))
	viper.BindPFlag("server.bind_address", serverCmd.Flags().Lookup("bind"))
}

// initializeServer performs all server initialization steps
func initializeServer(ctx context.Context, srv *server.Server, cfg *config.Config, bindAddress string, serverPort int, p *tea.Program) error {
	logger.Info("Starting server components...")
	// Start the server with context - this creates the SSH server
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// NOW set up client connection callbacks AFTER the SSH server is created
	if sshSrv := srv.GetNetworkServer(); sshSrv != nil {
		logger.Debug("Setting up SSH server callbacks")
		sshSrv.OnClientConnected = func(addr, publicKey string) {
			if p != nil {
				p.Send(ui.ClientConnectedMsg{ClientAddr: addr})
			} else {
				logger.Infof("Client connected from %s", addr)
			}
		}
		sshSrv.OnClientDisconnected = func(addr string) {
			if p != nil {
				p.Send(ui.ClientDisconnectedMsg{ClientAddr: addr})
			} else {
				logger.Infof("Client disconnected from %s", addr)
			}
		}
		sshSrv.OnAuthRequest = func(addr, publicKey, fingerprint string) bool {
			if p != nil {
				// Create a channel for the response
				responseChan := make(chan bool, 1)

				// Send auth request to UI with the response channel
				p.Send(ui.SSHAuthRequestMsg{
					ClientAddr:   addr,
					PublicKey:    publicKey,
					Fingerprint:  fingerprint,
					ResponseChan: responseChan,
				})

				// Wait for approval from UI
				select {
				case approved := <-responseChan:
					return approved
				case <-time.After(30 * time.Second):
					// Timeout after 30 seconds
					logger.Warn("SSH auth request timed out", "fingerprint", fingerprint)
					return false
				}
			} else {
				// In non-TUI mode, auto-approve (you might want to change this)
				logger.Warnf("Auto-approving SSH connection from %s (fingerprint: %s) - running in no-TUI mode", addr, fingerprint)
				return true
			}
		}
	} else {
		logger.Error("SSH server not initialized after Start()")
		return fmt.Errorf("SSH server not initialized")
	}

	logger.Info("Server components started successfully")

	// Show monitor configuration
	if disp := srv.GetDisplay(); disp != nil {
		monitors := disp.GetMonitors()
		logger.Infof("Detected %d monitor(s):", len(monitors))
		for _, mon := range monitors {
			monitorInfo := fmt.Sprintf("  %s: %dx%d at (%d,%d)", mon.Name, mon.Width, mon.Height, mon.X, mon.Y)
			if mon.Primary {
				monitorInfo += " [PRIMARY]"
			}
			if mon.Scale != 1.0 {
				monitorInfo += fmt.Sprintf(" scale=%.1f", mon.Scale)
			}
			logger.Info(monitorInfo)
		}
	}

	// Show server info
	logger.Infof("Starting Waymon SSH server '%s' on %s:%d", cfg.Server.Name, bindAddress, serverPort)
	// Get the actual expanded paths from the server
	if sshSrv := srv.GetNetworkServer(); sshSrv != nil {
		logger.Infof("SSH Host Key: %s", srv.GetSSHHostKeyPath())
		logger.Infof("SSH Authorized Keys: %s", srv.GetSSHAuthKeysPath())
	}

	// Start network server after all setup is complete
	logger.Info("Starting network server...")
	if err := srv.StartNetworking(ctx); err != nil {
		return fmt.Errorf("failed to start network server: %w", err)
	}

	return nil
}

func runServer(cmd *cobra.Command, args []string) error {
	// Set up file logging since Bubble Tea will hide terminal output
	logFile, err := logger.SetupFileLogging("SERVER")
	if err != nil {
		return fmt.Errorf("failed to setup file logging: %w", err)
	}
	defer logFile.Close()

	// Show log location if not using TUI
	if noTUI {
		fmt.Printf("Logging to: %s\n", logFile.Name())
	}

	// Server runs as normal user and will request sudo when needed for uinput

	// Check if uinput is available (but don't fail if no access yet)
	if err := CheckUinputAvailable(); err != nil {
		logger.Warnf("uinput not fully configured: %v", err)
		logger.Info("Server will request sudo access when needed for mouse injection")
	}

	// Ensure config file exists
	if err := ensureServerConfig(); err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	// Get configuration
	cfg := config.Get()

	// Use flag values if provided, otherwise use config
	if serverPort == 0 {
		serverPort = cfg.Server.Port
	}
	if bindAddress == "" {
		bindAddress = cfg.Server.BindAddress
	}
	cfg.Server.Port = serverPort
	cfg.Server.BindAddress = bindAddress

	var p *tea.Program
	var srv *server.Server

	if !noTUI {
		if debugTUI {
			// Use minimal debug UI
			debugModel := ui.NewDebugServerModel()
			p = tea.NewProgram(debugModel)
		} else {
			// Create full-screen TUI model
			model := ui.NewFullscreenServerModel(serverPort, cfg.Server.Name)
			p = tea.NewProgram(model, tea.WithAltScreen())
		}

		// Set up logger to send log entries to UI
		logger.SetUINotifier(func(level, message string) {
			if p != nil {
				logEntry := ui.LogEntry{
					Timestamp: time.Now(),
					Level:     level,
					Message:   message,
				}
				p.Send(ui.LogMsg{Entry: logEntry})
			}
		})
	} else {
		// No TUI - create server immediately
		logger.Info("Creating server instance...")
		var err error
		srv, err = server.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to create server: %w", err)
		}

		logger.Info("Waymon server starting...")
		logger.Debug("DEBUG: This is a debug message to test log level")
	}

	// Create a context that we'll cancel on shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if p == nil {
		// No TUI mode - initialize server immediately
		if err := initializeServer(ctx, srv, cfg, bindAddress, serverPort, nil); err != nil {
			return err
		}
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		// Cancel context first to start shutdown
		cancel()
		// Stop server if it exists
		if srv != nil {
			srv.Stop()
		}
		// Then tell TUI to quit
		if p != nil {
			p.Send(tea.Quit())
		}
	}()

	if p != nil {
		// Start server initialization in background after a delay
		go func() {
			// Give TUI time to start
			time.Sleep(500 * time.Millisecond)

			// Create server instance now that TUI is running
			logger.Info("Creating server instance...")
			var err error
			srv, err = server.New(cfg)
			if err != nil {
				logger.Errorf("Failed to create server: %v", err)
				p.Send(tea.Quit())
				return
			}

			logger.Info("Waymon server starting...")
			logger.Debug("DEBUG: This is a debug message to test log level")

			// Initialize server
			if err := initializeServer(ctx, srv, cfg, bindAddress, serverPort, p); err != nil {
				logger.Errorf("Server initialization failed: %v", err)
				// Send quit signal to TUI
				p.Send(tea.Quit())
			}
		}()

		// Run TUI (blocking)
		if _, err := p.Run(); err != nil {
			return err
		}
	} else {
		// In non-TUI mode, just wait for shutdown signal
		<-sigCh
		logger.Info("Shutting down server...")
	}

	return nil
}

// ensureServerConfig ensures the config file exists when running as server
func ensureServerConfig() error {
	configPath := config.GetConfigPath()

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logger.Infof("No config file found. Creating default config at %s", configPath)

		// Save default config
		if err := config.Save(); err != nil {
			return err
		}

		logger.Info("Default configuration created successfully")
	}

	return nil
}
