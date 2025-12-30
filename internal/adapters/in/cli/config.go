package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/bnema/waymon/internal/domain"
	"github.com/spf13/cobra"
)

func (c *CLI) newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Waymon configuration",
		Long:  `Manage Waymon configuration including hosts and settings.`,
	}

	// Add subcommands
	cmd.AddCommand(c.newConfigShowCmd())
	cmd.AddCommand(c.newConfigSaveCmd())
	cmd.AddCommand(c.newConfigInitCmd())
	cmd.AddCommand(c.newConfigHostCmd())
	cmd.AddCommand(c.newConfigSSHCmd())

	return cmd
}

func (c *CLI) newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.runConfigShow(cmd.Context())
		},
	}
}

func (c *CLI) runConfigShow(ctx context.Context) error {
	if c.configRepo == nil {
		return fmt.Errorf("config repository not available - CLI not properly initialized")
	}

	cfg, err := c.configRepo.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println("Current Configuration:")
	fmt.Printf("Config file: %s\n\n", c.configRepo.GetConfigPath())

	fmt.Println("[Server]")
	fmt.Printf("  Port: %d\n", cfg.Server.Port)
	fmt.Printf("  Bind Address: %s\n", cfg.Server.BindAddress)
	fmt.Printf("  Name: %s\n", cfg.Server.Name)
	fmt.Printf("  Max Clients: %d\n", cfg.Server.MaxClients)
	fmt.Printf("  SSH Host Key: %s\n", cfg.Server.SSHHostKeyPath)
	fmt.Printf("  SSH Authorized Keys: %s\n", cfg.Server.SSHAuthKeysPath)
	fmt.Printf("  SSH Whitelist Only: %v\n", cfg.Server.SSHWhitelistOnly)
	if len(cfg.Server.SSHWhitelist) > 0 {
		fmt.Println("  SSH Whitelist:")
		for _, fp := range cfg.Server.SSHWhitelist {
			fmt.Printf("    - %s\n", fp)
		}
	}

	fmt.Println("\n[Client]")
	fmt.Printf("  Server Address: %s\n", cfg.Client.ServerAddress)
	fmt.Printf("  Auto Connect: %v\n", cfg.Client.AutoConnect)
	fmt.Printf("  Reconnect Delay: %d seconds\n", cfg.Client.ReconnectDelay)
	fmt.Printf("  Edge Threshold: %d pixels\n", cfg.Client.EdgeThreshold)
	fmt.Printf("  Hotkey: %s+%s\n", cfg.Client.HotkeyModifier, cfg.Client.HotkeyKey)

	if len(cfg.Hosts) > 0 {
		fmt.Println("\n[Hosts]")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "  Name\tAddress\tPosition"); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
		for _, host := range cfg.Hosts {
			if _, err := fmt.Fprintf(w, "  %s\t%s\t%s\n", host.Name, host.Address, host.Position); err != nil {
				return fmt.Errorf("failed to write host: %w", err)
			}
		}
		if err := w.Flush(); err != nil {
			return fmt.Errorf("failed to flush output: %w", err)
		}
	}

	return nil
}

func (c *CLI) newConfigSaveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "save",
		Short: "Save current configuration to file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.runConfigSave(cmd.Context())
		},
	}
}

func (c *CLI) runConfigSave(ctx context.Context) error {
	if c.configRepo == nil {
		return fmt.Errorf("config repository not available - CLI not properly initialized")
	}

	cfg, err := c.configRepo.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := c.configRepo.Save(ctx, cfg); err != nil {
		return err
	}

	fmt.Printf("Configuration saved to: %s\n", c.configRepo.GetConfigPath())
	return nil
}

func (c *CLI) newConfigInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize configuration file with defaults",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.runConfigInit(cmd.Context(), force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force overwrite existing configuration")

	return cmd
}

func (c *CLI) runConfigInit(ctx context.Context, force bool) error {
	if c.configRepo == nil {
		return fmt.Errorf("config repository not available - CLI not properly initialized")
	}

	configPath := c.configRepo.GetConfigPath()

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil && !force {
		fmt.Printf("Configuration file already exists at: %s\n", configPath)
		fmt.Println("Use --force to overwrite")
		return nil
	}

	// Load defaults and save
	cfg, err := c.configRepo.Load(ctx)
	if err != nil {
		// Use empty config with defaults
		cfg = &domain.Config{}
	}

	if err := c.configRepo.Save(ctx, cfg); err != nil {
		return err
	}

	fmt.Printf("Configuration initialized at: %s\n", configPath)
	fmt.Println("\nYou can now:")
	fmt.Println("  - Edit the configuration file directly")
	fmt.Println("  - Use 'waymon config host add' to add hosts")
	fmt.Println("  - Use 'waymon config show' to view current settings")

	return nil
}

func (c *CLI) newConfigHostCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "host",
		Short: "Manage host configurations",
	}

	cmd.AddCommand(c.newConfigHostAddCmd())
	cmd.AddCommand(c.newConfigHostRemoveCmd())
	cmd.AddCommand(c.newConfigHostListCmd())

	return cmd
}

func (c *CLI) newConfigHostAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <address> <position>",
		Short: "Add a new host",
		Long:  `Add a new host to the configuration. Position can be: left, right, top, bottom`,
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runConfigHostAdd(cmd.Context(), args[0], args[1], args[2])
		},
	}
}

