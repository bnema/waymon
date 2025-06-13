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

	// Verify uinput setup has been completed
	if err := VerifyUinputSetup(); err != nil {
		return fmt.Errorf("uinput setup verification failed: %w", err)
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

	// Create privilege-separated server
	srv, err := server.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Create inline TUI first so we can reference it in callbacks
	model := ui.NewInlineServerModel(serverPort, cfg.Server.Name)
	p := tea.NewProgram(model)

	// Set up client connection callbacks
	if sshSrv := srv.GetNetworkServer(); sshSrv != nil {
		sshSrv.OnClientConnected = func(addr, publicKey string) {
			p.Send(ui.ClientConnectedMsg{ClientAddr: addr})
		}
		sshSrv.OnClientDisconnected = func(addr string) {
			p.Send(ui.ClientDisconnectedMsg{ClientAddr: addr})
		}
		sshSrv.OnAuthRequest = func(addr, publicKey, fingerprint string) bool {
			// Send auth request to UI
			p.Send(ui.SSHAuthRequestMsg{
				ClientAddr:  addr,
				PublicKey:   publicKey,
				Fingerprint: fingerprint,
			})
			
			// Wait for approval from UI
			if authChan := model.GetAuthChannel(); authChan != nil {
				select {
				case approved := <-authChan:
					return approved
				case <-time.After(30 * time.Second):
					// Timeout after 30 seconds
					return false
				}
			}
			return false
		}
	}

	// Create a context that we'll cancel on shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the server with context
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	defer srv.Stop()

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
	logger.Infof("\nStarting Waymon SSH server '%s' on %s:%d", cfg.Server.Name, bindAddress, serverPort)
	// Get the actual expanded paths from the server
	if sshSrv := srv.GetNetworkServer(); sshSrv != nil {
		logger.Infof("SSH Host Key: %s", srv.GetSSHHostKeyPath())
		logger.Infof("SSH Authorized Keys: %s", srv.GetSSHAuthKeysPath())
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		// Cancel context first to start shutdown
		cancel()
		// Then tell TUI to quit
		p.Send(tea.Quit())
	}()

	// Run TUI
	if _, err := p.Run(); err != nil {
		return err
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
