package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/bnema/waymon/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Waymon configuration",
	Long:  `Manage Waymon configuration including hosts and settings.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Get()
		
		fmt.Println("Current Configuration:")
		fmt.Printf("Config file: %s\n\n", config.GetConfigPath())
		
		fmt.Println("[Server]")
		fmt.Printf("  Port: %d\n", cfg.Server.Port)
		fmt.Printf("  Bind Address: %s\n", cfg.Server.BindAddress)
		fmt.Printf("  Name: %s\n", cfg.Server.Name)
		fmt.Printf("  Require Auth: %v\n", cfg.Server.RequireAuth)
		fmt.Printf("  Max Clients: %d\n", cfg.Server.MaxClients)
		fmt.Printf("  TLS Enabled: %v\n", cfg.Server.EnableTLS)
		
		fmt.Println("\n[Client]")
		fmt.Printf("  Server Address: %s\n", cfg.Client.ServerAddress)
		fmt.Printf("  Auto Connect: %v\n", cfg.Client.AutoConnect)
		fmt.Printf("  Reconnect Delay: %d seconds\n", cfg.Client.ReconnectDelay)
		fmt.Printf("  Edge Threshold: %d pixels\n", cfg.Client.EdgeThreshold)
		fmt.Printf("  Hotkey: %s+%s\n", cfg.Client.HotkeyModifier, cfg.Client.HotkeyKey)
		
		fmt.Println("\n[Display]")
		fmt.Printf("  Refresh Interval: %d seconds\n", cfg.Display.RefreshInterval)
		fmt.Printf("  Backend: %s\n", cfg.Display.Backend)
		fmt.Printf("  Cursor Tracking: %v\n", cfg.Display.CursorTracking)
		
		fmt.Println("\n[Input]")
		fmt.Printf("  Mouse Sensitivity: %.2f\n", cfg.Input.MouseSensitivity)
		fmt.Printf("  Scroll Speed: %.2f\n", cfg.Input.ScrollSpeed)
		fmt.Printf("  Keyboard Enabled: %v\n", cfg.Input.EnableKeyboard)
		fmt.Printf("  Keyboard Layout: %s\n", cfg.Input.KeyboardLayout)
		
		if len(cfg.Hosts) > 0 {
			fmt.Println("\n[Hosts]")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  Name\tAddress\tPosition\tAuth")
			for _, host := range cfg.Hosts {
				auth := "No"
				if host.AuthToken != "" {
					auth = "Yes"
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", host.Name, host.Address, host.Position, auth)
			}
			w.Flush()
		}
		
		return nil
	},
}

var configSaveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save current configuration to file",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Save(); err != nil {
			return err
		}
		fmt.Printf("Configuration saved to: %s\n", config.GetConfigPath())
		return nil
	},
}

var configHostCmd = &cobra.Command{
	Use:   "host",
	Short: "Manage host configurations",
}

var configHostAddCmd = &cobra.Command{
	Use:   "add <name> <address> <position>",
	Short: "Add a new host",
	Long:  `Add a new host to the configuration. Position can be: left, right, top, bottom`,
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		address := args[1]
		position := args[2]
		
		// Validate position
		validPositions := map[string]bool{
			"left": true, "right": true, "top": true, "bottom": true,
		}
		if !validPositions[position] {
			return fmt.Errorf("invalid position: %s (must be left, right, top, or bottom)", position)
		}
		
		// Get auth token if provided
		authToken, _ := cmd.Flags().GetString("auth-token")
		
		host := config.HostConfig{
			Name:      name,
			Address:   address,
			Position:  position,
			AuthToken: authToken,
		}
		
		if err := config.AddHost(host); err != nil {
			return err
		}
		
		fmt.Printf("Added host '%s' at %s (%s)\n", name, address, position)
		return nil
	},
}

var configHostRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a host",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		
		if err := config.RemoveHost(name); err != nil {
			return err
		}
		
		fmt.Printf("Removed host '%s'\n", name)
		return nil
	},
}

var configHostListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured hosts",
	RunE: func(cmd *cobra.Command, args []string) error {
		hosts := config.ListHosts()
		
		if len(hosts) == 0 {
			fmt.Println("No hosts configured")
			return nil
		}
		
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "Name\tAddress\tPosition\tAuth")
		fmt.Fprintln(w, "----\t-------\t--------\t----")
		
		for _, host := range hosts {
			auth := "No"
			if host.AuthToken != "" {
				auth = "Yes"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", host.Name, host.Address, host.Position, auth)
		}
		
		return w.Flush()
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file with defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if config already exists
		configPath := config.GetConfigPath()
		if _, err := os.Stat(configPath); err == nil {
			fmt.Printf("Configuration file already exists at: %s\n", configPath)
			fmt.Println("Use --force to overwrite")
			
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				return nil
			}
		}
		
		// Save default configuration
		if err := config.Save(); err != nil {
			return err
		}
		
		fmt.Printf("Configuration initialized at: %s\n", configPath)
		fmt.Println("\nYou can now:")
		fmt.Println("  - Edit the configuration file directly")
		fmt.Println("  - Use 'waymon config host add' to add hosts")
		fmt.Println("  - Use 'waymon config show' to view current settings")
		
		return nil
	},
}

func init() {
	// Add subcommands
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSaveCmd)
	configCmd.AddCommand(configHostCmd)
	configCmd.AddCommand(configInitCmd)
	
	// Add host subcommands
	configHostCmd.AddCommand(configHostAddCmd)
	configHostCmd.AddCommand(configHostRemoveCmd)
	configHostCmd.AddCommand(configHostListCmd)
	
	// Add flags
	configHostAddCmd.Flags().String("auth-token", "", "Authentication token for the host")
	configInitCmd.Flags().Bool("force", false, "Force overwrite existing configuration")
}