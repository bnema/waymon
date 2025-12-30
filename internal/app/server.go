// Package app provides the application wiring layer that connects all components.
// This is the composition root where dependencies are created and injected.
package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/bnema/waymon/internal/adapters/in/ipc"
	"github.com/bnema/waymon/internal/adapters/in/tui"
	"github.com/bnema/waymon/internal/adapters/out/config"
	"github.com/bnema/waymon/internal/adapters/out/evdev"
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

	// Load configuration
	cfg, err := configRepo.Load(ctx)
	if err != nil {
		// Use defaults if config not found
		cfg = defaultServerConfig()
	}

	// Apply option overrides
	if opts.Port > 0 {
		cfg.Server.Port = opts.Port
	}
	if opts.BindAddress != "" {
		cfg.Server.BindAddress = opts.BindAddress
	}

	// Set up logging
	ctx, logCleanup, err := setupServerLogging(ctx, cfg, opts.LogLevel)
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

	// Create server use case
	serverUseCase := serveruc.NewServerUseCase(inputCapture, networkServer, configRepo)

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

		// Handle stop command in background
		go func() {
			select {
			case <-stopChan:
				log.Info().Msg("stopping TUI due to IPC stop command")
				tuiCancel()
			case <-tuiCtx.Done():
			}
		}()

		if err := tui.RunServer(tuiCtx, serverUseCase, tuiOpts); err != nil {
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
// Returns a cleanup function to close log files.
func setupServerLogging(ctx context.Context, cfg *domain.Config, levelOverride string) (context.Context, func(), error) {
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

	// Create console writer
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}

	var writers []io.Writer
	writers = append(writers, consoleWriter)

	var fileHandle *os.File

	// Set up file logging if enabled
	if cfg.Logging.FileLogging {
		logDir := cfg.Logging.LogDir
		if logDir == "" {
			// Use /var/log/waymon for root, user cache dir otherwise
			if os.Getuid() == 0 {
				logDir = "/var/log/waymon"
			} else if cacheDir, err := os.UserCacheDir(); err == nil {
				logDir = filepath.Join(cacheDir, "waymon")
			} else {
				logDir = filepath.Join(os.TempDir(), "waymon")
			}
		}

		// Create log directory if it doesn't exist
		if err := os.MkdirAll(logDir, 0750); err != nil {
			return ctx, func() {}, fmt.Errorf("failed to create log directory: %w", err)
		}

		logPath := filepath.Join(logDir, "waymon-server.log")
		var err error
		fileHandle, err = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
		if err != nil {
			return ctx, func() {}, fmt.Errorf("failed to open log file: %w", err)
		}
		writers = append(writers, fileHandle)
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
		if fileHandle != nil {
			fileHandle.Close()
		}
	}

	return ctx, cleanup, nil
}

// defaultServerConfig returns a default server configuration.
func defaultServerConfig() *domain.Config {
	return &domain.Config{
		Server: domain.ServerCfg{
			Port:       52525,
			MaxClients: 5,
		},
		Logging: domain.LoggingConfig{
			LogLevel: "info",
		},
	}
}
