package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/logger"
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

		logger.Info("Current Configuration:")
		logger.Infof("Config file: %s\n", config.GetConfigPath())

		logger.Info("[Server]")
		logger.Infof("  Port: %d", cfg.Server.Port)
		logger.Infof("  Bind Address: %s", cfg.Server.BindAddress)
		logger.Infof("  Name: %s", cfg.Server.Name)
		logger.Infof("  Max Clients: %d", cfg.Server.MaxClients)
		logger.Infof("  SSH Host Key: %s", cfg.Server.SSHHostKeyPath)
		logger.Infof("  SSH Authorized Keys: %s", cfg.Server.SSHAuthKeysPath)
		logger.Infof("  SSH Whitelist Only: %v", cfg.Server.SSHWhitelistOnly)
		if len(cfg.Server.SSHWhitelist) > 0 {
			logger.Info("  SSH Whitelist:")
			for _, fp := range cfg.Server.SSHWhitelist {
				logger.Infof("    - %s", fp)
			}
		}

		logger.Info("\n[Client]")
		logger.Infof("  Server Address: %s", cfg.Client.ServerAddress)
		logger.Infof("  Auto Connect: %v", cfg.Client.AutoConnect)
		logger.Infof("  Reconnect Delay: %d seconds", cfg.Client.ReconnectDelay)
		logger.Infof("  Edge Threshold: %d pixels", cfg.Client.EdgeThreshold)
		logger.Infof("  Hotkey: %s+%s", cfg.Client.HotkeyModifier, cfg.Client.HotkeyKey)

		logger.Info("\n[Display]")
		logger.Infof("  Refresh Interval: %d seconds", cfg.Display.RefreshInterval)
		logger.Infof("  Backend: %s", cfg.Display.Backend)
		logger.Infof("  Cursor Tracking: %v", cfg.Display.CursorTracking)

		logger.Info("\n[Input]")
		logger.Infof("  Mouse Sensitivity: %.2f", cfg.Input.MouseSensitivity)
		logger.Infof("  Scroll Speed: %.2f", cfg.Input.ScrollSpeed)
		logger.Infof("  Keyboard Enabled: %v", cfg.Input.EnableKeyboard)
		logger.Infof("  Keyboard Layout: %s", cfg.Input.KeyboardLayout)

		if len(cfg.Hosts) > 0 {
			logger.Info("\n[Hosts]")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			if _, err := fmt.Fprintln(w, "  Name\tAddress\tPosition\tAuth"); err != nil {
				logger.Errorf("Failed to write header: %v", err)
			}
			for _, host := range cfg.Hosts {
				auth := "No"
				if host.AuthToken != "" {
					auth = "Yes"
				}
				if _, err := fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", host.Name, host.Address, host.Position, auth); err != nil {
					logger.Errorf("Failed to write host info: %v", err)
				}
			}
			if err := w.Flush(); err != nil {
				logger.Errorf("Failed to flush writer: %v", err)
			}
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
		logger.Infof("Configuration saved to: %s", config.GetConfigPath())
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

		logger.Infof("Added host '%s' at %s (%s)", name, address, position)
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

		logger.Infof("Removed host '%s'", name)
		return nil
	},
}

var configHostListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured hosts",
	RunE: func(cmd *cobra.Command, args []string) error {
		hosts := config.ListHosts()

		if len(hosts) == 0 {
			logger.Info("No hosts configured")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "Name\tAddress\tPosition\tAuth"); err != nil {
			logger.Errorf("Failed to write header: %v", err)
		}
		if _, err := fmt.Fprintln(w, "----\t-------\t--------\t----"); err != nil {
			logger.Errorf("Failed to write separator: %v", err)
		}

		for _, host := range hosts {
			auth := "No"
			if host.AuthToken != "" {
				auth = "Yes"
			}
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", host.Name, host.Address, host.Position, auth); err != nil {
				logger.Errorf("Failed to write host: %v", err)
			}
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
			logger.Infof("Configuration file already exists at: %s", configPath)
			logger.Info("Use --force to overwrite")

			force, _ := cmd.Flags().GetBool("force")
			if !force {
				return nil
			}
		}

		// Save default configuration
		if err := config.Save(); err != nil {
			return err
		}

		logger.Infof("Configuration initialized at: %s", configPath)
		logger.Info("\nYou can now:")
		logger.Info("  - Edit the configuration file directly")
		logger.Info("  - Use 'waymon config host add' to add hosts")
		logger.Info("  - Use 'waymon config show' to view current settings")

		return nil
	},
}

var configSSHCmd = &cobra.Command{
	Use:   "ssh",
	Short: "Manage SSH whitelist",
}

var configSSHListCmd = &cobra.Command{
	Use:   "list",
	Short: "List whitelisted SSH keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Get()

		if len(cfg.Server.SSHWhitelist) == 0 {
			logger.Info("No SSH keys in whitelist")
			if cfg.Server.SSHWhitelistOnly {
				logger.Info("\nWhitelist-only mode is ENABLED")
				logger.Info("New connections will require approval")
			} else {
				logger.Info("\nWhitelist-only mode is DISABLED")
				logger.Info("All SSH keys are accepted")
			}
			return nil
		}

		logger.Info("Whitelisted SSH Keys:")
		for i, fp := range cfg.Server.SSHWhitelist {
			logger.Infof("%d. %s", i+1, fp)
		}

		if cfg.Server.SSHWhitelistOnly {
			logger.Info("\nWhitelist-only mode is ENABLED")
		} else {
			logger.Info("\nWhitelist-only mode is DISABLED")
		}

		return nil
	},
}

var configSSHRemoveCmd = &cobra.Command{
	Use:   "remove <fingerprint>",
	Short: "Remove SSH key from whitelist",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fingerprint := args[0]

		if err := config.RemoveSSHKeyFromWhitelist(fingerprint); err != nil {
			return err
		}

		logger.Infof("Removed SSH key from whitelist: %s", fingerprint)
		return nil
	},
}

var configSSHClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all SSH keys from whitelist",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Get()
		count := len(cfg.Server.SSHWhitelist)

		if count == 0 {
			logger.Info("Whitelist is already empty")
			return nil
		}

		// Clear whitelist
		cfg.Server.SSHWhitelist = []string{}
		if err := config.UpdateServer(cfg.Server); err != nil {
			return err
		}

		logger.Infof("Cleared %d SSH key(s) from whitelist", count)
		return nil
	},
}

func init() {
	// Add subcommands
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSaveCmd)
	configCmd.AddCommand(configHostCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configSSHCmd)

	// Add host subcommands
	configHostCmd.AddCommand(configHostAddCmd)
	configHostCmd.AddCommand(configHostRemoveCmd)
	configHostCmd.AddCommand(configHostListCmd)

	// Add SSH subcommands
	configSSHCmd.AddCommand(configSSHListCmd)
	configSSHCmd.AddCommand(configSSHRemoveCmd)
	configSSHCmd.AddCommand(configSSHClearCmd)

	// Add flags
	configHostAddCmd.Flags().String("auth-token", "", "Authentication token for the host")
	configInitCmd.Flags().Bool("force", false, "Force overwrite existing configuration")
}
