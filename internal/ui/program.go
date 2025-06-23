package ui

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
)

// ProgramConfig holds configuration for running a UI program
type ProgramConfig struct {
	ShutdownConfig ShutdownConfig
	Debug          bool
	LogFile        string
}

// DefaultProgramConfig returns default configuration
func DefaultProgramConfig() ProgramConfig {
	return ProgramConfig{
		ShutdownConfig: DefaultShutdownConfig(),
		Debug:          false,
		LogFile:        "",
	}
}

// UIModel interface that all UI models must implement
type UIModel interface {
	tea.Model
	// SetBase allows the model to store reference to base UI
	SetBase(base *BaseUI)
	// OnShutdown is called during shutdown
	OnShutdown() error
}

// ProgramModel is an optional interface for models that need program reference
type ProgramModel interface {
	UIModel
	SetProgram(p *tea.Program)
}

// ProgramRunner manages the lifecycle of a Bubble Tea program with proper shutdown
type ProgramRunner struct {
	config  ProgramConfig
	base    *BaseUI
	program *tea.Program
	logger  *log.Logger
	done    chan struct{} // Signals when the program has exited
}

// NewProgramRunner creates a new program runner
func NewProgramRunner(config ProgramConfig) *ProgramRunner {
	logger := log.New(os.Stderr)
	if config.Debug {
		logger.SetLevel(log.DebugLevel)
	} else {
		logger.SetLevel(log.InfoLevel)
	}

	return &ProgramRunner{
		config: config,
		logger: logger,
		done:   make(chan struct{}),
	}
}

// Run starts the UI program with the given model
func (r *ProgramRunner) Run(ctx context.Context, model UIModel) error {
	// Ensure done channel is closed when we exit
	defer close(r.done)
	
	// Create base UI with context
	r.base = NewBaseUI(ctx, r.config.ShutdownConfig)

	// Set up shutdown callback
	r.base.SetOnShutdown(func() error {
		r.logger.Info("Starting graceful shutdown...")

		// Call model's shutdown method
		if err := model.OnShutdown(); err != nil {
			r.logger.Error("Model shutdown error", "error", err)
			return err
		}

		// Additional cleanup can go here
		r.logger.Info("Graceful shutdown complete")
		return nil
	})

	// Give model reference to base
	model.SetBase(r.base)

	// Configure program options
	opts := []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	}

	if r.config.LogFile != "" {
		f, err := os.OpenFile(r.config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		defer f.Close()
		opts = append(opts, tea.WithOutput(f))
	}

	// Create program
	r.program = tea.NewProgram(model, opts...)

	// If model implements ProgramModel, set the program reference
	if pm, ok := model.(ProgramModel); ok {
		pm.SetProgram(r.program)
	}

	// Run in a goroutine to handle context cancellation
	errCh := make(chan error, 1)
	go func() {
		_, err := r.program.Run()
		errCh <- err
	}()

	// Wait for either program completion or context cancellation
	var runErr error
	select {
	case err := <-errCh:
		// Program exited normally
		runErr = err
	case <-ctx.Done():
		// Context cancelled, tell the program to quit
		r.program.Quit()

		// Wait for program to finish with timeout
		select {
		case err := <-errCh:
			runErr = err
		case <-time.After(2 * time.Second):
			// Force kill the program if it's not responding
			r.program.Kill()
			<-errCh // Wait for it to actually exit
		}
	}

	// Now that Bubble Tea has exited, run the shutdown callback
	if r.base.onShutdown != nil {
		// Create a context for shutdown with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), r.config.ShutdownConfig.GracePeriod)
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- r.base.onShutdown()
		}()

		select {
		case err := <-done:
			if err != nil {
				r.logger.Error("Shutdown callback error", "error", err)
			}
		case <-shutdownCtx.Done():
			r.logger.Warn("Shutdown callback timed out")
			// Continue anyway
		}
	}

	return runErr
}

// Send sends a message to the running program
func (r *ProgramRunner) Send(msg tea.Msg) {
	if r.program != nil {
		r.program.Send(msg)
	}
}

// Quit sends a quit message to the program
func (r *ProgramRunner) Quit() {
	if r.program != nil {
		r.program.Quit()
	}
}

// Done returns a channel that's closed when the program exits
func (r *ProgramRunner) Done() <-chan struct{} {
	return r.done
}

// Example base model implementation that embeds BaseUI functionality
type BaseModel struct {
	base *BaseUI
}

// SetBase implements UIModel interface
func (m *BaseModel) SetBase(base *BaseUI) {
	m.base = base
}

// OnShutdown implements UIModel interface
func (m *BaseModel) OnShutdown() error {
	// Override in specific implementations
	return nil
}

// Init implements tea.Model
func (m *BaseModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model with base functionality
func (m *BaseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle base updates
	if m.base != nil {
		if cmd := m.base.BaseUpdate(msg); cmd != nil {
			return m, cmd
		}
	}
	return m, nil
}

// View implements tea.Model
func (m *BaseModel) View() string {
	return "Base view - override in implementation"
}