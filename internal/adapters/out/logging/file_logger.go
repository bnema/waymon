// Package logging provides file-based logging adapters for waymon.
package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bnema/waymon/internal/boundaries/out"
)

// FileLogger implements the LoggingPort interface with file-based session logging.
type FileLogger struct {
	logDir string
}

// New creates a new FileLogger with the specified log directory.
// If logDir is empty, it uses a default location based on user permissions.
func New(logDir string) *FileLogger {
	if logDir == "" {
		logDir = defaultLogDir()
	}
	return &FileLogger{logDir: logDir}
}

// NewSession creates a new logging session with a timestamped log file.
func (f *FileLogger) NewSession(ctx context.Context, component string) (io.Writer, func(), error) {
	// Ensure log directory exists
	if err := os.MkdirAll(f.logDir, 0750); err != nil {
		return nil, nil, fmt.Errorf("failed to create log directory %s: %w", f.logDir, err)
	}

	// Create timestamped filename: waymon-{component}-{timestamp}.log
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("waymon-%s-%s.log", component, timestamp)
	logPath := filepath.Join(f.logDir, filename)

	// Open file for writing
	// Note: logPath is safely constructed from logDir + timestamp - this is intentional
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640) //nolint:gosec // G304: Log path is safely constructed
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create log file %s: %w", logPath, err)
	}

	// Write session header
	header := fmt.Sprintf("=== Waymon %s session started at %s ===\n",
		component, time.Now().Format(time.RFC3339))
	if _, err := file.WriteString(header); err != nil {
		_ = file.Close() // Best effort close on error
		return nil, nil, fmt.Errorf("failed to write log header: %w", err)
	}

	// Cleanup function closes the file and writes footer
	cleanup := func() {
		footer := fmt.Sprintf("\n=== Waymon %s session ended at %s ===\n",
			component, time.Now().Format(time.RFC3339))
		_, _ = file.WriteString(footer) // Best effort
		_ = file.Close()
	}

	return file, cleanup, nil
}

// GetLogDir returns the directory where log files are stored.
func (f *FileLogger) GetLogDir() string {
	return f.logDir
}

// ListSessions returns a list of available log session files.
func (f *FileLogger) ListSessions(ctx context.Context) ([]out.LogSession, error) {
	entries, err := os.ReadDir(f.logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []out.LogSession{}, nil
		}
		return nil, fmt.Errorf("failed to read log directory: %w", err)
	}

	var sessions []out.LogSession
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "waymon-") || !strings.HasSuffix(name, ".log") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Parse component and timestamp from filename
		// Format: waymon-{component}-{timestamp}.log
		parts := strings.TrimPrefix(name, "waymon-")
		parts = strings.TrimSuffix(parts, ".log")

		// Find last dash to split component from timestamp
		lastDash := strings.LastIndex(parts, "-")
		var component, timestamp string
		if lastDash > 0 {
			// Timestamp format: 2006-01-02_15-04-05 (contains dashes)
			// We need to find the date part
			underscoreIdx := strings.Index(parts, "_")
			if underscoreIdx > 0 {
				// Find the dash before the date (YYYY-MM-DD)
				datePart := parts[:underscoreIdx]
				firstDateDash := strings.Index(datePart, "-")
				if firstDateDash > 0 && firstDateDash < len(datePart)-5 {
					// Check if this looks like a date (has two more dashes)
					afterFirst := datePart[firstDateDash+1:]
					if strings.Contains(afterFirst, "-") {
						// This is component-YYYY-MM-DD format
						component = datePart[:firstDateDash]
						timestamp = parts[firstDateDash+1:]
					}
				}
			}
			if component == "" {
				// Fallback: just use everything before last dash as component
				component = parts[:lastDash]
				timestamp = parts[lastDash+1:]
			}
		} else {
			component = parts
			timestamp = ""
		}

		sessions = append(sessions, out.LogSession{
			Path:      filepath.Join(f.logDir, name),
			Component: component,
			Timestamp: timestamp,
			Size:      info.Size(),
		})
	}

	// Sort by timestamp (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Timestamp > sessions[j].Timestamp
	})

	return sessions, nil
}

// defaultLogDir returns the default log directory based on user permissions.
func defaultLogDir() string {
	// Root uses /var/log/waymon
	if os.Getuid() == 0 {
		return "/var/log/waymon"
	}

	// Regular users use XDG cache dir or fallback
	if cacheDir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(cacheDir, "waymon", "logs")
	}

	// Fallback to /tmp
	return filepath.Join(os.TempDir(), "waymon", "logs")
}
