package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/display"
	"github.com/bnema/waymon/internal/input"
	"github.com/bnema/waymon/internal/network"
	"github.com/bnema/waymon/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	serverPort int
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

	// Initialize display detection
	disp, err := display.New()
	if err != nil {
		return fmt.Errorf("failed to initialize display: %w", err)
	}
	defer disp.Close()

	// Show monitor configuration
	monitors := disp.GetMonitors()
	fmt.Printf("Detected %d monitor(s):\n", len(monitors))
	for _, mon := range monitors {
		fmt.Printf("  %s: %dx%d at (%d,%d)\n", mon.Name, mon.Width, mon.Height, mon.X, mon.Y)
	}

	// Initialize input handler
	inputHandler, err := input.NewHandler()
	if err != nil {
		return fmt.Errorf("failed to initialize input handler: %w", err)
	}
	defer inputHandler.Close()

	// Get configuration
	cfg := config.Get()
	
	// Use flag values if provided, otherwise use config
	if serverPort == 0 {
		serverPort = cfg.Server.Port
	}
	if bindAddress == "" {
		bindAddress = cfg.Server.BindAddress
	}
	
	// Show server info
	fmt.Printf("Starting Waymon server '%s' on %s:%d\n", cfg.Server.Name, bindAddress, serverPort)
	if cfg.Server.RequireAuth {
		fmt.Println("Authentication enabled")
	}
	
	// Create server
	server := network.NewServer(serverPort)

	// Start server in background
	errCh := make(chan error)
	go func() {
		if err := server.Start(context.Background()); err != nil {
			errCh <- err
		}
	}()

	// Create TUI
	monitorList := []ui.Monitor{}
	for _, mon := range monitors {
		monitorList = append(monitorList, ui.Monitor{
			Name:     mon.Name,
			Size:     fmt.Sprintf("%dx%d", mon.Width, mon.Height),
			Position: fmt.Sprintf("%d,%d", mon.X, mon.Y),
			Primary:  mon.Primary,
		})
	}
	
	model := ui.NewServerModel(ui.ServerConfig{
		Port:     serverPort,
		Name:     cfg.Server.Name,
		Monitors: monitorList,
	})
	
	p := tea.NewProgram(model)

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-sigCh:
			p.Send(tea.Quit())
		case err := <-errCh:
			log.Printf("Server error: %v", err)
			p.Send(tea.Quit())
		}
	}()

	// Run TUI
	if _, err := p.Run(); err != nil {
		return err
	}

	// Cleanup
	server.Stop()
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

