package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/ipc"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/protocol"
	"github.com/bnema/waymon/internal/server"
	"github.com/bnema/waymon/internal/setup"
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
			// Register client with ClientManager
			if cm := srv.GetClientManager(); cm != nil {
				// Generate client ID from address (simple for now)
				clientID := addr
				clientName := addr // Use address as name for now
				cm.RegisterClient(clientID, clientName, addr)
			}

			// Send UI notification
			if p != nil {
				p.Send(ui.ClientConnectedMsg{ClientAddr: addr})
			} else {
				logger.Infof("Client connected from %s", addr)
			}
		}
		sshSrv.OnClientDisconnected = func(addr string) {
			// Unregister client from ClientManager
			if cm := srv.GetClientManager(); cm != nil {
				clientID := addr
				cm.UnregisterClient(clientID)
			}

			// Send UI notification
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
		
		// Set up client connection handlers
		sshSrv.OnClientConnected = func(addr, publicKey string) {
			if cm := srv.GetClientManager(); cm != nil {
				// Use address as both ID and name initially
				// The client will send its actual configuration later
				cm.RegisterClient(addr, addr, addr)
				logger.Infof("Client connected and registered: %s", addr)
			}
		}
		
		sshSrv.OnClientDisconnected = func(addr string) {
			if cm := srv.GetClientManager(); cm != nil {
				cm.UnregisterClient(addr)
				logger.Infof("Client disconnected and unregistered: %s", addr)
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
	if cm := srv.GetClientManager(); cm != nil && p != nil {
		// Send a message to set the client manager
		p.Send(ui.SetClientManagerMsg{ClientManager: cm})

		// Send a message to set the server instance for proper shutdown
		p.Send(ui.SetServerMsg{Server: srv})

		// Set up activity callback to send logs to UI
		cm.SetOnActivity(func(level, message string) {
			if p != nil {
				logEntry := ui.LogEntry{
					Timestamp: time.Now(),
					Level:     level,
					Message:   message,
				}
				p.Send(ui.LogMsg{Entry: logEntry})
			}
		})

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

	// Set up file logging since Bubble Tea will hide terminal output
	logFile, err := logger.SetupFileLogging("SERVER")
	if err != nil {
		return fmt.Errorf("failed to setup file logging: %w", err)
	}
	defer logFile.Close()

	// Get configuration
	cfg := config.Get()

	// Run device setup for evdev input capture
	deviceSetup := setup.NewDeviceSetup()
	
	// Validate existing devices first
	if err := deviceSetup.ValidateDevices(); err != nil {
		logger.Warnf("Device validation failed: %v", err)
		
		// If we have invalid devices, run interactive setup
		if !noTUI {
			fmt.Println("⚠️  Device configuration issue detected.")
			if err := deviceSetup.RunInteractiveSetup(); err != nil {
				return fmt.Errorf("device setup failed: %w", err)
			}
		} else {
			logger.Error("Invalid device configuration. Run 'waymon setup --devices' to fix.")
			return fmt.Errorf("invalid device configuration: %w", err)
		}
	} else {
		// Check if devices need initial setup
		if err := deviceSetup.RunInteractiveSetup(); err != nil {
			if !noTUI {
				return fmt.Errorf("device setup failed: %w", err)
			}
			// In no-TUI mode, warn but continue (devices will be auto-detected)
			logger.Warnf("Device setup failed, will use auto-detection: %v", err)
		}
	}

	// Show log location if not using TUI
	if noTUI {
		fmt.Printf("Logging to: %s\n", logFile.Name())
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

	var p *tea.Program
	var srv *server.Server

	if !noTUI {
		if debugTUI {
			// Use minimal debug UI
			debugModel := ui.NewDebugServerModel()
			p = tea.NewProgram(debugModel)
		} else {
			// Create redesigned full-screen TUI model
			model := ui.NewServerModel(serverPort, cfg.Server.Name, Version)
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

		// Create /etc/waymon directory if it doesn't exist
		configDir := filepath.Dir(configPath)
		if err := os.MkdirAll(configDir, 0755); err != nil {
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
