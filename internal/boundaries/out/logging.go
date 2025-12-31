package out

import (
	"context"
	"io"
)

// LoggingPort defines the interface for session-based file logging.
// Implementations handle creating log files with session timestamps
// and managing log file lifecycle.
type LoggingPort interface {
	// NewSession creates a new logging session with a timestamped log file.
	// The component parameter identifies the source (e.g., "server", "client").
	// Returns a writer that can be used with zerolog and a cleanup function.
	NewSession(ctx context.Context, component string) (io.Writer, func(), error)

	// GetLogDir returns the directory where log files are stored.
	GetLogDir() string

	// ListSessions returns a list of available log session files.
	ListSessions(ctx context.Context) ([]LogSession, error)
}

// LogSession represents metadata about a log session file.
type LogSession struct {
	// Path is the full path to the log file.
	Path string
	// Component identifies the source (server/client).
	Component string
	// Timestamp is when the session was created.
	Timestamp string
	// Size is the file size in bytes.
	Size int64
}
