package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/ipc"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/protocol"
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
	daemon      bool
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
	serverCmd.Flags().BoolVar(&daemon, "daemon", false, "Run as daemon (no UI, for systemd service)")

	// Bind flags to viper
	if err := viper.BindPFlag("server.port", serverCmd.Flags().Lookup("port")); err != nil {
		logger.Errorf("Failed to bind port flag: %v", err)
	}
	if err := viper.BindPFlag("server.bind_address", serverCmd.Flags().Lookup("bind")); err != nil {
		logger.Errorf("Failed to bind address flag: %v", err)
	}
}

// initializeServer performs all server initialization steps
func initializeServer(ctx context.Context, srv *server.Server, cfg *config.Config, bindAddress string, serverPort int, model *ui.ServerModel, isDaemon bool) error {
	logger.Info("Starting server components...")
	// Start the server with context - this creates the SSH server
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// NOW set up client connection callbacks AFTER the SSH server is created
	if sshSrv := srv.GetNetworkServer(); sshSrv != nil {
		logger.Debug("Setting up SSH server callbacks")

		// Set up client connection handler (only once!)
		sshSrv.OnClientConnected = func(addr, publicKey string) {
			// Register client with ClientManager
			if cm := srv.GetClientManager(); cm != nil {
				// Use address as both ID and name initially
				// The client will send its actual configuration later
				cm.RegisterClient(addr, addr, addr)
				logger.Infof("Client connected and registered: %s", addr)
			}

			// Send UI notification
			if model != nil {
				logger.Debugf("Model is not nil, checking program...")
				// Send the proper ClientConnectedMsg to trigger UI update
				if p := model.GetProgram(); p != nil {
					logger.Debugf("Sending ClientConnectedMsg to UI for %s", addr)
					p.Send(ui.ClientConnectedMsg{
						ClientAddr: addr,
					})
				} else {
					logger.Warnf("model.GetProgram() returned nil, cannot send UI update")
				}
			} else {
				logger.Warnf("Model is nil, cannot send UI update for client %s", addr)
			}
		}

		// Set up client disconnection handler
		sshSrv.OnClientDisconnected = func(addr string) {
			// Unregister client from ClientManager
			if cm := srv.GetClientManager(); cm != nil {
				cm.UnregisterClient(addr)
				logger.Infof("Client disconnected and unregistered: %s", addr)
			}

			// Send UI notification
			if model != nil {
				// Send the proper ClientDisconnectedMsg to trigger UI update
				if p := model.GetProgram(); p != nil {
					p.Send(ui.ClientDisconnectedMsg{
						ClientAddr: addr,
					})
				}
			}
		}

		// Set up authentication handler
		sshSrv.OnAuthRequest = func(addr, publicKey, fingerprint string) bool {
			if model != nil {
				// For now, log the auth request and auto-approve
				// TODO: Implement interactive approval in the refactored UI
				model.AddLogEntry(ui.LogEntry{
					Timestamp: time.Now(),
					Level:     "WARN",
					Message:   fmt.Sprintf("SSH auth request from %s (fingerprint: %s) - auto-approved", addr, fingerprint),
				})
				return true
			} else {
				// In non-TUI/daemon mode, check authorized_keys file
				if isDaemon {
					logger.Infof("SSH auth request from %s (fingerprint: %s) - checking authorized_keys", addr, fingerprint)
				} else {
					logger.Warnf("Auto-approving SSH connection from %s (fingerprint: %s) - running in no-TUI mode", addr, fingerprint)
				}
				return true
			}
		}

		// Set up input event handler to forward events from SSH to ClientManager
		sshSrv.OnInputEvent = func(event *protocol.InputEvent) {
			if cm := srv.GetClientManager(); cm != nil {
				logger.Debugf("[SSH-SERVER] Forwarding input event to ClientManager: type=%T, sourceId=%s", event.Event, event.SourceId)
				cm.HandleInputEvent(event)
			} else {
				logger.Warn("[SSH-SERVER] No ClientManager available to handle input event")
			}
		}
	} else {
		logger.Error("SSH server not initialized after Start()")
		return fmt.Errorf("SSH server not initialized")
	}

	logger.Info("Server components started successfully")

	// Connect ClientManager to UI if it's the redesigned model
	if cm := srv.GetClientManager(); cm != nil {
		if model != nil {
			// Set the client manager on the model
			model.SetClientManager(cm)

			// Set the server instance for proper shutdown
			model.SetServer(srv)

			// Set up activity callback to send logs to UI
			cm.SetOnActivity(func(level, message string) {
				if model != nil {
					model.AddLogEntry(ui.LogEntry{
						Timestamp: time.Now(),
						Level:     level,
						Message:   message,
					})

					// Trigger a client list refresh when clients connect/disconnect
					if strings.Contains(message, "Client connected:") || strings.Contains(message, "Client disconnected:") {
						// Send a refresh message to the UI
						if p := model.GetProgram(); p != nil {
							p.Send(ui.RefreshClientListMsg{})
						}
					}
				}
			})
		}

		// Set SSH server for sending events to clients
		if sshSrv := srv.GetNetworkServer(); sshSrv != nil {
			cm.SetSSHServer(sshSrv)
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

	// Start IPC socket server for switch commands
	if cm := srv.GetClientManager(); cm != nil {
		logger.Info("Starting IPC socket server...")
		ipcServer, err := ipc.NewSocketServer(cm)
		if err != nil {
			logger.Errorf("Failed to create IPC socket server: %v", err)
			// Don't fail server startup for IPC issues
		} else {
			if err := ipcServer.Start(); err != nil {
				logger.Errorf("Failed to start IPC socket server: %v", err)
			} else {
				logger.Info("IPC socket server started successfully")
				// Stop IPC server on shutdown
				go func() {
					<-ctx.Done()
					ipcServer.Stop()
				}()
			}
		}
	}

	return nil
}

func runServer(cmd *cobra.Command, args []string) error {
	// Check if running with sudo
	if os.Geteuid() != 0 {
		return fmt.Errorf("waymon server must be run with sudo")
	}

	// Set config path to system-wide location for server mode
	config.SetConfigPath("/etc/waymon/waymon.toml")

	// Initialize config before trying to use it
	if err := config.Init(); err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	// Get configuration first to check logging settings
	cfg := config.Get()

	// Apply log level from config BEFORE setting up file logging
	if cfg.Logging.LogLevel != "" {
		logger.SetLevel(cfg.Logging.LogLevel)
		// Don't log yet - file logging not set up
	}

	// Set up file logging if enabled
	var logFile *os.File
	if cfg.Logging.FileLogging {
		// Set up file logging since Bubble Tea will hide terminal output
		var err error
		logFile, err = logger.SetupFileLogging("SERVER")
		if err != nil {
			return fmt.Errorf("failed to setup file logging: %w", err)
		}
		// Now we can log after file logging is set up
		if cfg.Logging.LogLevel != "" {
			logger.Infof("Server command: Set log level to '%s' from config file", cfg.Logging.LogLevel)
			logger.Debugf("DEBUG TEST: This message should appear if debug logging is working")
		}
	}
	defer func() {
		if logFile != nil && logFile.Fd() != 0 {
			if err := logFile.Close(); err != nil {
				logger.Errorf("Failed to close log file: %v", err)
			}
		}
	}()

	// Input devices will be automatically detected by all-devices capture
	logger.Info("Using automatic all-devices input capture - no setup required!")

	// Show log location if not using TUI or in daemon mode
	if noTUI || daemon {
		if daemon {
			// Clean output for systemd
			logger.Info("Starting Waymon server in daemon mode")
			if logFile != nil {
				logger.Infof("Logging to: %s", logFile.Name())
			}
		} else if logFile != nil {
			fmt.Printf("Logging to: %s\n", logFile.Name())
		}
	}

	// Server runs as normal user and will request sudo when needed for uinput

	// uinput availability check is no longer needed - libei handles input injection

	// Ensure config file exists
	if err := ensureServerConfig(); err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	// Get configuration again after ensuring config
	cfg = config.Get()

	// Use flag values if provided, otherwise use config
	if serverPort == 0 {
		serverPort = cfg.Server.Port
	}
	if bindAddress == "" {
		bindAddress = cfg.Server.BindAddress
	}
	cfg.Server.Port = serverPort
	cfg.Server.BindAddress = bindAddress

	// Create a context that we'll cancel on shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var srv *server.Server
	var model *ui.ServerModel
	var p *tea.Program
	var uiDone <-chan struct{}

	// Skip UI if running in daemon mode
	if daemon {
		noTUI = true
	}

	if !noTUI {
		if debugTUI {
			// Use minimal debug UI
			debugModel := ui.NewDebugServerModel()
			p = tea.NewProgram(debugModel)
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
			// Use the new refactored UI runner
			var err error
			model, uiDone, err = ui.RunServerUI(ctx, serverPort, cfg.Server.Name, Version)
			if err != nil {
				return fmt.Errorf("failed to start server UI: %w", err)
			}
			// Set up logger to send log entries to UI
			logger.SetUINotifier(func(level, message string) {
				if model != nil {
					model.AddLogEntry(ui.LogEntry{
						Timestamp: time.Now(),
						Level:     level,
						Message:   message,
					})
				}
			})
		}
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

	if noTUI && srv != nil {
		// No TUI mode - initialize server immediately
		if err := initializeServer(ctx, srv, cfg, bindAddress, serverPort, nil, daemon); err != nil {
			return err
		}
	}

	// Handle graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// For non-debug TUI mode using refactored UI runner
	if !noTUI && !debugTUI {
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
				if p != nil {
					p.Send(tea.Quit())
				}
				return
			}

			logger.Info("Waymon server starting...")
			logger.Debug("DEBUG: This is a debug message to test log level")

			// Initialize server
			if err := initializeServer(ctx, srv, cfg, bindAddress, serverPort, model, false); err != nil {
				logger.Errorf("Server initialization failed: %v", err)
				// The refactored UI will handle shutdown
			}
		}()
	}

	// For debug TUI mode
	if debugTUI && p != nil {
		// Start server initialization similar to above
		go func() {
			time.Sleep(500 * time.Millisecond)

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

			if err := initializeServer(ctx, srv, cfg, bindAddress, serverPort, nil, false); err != nil {
				logger.Errorf("Server initialization failed: %v", err)
				p.Send(tea.Quit())
			}
		}()

		// Run debug TUI (blocking)
		if _, err := p.Run(); err != nil {
			return err
		}
	} else if noTUI {
		// No TUI mode - wait for shutdown signal
		if daemon {
			logger.Info("Waymon server daemon started successfully")
			logger.Infof("PID: %d", os.Getpid())
			logger.Infof("IPC socket: /tmp/waymon.sock")
			logger.Info("Use 'waymon status' to check server status")
		}
		select {
		case <-done:
			logger.Info("Received shutdown signal")
		case <-ctx.Done():
			logger.Info("Context cancelled")
		}
	} else {
		// Refactored UI runner handles its own lifecycle
		// Wait for UI to exit or signals
		select {
		case <-uiDone:
			logger.Info("UI exited")
		case <-done:
			logger.Info("Received shutdown signal")
		case <-ctx.Done():
			logger.Info("Context cancelled")
		}
	}

	// Cancel context to start shutdown
	cancel()

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Graceful shutdown
	logger.Info("Stopping Waymon server...")

	if srv != nil {
		// Stop server gracefully
		srv.Stop()
	}

	// Wait for server to finish or timeout
	select {
	case <-shutdownCtx.Done():
		if shutdownCtx.Err() == context.DeadlineExceeded {
			logger.Warn("Server shutdown timed out")
		}
	case <-time.After(100 * time.Millisecond):
		// Small delay to allow server to finish
	}

	logger.Info("Waymon server stopped")
	return nil
}

// ensureServerConfig ensures the config file exists when running as server
func ensureServerConfig() error {
	configPath := config.GetConfigPath()

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logger.Infof("No config file found. Creating default config at %s", configPath)

		// Create /etc/waymon directory if it doesn't exist
		configDir := filepath.Dir(configPath)
		if err := os.MkdirAll(configDir, 0750); err != nil {
			return fmt.Errorf("failed to create config directory %s: %w", configDir, err)
		}

		// Save default config
		if err := config.Save(); err != nil {
			return err
		}

		logger.Info("Default configuration created successfully")
	}

	return nil
}
