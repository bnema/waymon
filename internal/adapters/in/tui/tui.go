// Package tui provides the terminal user interface for waymon.
// It implements driving adapters that call into the use case layer
// through boundary interfaces.
package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bnema/waymon/internal/adapters/in/tui/views/client"
	"github.com/bnema/waymon/internal/adapters/in/tui/views/monitors"
	"github.com/bnema/waymon/internal/adapters/in/tui/views/server"
	"github.com/bnema/waymon/internal/adapters/in/tui/views/status"
	"github.com/bnema/waymon/internal/boundaries/in"
	"github.com/bnema/waymon/internal/domain"
)

// Options configures TUI behavior.
type Options struct {
	// AltScreen uses the alternate screen buffer (recommended for full TUI).
	AltScreen bool
	// DisableMouse disables mouse input.
	DisableMouse bool
}

// DefaultOptions returns sensible default options.
func DefaultOptions() Options {
	return Options{
		AltScreen:    true,
		DisableMouse: false,
	}
}

// RunServer starts the server TUI.
// It creates a tea.Program with the server view model and runs until completion.
func RunServer(ctx context.Context, useCase in.ServerUseCase, opts Options) error {
	model := server.New(ctx, useCase)

	programOpts := []tea.ProgramOption{}
	if opts.AltScreen {
		programOpts = append(programOpts, tea.WithAltScreen())
	}
	if !opts.DisableMouse {
		programOpts = append(programOpts, tea.WithMouseCellMotion())
	}

	p := tea.NewProgram(model, programOpts...)

	// Run the program
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("server TUI error: %w", err)
	}

	return nil
}

// RunClient starts the client TUI.
// It creates a tea.Program with the client view model and runs until completion.
func RunClient(ctx context.Context, useCase in.ClientUseCase, opts Options) error {
	model := client.New(ctx, useCase)

	programOpts := []tea.ProgramOption{}
	if opts.AltScreen {
		programOpts = append(programOpts, tea.WithAltScreen())
	}
	if !opts.DisableMouse {
		programOpts = append(programOpts, tea.WithMouseCellMotion())
	}

	p := tea.NewProgram(model, programOpts...)

	// Run the program
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("client TUI error: %w", err)
	}

	return nil
}

// RunStatus displays the status view and exits.
// This is a one-shot TUI that shows current status and waits for a key press.
func RunStatus(response *in.StatusResponse, opts Options) error {
	model := status.NewFromIPC(response)

	programOpts := []tea.ProgramOption{}
	if opts.AltScreen {
		programOpts = append(programOpts, tea.WithAltScreen())
	}

	p := tea.NewProgram(model, programOpts...)

	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("status TUI error: %w", err)
	}

	return nil
}

// PrintStatus prints status to stdout without TUI (for non-interactive use).
func PrintStatus(response *in.StatusResponse) string {
	model := status.NewFromIPC(response)
	return model.RenderStatic()
}

// RunMonitors displays the monitors view and exits.
// This shows detected monitors with a visual layout representation.
func RunMonitors(detectedMonitors []domain.Monitor, opts Options) error {
	model := monitors.New().SetMonitors(detectedMonitors)

	programOpts := []tea.ProgramOption{}
	if opts.AltScreen {
		programOpts = append(programOpts, tea.WithAltScreen())
	}

	p := tea.NewProgram(model, programOpts...)

	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("monitors TUI error: %w", err)
	}

	return nil
}

// PrintMonitors prints monitors to stdout without TUI (for non-interactive use).
func PrintMonitors(detectedMonitors []domain.Monitor) string {
	model := monitors.New().SetMonitors(detectedMonitors)
	return model.RenderStatic()
}

// ServerProgram creates a tea.Program for the server TUI without running it.
// This allows the caller to control when to start/stop the program.
func ServerProgram(ctx context.Context, useCase in.ServerUseCase, opts Options) *tea.Program {
	model := server.New(ctx, useCase)

	programOpts := []tea.ProgramOption{}
	if opts.AltScreen {
		programOpts = append(programOpts, tea.WithAltScreen())
	}
	if !opts.DisableMouse {
		programOpts = append(programOpts, tea.WithMouseCellMotion())
	}

	return tea.NewProgram(model, programOpts...)
}

// ClientProgram creates a tea.Program for the client TUI without running it.
// This allows the caller to control when to start/stop the program.
func ClientProgram(ctx context.Context, useCase in.ClientUseCase, opts Options) *tea.Program {
	model := client.New(ctx, useCase)

	programOpts := []tea.ProgramOption{}
	if opts.AltScreen {
		programOpts = append(programOpts, tea.WithAltScreen())
	}
	if !opts.DisableMouse {
		programOpts = append(programOpts, tea.WithMouseCellMotion())
	}

	return tea.NewProgram(model, programOpts...)
}
