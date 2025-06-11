package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/display"
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
	fmt.Printf("Detected %d monitor(s):\n", len(monitors))
	for _, mon := range monitors {
		fmt.Printf("  %s: %dx%d at (%d,%d)\n", mon.Name, mon.Width, mon.Height, mon.X, mon.Y)
	}

	// Create client
	client := network.NewClient()

	// Connect to server
	if err := client.Connect(context.Background(), serverAddr); err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer client.Disconnect()

	// TODO: Create edge detector when implementing mouse capture
	// edgeDetector := &EdgeDetector{
	// 	display:   disp,
	// 	client:    client,
	// 	threshold: int32(edgeSize),
	// 	active:    false,
	// }

	// Create TUI
	model := ui.NewClientModel(ui.ClientConfig{
		ServerAddress: serverAddr,
		ServerName:    hostName,
		EdgeThreshold: edgeSize,
	})
	
	p := tea.NewProgram(model)

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

// EdgeDetector monitors cursor position and triggers edge events
type EdgeDetector struct {
	display   *display.Display
	client    *network.Client
	threshold int32
	active    bool
	lastEdge  display.Edge
	capturing bool
}

func (e *EdgeDetector) Start() {
	e.active = true
	// TODO: Start monitoring cursor position
	// This would use a separate goroutine to poll cursor position
	// and detect when it hits screen edges
}

func (e *EdgeDetector) Stop() {
	e.active = false
}