func (c *CLI) runConfigHostAdd(ctx context.Context, name, address, position string) error {
	if c.configRepo == nil {
		return fmt.Errorf("config repository not available - CLI not properly initialized")
	}

	// Validate position
	validPositions := map[string]bool{
		"left": true, "right": true, "top": true, "bottom": true,
	}
	if !validPositions[position] {
		return fmt.Errorf("invalid position: %s (must be left, right, top, or bottom)", position)
	}

	cfg, err := c.configRepo.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check for duplicate
	for _, h := range cfg.Hosts {
		if h.Name == name {
			return fmt.Errorf("host '%s' already exists", name)
		}
	}

	// Add host
	cfg.Hosts = append(cfg.Hosts, domain.HostConfig{
		Name:     name,
		Address:  address,
		Position: position,
	})

	if err := c.configRepo.Save(ctx, cfg); err != nil {
		return err
	}

	fmt.Printf("Added host '%s' at %s (%s)\n", name, address, position)
	return nil
}

func (c *CLI) newConfigHostRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runConfigHostRemove(cmd.Context(), args[0])
		},
	}
}

func (c *CLI) runConfigHostRemove(ctx context.Context, name string) error {
	if c.configRepo == nil {
		return fmt.Errorf("config repository not available - CLI not properly initialized")
	}

	cfg, err := c.configRepo.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find and remove host
	found := false
	newHosts := make([]domain.HostConfig, 0, len(cfg.Hosts))
	for _, h := range cfg.Hosts {
		if h.Name == name {
			found = true
		} else {
			newHosts = append(newHosts, h)
		}
	}

	if !found {
		return fmt.Errorf("host '%s' not found", name)
	}

	cfg.Hosts = newHosts

	if err := c.configRepo.Save(ctx, cfg); err != nil {
		return err
	}

	fmt.Printf("Removed host '%s'\n", name)
	return nil
}

func (c *CLI) newConfigHostListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured hosts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.runConfigHostList(cmd.Context())
		},
	}
}

func (c *CLI) runConfigHostList(ctx context.Context) error {
	if c.configRepo == nil {
		return fmt.Errorf("config repository not available - CLI not properly initialized")
	}

	cfg, err := c.configRepo.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Hosts) == 0 {
		fmt.Println("No hosts configured")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "Name\tAddress\tPosition"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if _, err := fmt.Fprintln(w, "----\t-------\t--------"); err != nil {
		return fmt.Errorf("failed to write separator: %w", err)
	}

	for _, host := range cfg.Hosts {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", host.Name, host.Address, host.Position); err != nil {
			return fmt.Errorf("failed to write host: %w", err)
		}
	}

	return w.Flush()
}

func (c *CLI) newConfigSSHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Manage SSH whitelist",
	}

	cmd.AddCommand(c.newConfigSSHListCmd())
	cmd.AddCommand(c.newConfigSSHRemoveCmd())
	cmd.AddCommand(c.newConfigSSHClearCmd())

	return cmd
}

func (c *CLI) newConfigSSHListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List whitelisted SSH keys",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.runConfigSSHList(cmd.Context())
		},
	}
}

func (c *CLI) runConfigSSHList(ctx context.Context) error {
	if c.configRepo == nil {
		return fmt.Errorf("config repository not available - CLI not properly initialized")
	}

	cfg, err := c.configRepo.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Server.SSHWhitelist) == 0 {
		fmt.Println("No SSH keys in whitelist")
		if cfg.Server.SSHWhitelistOnly {
			fmt.Println("\nWhitelist-only mode is ENABLED")
			fmt.Println("New connections will require approval")
		} else {
			fmt.Println("\nWhitelist-only mode is DISABLED")
			fmt.Println("All SSH keys are accepted")
		}
		return nil
	}

	fmt.Println("Whitelisted SSH Keys:")
	for i, fp := range cfg.Server.SSHWhitelist {
		fmt.Printf("%d. %s\n", i+1, fp)
	}

	if cfg.Server.SSHWhitelistOnly {
		fmt.Println("\nWhitelist-only mode is ENABLED")
	} else {
		fmt.Println("\nWhitelist-only mode is DISABLED")
	}

	return nil
}

func (c *CLI) newConfigSSHRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <fingerprint>",
		Short: "Remove SSH key from whitelist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runConfigSSHRemove(cmd.Context(), args[0])
		},
	}
}

func (c *CLI) runConfigSSHRemove(ctx context.Context, fingerprint string) error {
	if c.configRepo == nil {
		return fmt.Errorf("config repository not available - CLI not properly initialized")
	}

	cfg, err := c.configRepo.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find and remove fingerprint
	found := false
	newWhitelist := make([]string, 0, len(cfg.Server.SSHWhitelist))
	for _, fp := range cfg.Server.SSHWhitelist {
		if fp == fingerprint {
			found = true
		} else {
			newWhitelist = append(newWhitelist, fp)
		}
	}

	if !found {
		return fmt.Errorf("fingerprint not found in whitelist")
	}

	cfg.Server.SSHWhitelist = newWhitelist

	if err := c.configRepo.Save(ctx, cfg); err != nil {
		return err
	}

	fmt.Printf("Removed SSH key from whitelist: %s\n", fingerprint)
	return nil
}

func (c *CLI) newConfigSSHClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear all SSH keys from whitelist",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.runConfigSSHClear(cmd.Context())
		},
	}
}

func (c *CLI) runConfigSSHClear(ctx context.Context) error {
	if c.configRepo == nil {
		return fmt.Errorf("config repository not available - CLI not properly initialized")
	}

	cfg, err := c.configRepo.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	count := len(cfg.Server.SSHWhitelist)
	if count == 0 {
		fmt.Println("Whitelist is already empty")
		return nil
	}

	cfg.Server.SSHWhitelist = []string{}

	if err := c.configRepo.Save(ctx, cfg); err != nil {
		return err
	}

	fmt.Printf("Cleared %d SSH key(s) from whitelist\n", count)
	return nil
}
