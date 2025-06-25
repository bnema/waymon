package ui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ShutdownConfig holds configuration for graceful shutdown
type ShutdownConfig struct {
	GracePeriod time.Duration // Time to wait for graceful shutdown
	ForcePeriod time.Duration // Time to wait before SIGKILL
}

// DefaultShutdownConfig returns sensible defaults
func DefaultShutdownConfig() ShutdownConfig {
	return ShutdownConfig{
		GracePeriod: 5 * time.Second,
		ForcePeriod: 2 * time.Second,
	}
}

// BaseUI provides common functionality for all UI models
type BaseUI struct {
	// Context and shutdown
	ctx            context.Context
	cancel         context.CancelFunc
	shutdownConfig ShutdownConfig
	shutdownOnce   sync.Once
	isShuttingDown bool
	shutdownMu     sync.RWMutex

	// Window dimensions
	windowWidth  int
	windowHeight int

	// Logging
	logBuffer   []LogEntry
	maxLogLines int
	logMu       sync.RWMutex

	// Common UI elements
	spinner      spinner.Model
	lastUpdate   time.Time
	messageTimer *time.Timer

	// Callbacks for lifecycle events
	onShutdown func() error // Called during shutdown
}

// NewBaseUI creates a new base UI with context and shutdown handling
func NewBaseUI(ctx context.Context, cfg ShutdownConfig) *BaseUI {
	ctx, cancel := context.WithCancel(ctx)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	base := &BaseUI{
		ctx:            ctx,
		cancel:         cancel,
		shutdownConfig: cfg,
		maxLogLines:    100,
		logBuffer:      make([]LogEntry, 0),
		spinner:        s,
		lastUpdate:     time.Now(),
	}

	// Setup signal handling
	go base.handleSignals()

	return base
}

// handleSignals manages OS signals for graceful shutdown
func (b *BaseUI) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
		b.InitiateShutdown()
	case <-b.ctx.Done():
		// Context cancelled, nothing to do
	}
}

// InitiateShutdown starts the shutdown process
func (b *BaseUI) InitiateShutdown() tea.Cmd {
	var cmd tea.Cmd

	b.shutdownOnce.Do(func() {
		b.shutdownMu.Lock()
		b.isShuttingDown = true
		b.shutdownMu.Unlock()

		b.AddLogEntry("info", "Shutting down...")

		// Cancel main context to signal shutdown
		b.cancel()

		// Return quit command to exit Bubble Tea
		cmd = tea.Quit
	})

	return cmd
}

// IsShuttingDown returns true if shutdown has been initiated
func (b *BaseUI) IsShuttingDown() bool {
	b.shutdownMu.RLock()
	defer b.shutdownMu.RUnlock()
	return b.isShuttingDown
}

// Context returns the UI's context
func (b *BaseUI) Context() context.Context {
	return b.ctx
}

// SetOnShutdown sets the shutdown callback
func (b *BaseUI) SetOnShutdown(fn func() error) {
	b.onShutdown = fn
}

// AddLogEntry adds a log entry with thread-safe access
func (b *BaseUI) AddLogEntry(level, message string) {
	b.logMu.Lock()
	defer b.logMu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	}

	b.logBuffer = append(b.logBuffer, entry)

	// Trim buffer if needed
	if len(b.logBuffer) > b.maxLogLines {
		b.logBuffer = b.logBuffer[len(b.logBuffer)-b.maxLogLines:]
	}
}

// GetLogs returns a copy of the log buffer
func (b *BaseUI) GetLogs() []LogEntry {
	b.logMu.RLock()
	defer b.logMu.RUnlock()

	logs := make([]LogEntry, len(b.logBuffer))
	copy(logs, b.logBuffer)
	return logs
}

// UpdateWindowSize updates the window dimensions
func (b *BaseUI) UpdateWindowSize(width, height int) {
	b.windowWidth = width
	b.windowHeight = height
}

// GetWindowSize returns the current window dimensions
func (b *BaseUI) GetWindowSize() (width, height int) {
	return b.windowWidth, b.windowHeight
}

// TickSpinner returns a command to update the spinner
func (b *BaseUI) TickSpinner() tea.Cmd {
	return b.spinner.Tick
}

// UpdateSpinner processes spinner messages
func (b *BaseUI) UpdateSpinner(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	b.spinner, cmd = b.spinner.Update(msg)
	return cmd
}

// GetSpinner returns the current spinner view
func (b *BaseUI) GetSpinner() string {
	return b.spinner.View()
}

// BaseUpdate handles common update logic
func (b *BaseUI) BaseUpdate(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.UpdateWindowSize(msg.Width, msg.Height)
		return nil

	case spinner.TickMsg:
		return b.UpdateSpinner(msg)

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return b.InitiateShutdown()
		}
	}

	return nil
}

// FormatLogEntry formats a log entry for display
func (b *BaseUI) FormatLogEntry(entry LogEntry) string {
	levelStyle := lipgloss.NewStyle().Bold(true)
	levelText := ""

	switch entry.Level {
	case "ERROR", "error":
		levelStyle = levelStyle.Foreground(lipgloss.Color("196")) // Bright red
		levelText = "ERROR"
	case "WARN", "warn", "warning":
		levelStyle = levelStyle.Foreground(lipgloss.Color("214")) // Orange
		levelText = "WARN "
	case "INFO", "info":
		levelStyle = levelStyle.Foreground(lipgloss.Color("42")) // Bright green
		levelText = "INFO "
	case "DEBUG", "debug":
		levelStyle = levelStyle.Foreground(lipgloss.Color("245")) // Gray (more visible than 241)
		levelText = "DEBUG"
	default:
		levelStyle = levelStyle.Foreground(lipgloss.Color("255")) // White
		levelText = fmt.Sprintf("%-5s", entry.Level)
	}

	// Format timestamp with subtle color
	timestampStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	timestamp := timestampStyle.Render(entry.Timestamp.Format("15:04:05"))

	// Format the level with color and fixed width
	level := levelStyle.Render(levelText)

	// Message in default color
	return fmt.Sprintf("%s %s %s", timestamp, level, entry.Message)
}

// Common messages that can be embedded in specific UI models
type BaseMessages struct{}

// ShutdownStartedMsg indicates shutdown has begun
type ShutdownStartedMsg struct{}

// ShutdownCompleteMsg indicates shutdown is complete
type ShutdownCompleteMsg struct{}

// ForceShutdownMsg indicates forced shutdown is needed
type ForceShutdownMsg struct{}
