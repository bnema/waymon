// Package tui provides the terminal user interface for waymon.
package tui

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bnema/waymon/internal/adapters/in/tui/messages"
)

// TUILogWriter implements io.Writer and sends log entries to a Bubble Tea program.
// It supports late-binding: the program can be set after creation.
type TUILogWriter struct {
	program *tea.Program
	mu      sync.Mutex
	buf     bytes.Buffer
}

// NewTUILogWriter creates a new TUILogWriter that sends logs to the given program.
func NewTUILogWriter(program *tea.Program) *TUILogWriter {
	return &TUILogWriter{
		program: program,
	}
}

// NewDeferredTUILogWriter creates a TUILogWriter without a program.
// Call SetProgram() to connect it later. Writes before SetProgram are discarded.
func NewDeferredTUILogWriter() *TUILogWriter {
	return &TUILogWriter{}
}

// SetProgram sets the tea.Program to send logs to.
func (w *TUILogWriter) SetProgram(program *tea.Program) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.program = program
}

// Write implements io.Writer. It parses zerolog JSON output and sends LogMsg to the TUI.
func (w *TUILogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// If no program connected yet, discard
	if w.program == nil {
		return len(p), nil
	}

	// Write to buffer
	w.buf.Write(p)

	// Try to parse complete JSON lines
	for {
		line, err := w.buf.ReadBytes('\n')
		if err == io.EOF {
			// Put back incomplete line
			w.buf.Write(line)
			break
		}
		if err != nil {
			break
		}

		// Parse JSON log entry
		var entry map[string]interface{}
		if err := json.Unmarshal(line, &entry); err != nil {
			// Not JSON, just send as raw message
			w.program.Send(messages.LogMsg{
				Level:   "INF",
				Message: string(bytes.TrimSpace(line)),
			})
			continue
		}

		// Extract fields
		level := "INF"
		if l, ok := entry["level"].(string); ok {
			level = l
		}

		message := ""
		if m, ok := entry["message"].(string); ok {
			message = m
		}

		// Collect other fields
		fields := make(map[string]string)
		for k, v := range entry {
			if k == "level" || k == "message" || k == "time" || k == "timestamp" {
				continue
			}
			switch val := v.(type) {
			case string:
				fields[k] = val
			case float64:
				fields[k] = formatFloat(val)
			case bool:
				if val {
					fields[k] = "true"
				} else {
					fields[k] = "false"
				}
			}
		}

		// Send to TUI
		w.program.Send(messages.LogMsg{
			Level:   level,
			Message: message,
			Fields:  fields,
		})
	}

	return len(p), nil
}

// formatFloat formats a float for display.
func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return formatInt64(int64(f))
	}
	// Simple float formatting
	return formatInt64(int64(f*100)/100) + "." + formatInt64(int64(f*100)%100)
}

// formatInt64 formats an int64 as string.
func formatInt64(n int64) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + formatInt64(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
