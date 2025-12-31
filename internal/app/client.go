package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/bnema/waymon/internal/adapters/in/tui"
	"github.com/bnema/waymon/internal/adapters/out/config"
	"github.com/bnema/waymon/internal/adapters/out/display"
	"github.com/bnema/waymon/internal/adapters/out/input"
	"github.com/bnema/waymon/internal/adapters/out/keyboard"
	"github.com/bnema/waymon/internal/adapters/out/logging"
	"github.com/bnema/waymon/internal/adapters/out/ssh"
	"github.com/bnema/waymon/internal/domain"
	clientuc "github.com/bnema/waymon/internal/usecase/client"
)

// ClientOptions configures the client application.
type ClientOptions struct {
	// ServerAddress overrides the config server address (host:port).
	ServerAddress string
	// HostName specifies a named host from the config to connect to.
	HostName string
	// ConfigPath overrides the default config file path.
	ConfigPath string
	// LogLevel overrides the config log level.
	LogLevel string
	// NoTUI disables the terminal user interface.
	NoTUI bool
}

// RunClient starts the client application with all dependencies wired together.
func RunClient(ctx context.Context, opts ClientOptions) error {
	// Set up signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create config repository
	configRepo := config.NewViperRepository()
	if opts.ConfigPath != "" {
		configRepo.SetConfigPath(opts.ConfigPath)
	}

	// Auto-initialize config if it doesn't exist
	configCreated := !configRepo.Exists()
	if err := initializeConfig(ctx, configRepo); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create config file: %v\n", err)
	}
	if configCreated && configRepo.Exists() {
		fmt.Fprintf(os.Stderr, "Created default config at: %s\n", configRepo.GetConfigPath())
	}

	// Load configuration
	cfg, err := configRepo.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Determine server address
	serverAddress := resolveServerAddress(cfg, opts)
	if serverAddress == "" {
		return fmt.Errorf("no server address specified (use --host flag or set in config)")
	}

	// Update config with resolved address for use case
	cfg.Client.ServerAddress = serverAddress

	// Determine if TUI will be used
	useTUI := !opts.NoTUI

	// Set up logging
	ctx, logCleanup, err := setupClientLogging(ctx, cfg, opts.LogLevel, useTUI)
	if err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}
	defer logCleanup()

	log := zerolog.Ctx(ctx)
	log.Info().
		Str("server", serverAddress).
		Bool("tui", !opts.NoTUI).
		Msg("starting waymon client")

	// Create display adapter for monitor detection
	displayAdapter, err := display.New(ctx)
	if err != nil {
		return fmt.Errorf("failed to create display adapter: %w", err)
	}
	defer func() {
		if err := displayAdapter.Close(); err != nil {
			log.Error().Err(err).Msg("error closing display adapter")
		}
	}()

	// Create input injection adapter
	inputInjection, err := input.New(ctx)
	if err != nil {
		return fmt.Errorf("failed to create input injection adapter: %w", err)
	}

	// Create SSH client adapter
	networkClient := ssh.NewClientAdapter()

	// Create keyboard layout adapter
	keyboardAdapter := keyboard.NewAdapter()

	// Set keyboard layout port on input injector for character translation
	inputInjection.SetKeyboardLayoutPort(keyboardAdapter)

	// Create client use case
	clientUseCase := clientuc.NewClientUseCase(inputInjection, networkClient, displayAdapter, configRepo, keyboardAdapter)

	// Set server address from CLI/config resolution
	clientUseCase.SetServerAddress(serverAddress)

	// Connect to server
	if err := clientUseCase.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		log.Debug().Msg("disconnecting from server")
		if err := clientUseCase.Disconnect(ctx); err != nil {
			log.Error().Err(err).Msg("error disconnecting")
		}
	}()

	// Run TUI or wait for signals
	if opts.NoTUI {
		// No TUI mode: wait for context cancellation
		log.Info().Msg("running without TUI, waiting for shutdown signal")
		<-ctx.Done()
		log.Info().Msg("received shutdown signal")
	} else {
		// TUI mode: run the terminal interface
		tuiOpts := tui.DefaultOptions()

		if err := tui.RunClient(ctx, clientUseCase, tuiOpts); err != nil {
			// Check if it was a normal exit due to context cancellation
			if ctx.Err() != nil {
				log.Debug().Msg("TUI exited due to context cancellation")
			} else {
				return fmt.Errorf("TUI error: %w", err)
			}
		}
	}

	log.Info().Msg("client shutdown complete")
	return nil
}

// resolveServerAddress determines the server address from options and config.
func resolveServerAddress(cfg *domain.Config, opts ClientOptions) string {
	var addr string

	// Priority order: CLI flag > named host > config default
	switch {
	case opts.ServerAddress != "":
		addr = opts.ServerAddress
	case opts.HostName != "":
		// Named host from config
		for _, host := range cfg.Hosts {
			if host.Name == opts.HostName {
				addr = host.Address
				break
			}
		}
		// Host name specified but not found - addr remains empty
	default:
		// Default server address from config
		addr = cfg.Client.ServerAddress
	}

	// Normalize: add default port if not specified
	return normalizeServerAddress(addr)
}

// setupClientLogging configures zerolog with file and console output.
// When useTUI is true, console output is disabled to prevent logs from interfering with the TUI.
// Returns a cleanup function to close log files.
func setupClientLogging(ctx context.Context, cfg *domain.Config, levelOverride string, useTUI bool) (context.Context, func(), error) {
	// Determine log level
	level := zerolog.InfoLevel
	levelStr := cfg.Logging.LogLevel
	if levelOverride != "" {
		levelStr = levelOverride
	}
	if levelStr != "" {
		var err error
		level, err = zerolog.ParseLevel(levelStr)
		if err != nil {
			level = zerolog.InfoLevel
		}
	}

	var writers []io.Writer

	// Only add console writer if NOT in TUI mode
	if !useTUI {
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "15:04:05",
		}
		writers = append(writers, consoleWriter)
	}

	// Always set up file logging for session-based logs
	fileLogger := logging.New(cfg.Logging.LogDir)
	fileWriter, fileCleanup, err := fileLogger.NewSession(ctx, "client")
	if err != nil {
		// Log warning but continue without file logging
		fmt.Fprintf(os.Stderr, "Warning: could not create log file: %v\n", err)
	} else {
		writers = append(writers, fileWriter)
		if !useTUI {
			fmt.Fprintf(os.Stderr, "Logging to: %s\n", fileLogger.GetLogDir())
		}
	}

	// If no writers, use discard
	if len(writers) == 0 {
		writers = append(writers, io.Discard)
	}

	// Create multi-writer
	multi := zerolog.MultiLevelWriter(writers...)
	logger := zerolog.New(multi).
		Level(level).
		With().
		Timestamp().
		Str("component", "client").
		Logger()

	// Store logger in context
	ctx = logger.WithContext(ctx)

	cleanup := func() {
		if fileCleanup != nil {
			fileCleanup()
		}
	}

	return ctx, cleanup, nil
}
