// Package app provides the application wiring layer that connects all components.
// This is the composition root where dependencies are created and injected.
package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/bnema/waymon/internal/adapters/in/ipc"
	"github.com/bnema/waymon/internal/adapters/in/tui"
	"github.com/bnema/waymon/internal/adapters/out/config"
	"github.com/bnema/waymon/internal/adapters/out/evdev"
	"github.com/bnema/waymon/internal/adapters/out/keyboard"
	"github.com/bnema/waymon/internal/adapters/out/logging"
	"github.com/bnema/waymon/internal/adapters/out/ssh"
	"github.com/bnema/waymon/internal/domain"
	serveruc "github.com/bnema/waymon/internal/usecase/server"
)

// ServerOptions configures the server application.
type ServerOptions struct {
	// Port overrides the config port (0 = use config).
	Port int
	// BindAddress overrides the config bind address.
	BindAddress string
	// NoTUI disables the terminal user interface.
	NoTUI bool
	// DebugTUI uses minimal debug output instead of full TUI.
	DebugTUI bool
	// Daemon mode runs as a background service (no TUI, for systemd).
	Daemon bool
	// ConfigPath overrides the default config file path.
	ConfigPath string
	// LogLevel overrides the config log level.
	LogLevel string
}

// RunServer starts the server application with all dependencies wired together.
func RunServer(ctx context.Context, opts ServerOptions) error {
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

	// Apply option overrides
	if opts.Port > 0 {
		cfg.Server.Port = opts.Port
	}
	if opts.BindAddress != "" {
		cfg.Server.BindAddress = opts.BindAddress
	}

	// Determine if TUI will be used (affects console logging)
	useTUI := !opts.NoTUI && !opts.Daemon

	// Create deferred TUI log writer (will be connected when TUI starts)
	var tuiLogWriter *tui.TUILogWriter
	if useTUI {
		tuiLogWriter = tui.NewDeferredTUILogWriter()
	}

	// Set up logging
	ctx, logCleanup, err := setupServerLogging(ctx, cfg, opts.LogLevel, useTUI, tuiLogWriter)
	if err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}
	defer logCleanup()

	log := zerolog.Ctx(ctx)
	log.Info().
		Int("port", cfg.Server.Port).
		Str("bind", cfg.Server.BindAddress).
		Bool("tui", !opts.NoTUI && !opts.Daemon).
		Msg("starting waymon server")

	// Create evdev capture adapter
	inputCapture := evdev.New()

	// Create SSH server adapter
	sshConfig := ssh.ServerConfig{
		Port:         cfg.Server.Port,
		BindAddress:  cfg.Server.BindAddress,
		HostKeyPath:  cfg.Server.SSHHostKeyPath,
		AuthKeysPath: cfg.Server.SSHAuthKeysPath,
		MaxClients:   cfg.Server.MaxClients,
	}
	networkServer := ssh.NewServerAdapter(sshConfig)

	// Create keyboard layout adapter for cross-layout translation
	keyboardLayoutAdapter := keyboard.NewAdapter()

	// Create server use case
	serverUseCase := serveruc.NewServerUseCase(inputCapture, networkServer, configRepo, keyboardLayoutAdapter)

	// Start the server use case (initializes input capture)
	if err := serverUseCase.Start(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	defer func() {
		log.Debug().Msg("stopping server use case")
		if err := serverUseCase.Stop(ctx); err != nil {
			log.Error().Err(err).Msg("error stopping server")
		}
	}()

	// Start networking (SSH server)
	if err := serverUseCase.StartNetworking(ctx); err != nil {
		return fmt.Errorf("failed to start networking: %w", err)
	}

	// Create IPC handler adapter (bridges use case to IPC interface)
	ipcHandler := NewIPCHandlerAdapter(serverUseCase)

	// Create IPC server for control commands (status, switch, stop, etc.)
	ipcServer, err := ipc.NewServer(ipcHandler)
	if err != nil {
		return fmt.Errorf("failed to create IPC server: %w", err)
	}

	// Set up stop handler
	stopChan := make(chan struct{})
	ipcServer.OnStop = func() {
		log.Info().Msg("stop command received via IPC")
		close(stopChan)
	}

	// Start IPC server
	if err := ipcServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start IPC server: %w", err)
	}
	defer func() {
		log.Debug().Msg("stopping IPC server")
		ipcServer.Stop(ctx)
	}()

	log.Info().Str("socket", ipcServer.SocketPath()).Msg("IPC server started")

	// Run TUI or wait for signals
	if opts.Daemon || opts.NoTUI {
		// Daemon mode: wait for stop signal or context cancellation
		log.Info().Msg("running in daemon mode, waiting for stop signal")
		select {
		case <-ctx.Done():
			log.Info().Msg("received shutdown signal")
		case <-stopChan:
			log.Info().Msg("stopping due to IPC stop command")
		}
	} else {
		// TUI mode: run the terminal interface
		tuiOpts := tui.DefaultOptions()
		if opts.DebugTUI {
			tuiOpts.AltScreen = false
		}

		// Create a TUI context that can be cancelled
		tuiCtx, tuiCancel := context.WithCancel(ctx)
		defer tuiCancel()

		// Create the TUI program and connect the deferred log writer
		tuiProgram := tui.ServerProgram(tuiCtx, serverUseCase, tuiOpts)
		if tuiLogWriter != nil {
			tuiLogWriter.SetProgram(tuiProgram)
		}

		// Handle stop command in background
		go func() {
			select {
			case <-stopChan:
				log.Info().Msg("stopping TUI due to IPC stop command")
				tuiCancel()
				tuiProgram.Quit()
			case <-tuiCtx.Done():
			}
		}()

		if _, err := tuiProgram.Run(); err != nil {
			// Check if it was a normal exit due to context cancellation
			if tuiCtx.Err() != nil {
				log.Debug().Msg("TUI exited due to context cancellation")
			} else {
				return fmt.Errorf("TUI error: %w", err)
			}
		}
	}

	// Notify connected clients of shutdown
	log.Debug().Msg("notifying clients of shutdown")
	serverUseCase.NotifyShutdown(ctx)

	log.Info().Msg("server shutdown complete")
	return nil
}

