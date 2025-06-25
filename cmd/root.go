package cmd

import (
	"os"

	"github.com/bnema/waymon/internal/config"
	"github.com/bnema/waymon/internal/logger"
	"github.com/spf13/cobra"
)

var (
	logLevel string

	rootCmd = &cobra.Command{
		Use:   "waymon",
		Short: "Waymon - Wayland mouse sharing",
		Long: `Waymon is a client/server mouse sharing application for Wayland systems.
It allows seamless mouse movement between two computers on a local network,
working around Wayland's security restrictions by using the uinput kernel module.`,
		SilenceUsage: true,
	}
)

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Initialize configuration
	cobra.OnInitialize(initConfig)

	rootCmd.Version = Version
	rootCmd.SetVersionTemplate(`{{with .Name}}{{printf "%s " .}}{{end}}{{printf "version %s\n" .Version}}`)

	// Add global flags
	rootCmd.PersistentFlags().String("config", "", "config file (default is $HOME/.config/waymon/waymon.toml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "set log level (debug, info, warn, error, fatal)")

	// Add commands
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(clientCmd)
	rootCmd.AddCommand(configCmd)
}

// initConfig reads in config file
func initConfig() {
	// Set log level from flag if provided
	if logLevel != "" {
		logger.SetLevel(logLevel)
		logger.Infof("Setting log level to '%s' from command line flag", logLevel)
	}

	if err := config.Init(); err != nil {
		logger.Warnf("Warning: %v", err)
		return
	}

	// Apply log level from config file if not overridden by flag
	if logLevel == "" {
		cfg := config.Get()
		if cfg.Logging.LogLevel != "" {
			logger.SetLevel(cfg.Logging.LogLevel)
			logger.Infof("Setting log level to '%s' from config file", cfg.Logging.LogLevel)
		} else {
			// Check if LOG_LEVEL env var is set
			envLevel := os.Getenv("LOG_LEVEL")
			if envLevel != "" {
				logger.Infof("Using log level '%s' from LOG_LEVEL environment variable", envLevel)
			} else {
				logger.Info("Using default log level 'INFO' (no config, flag, or env var specified)")
			}
		}
	}
}
