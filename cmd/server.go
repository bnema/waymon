package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bnema/waymon/internal/config"
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

	// Start the server
	if err := srv.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	defer srv.Stop()

	// Show monitor configuration
	if disp := srv.GetDisplay(); disp != nil {
		monitors := disp.GetMonitors()
		fmt.Printf("Detected %d monitor(s):\n", len(monitors))
		for _, mon := range monitors {
			fmt.Printf("  %s: %dx%d at (%d,%d)", mon.Name, mon.Width, mon.Height, mon.X, mon.Y)
			if mon.Primary {
				fmt.Printf(" [PRIMARY]")
			}
			if mon.Scale != 1.0 {
				fmt.Printf(" scale=%.1f", mon.Scale)
			}
			fmt.Println()
		}
	}

	// Show server info
	fmt.Printf("\nStarting Waymon server '%s' on %s:%d\n", cfg.Server.Name, bindAddress, serverPort)
	if cfg.Server.RequireAuth {
		fmt.Println("Authentication enabled")
	}

	// Create simple TUI
	model := ui.NewSimpleServerModel(srv.GetPort(), srv.GetName())

	p := tea.NewProgram(model, tea.WithAltScreen())

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

// ensureServerConfig ensures the config file exists when running as server
func ensureServerConfig() error {
	configPath := config.GetConfigPath()

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("No config file found. Creating default config at %s\n", configPath)

		// Save default config
		if err := config.Save(); err != nil {
			return err
		}

		fmt.Println("Default configuration created successfully")
	}

	return nil
}