// setupServerLogging configures zerolog with file and console output.
// When useTUI is true, console output is disabled to prevent logs from interfering with the TUI.
// If tuiWriter is provided, logs will also be sent to the TUI.
// Returns a cleanup function to close log files.
func setupServerLogging(ctx context.Context, cfg *domain.Config, levelOverride string, useTUI bool, tuiWriter io.Writer) (context.Context, func(), error) {
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
	// In TUI mode, console output interferes with the alternate screen buffer
	if !useTUI {
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "15:04:05",
		}
		writers = append(writers, consoleWriter)
	}

	// Always set up file logging for session-based logs
	fileLogger := logging.New(cfg.Logging.LogDir)
	fileWriter, fileCleanup, err := fileLogger.NewSession(ctx, "server")
	if err != nil {
		// Log warning but continue without file logging
		fmt.Fprintf(os.Stderr, "Warning: could not create log file: %v\n", err)
	} else {
		writers = append(writers, fileWriter)
		// Only print log path if not in TUI mode
		if !useTUI {
			fmt.Fprintf(os.Stderr, "Logging to: %s\n", fileLogger.GetLogDir())
		}
	}

	// Add TUI writer if provided
	if tuiWriter != nil {
		writers = append(writers, tuiWriter)
	}

	// If no writers (TUI mode with failed file logging and no TUI writer), use discard
	if len(writers) == 0 {
		writers = append(writers, io.Discard)
	}

	// Create multi-writer
	multi := zerolog.MultiLevelWriter(writers...)
	logger := zerolog.New(multi).
		Level(level).
		With().
		Timestamp().
		Str("component", "server").
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
