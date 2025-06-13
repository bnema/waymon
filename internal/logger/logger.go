package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

var (
	Logger       *log.Logger
	currentWriter io.Writer = os.Stderr
	uiNotifier   func(level, message string) // Callback to notify UI of new log entries
)

func init() {
	Logger = log.New(os.Stderr)

	// Suppress all logs when used as a display helper
	if os.Getenv("WAYMON_DISPLAY_HELPER") == "1" {
		Logger.SetLevel(log.FatalLevel + 1) // Suppress everything
		return
	}

	// Set log level from environment variable
	logLevel := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	switch logLevel {
	case "DEBUG":
		Logger.SetLevel(log.DebugLevel)
	case "INFO":
		Logger.SetLevel(log.InfoLevel)
	case "WARN", "WARNING":
		Logger.SetLevel(log.WarnLevel)
	case "ERROR":
		Logger.SetLevel(log.ErrorLevel)
	case "FATAL":
		Logger.SetLevel(log.FatalLevel)
	default:
		// Default to INFO level if not specified or invalid
		Logger.SetLevel(log.InfoLevel)
	}
}

// SetUINotifier sets a callback function to notify UI of log entries
func SetUINotifier(notifier func(level, message string)) {
	uiNotifier = notifier
}

// notifyUI calls the UI notifier if set
func notifyUI(level, message string) {
	if uiNotifier != nil {
		uiNotifier(level, message)
	}
}

// Convenience functions for common operations
func Info(msg interface{}, keyvals ...interface{}) {
	Logger.Info(msg, keyvals...)
	notifyUI("INFO", fmt.Sprintf("%v", msg))
}

func Debug(msg interface{}, keyvals ...interface{}) {
	Logger.Debug(msg, keyvals...)
	notifyUI("DEBUG", fmt.Sprintf("%v", msg))
}

func Warn(msg interface{}, keyvals ...interface{}) {
	Logger.Warn(msg, keyvals...)
	notifyUI("WARN", fmt.Sprintf("%v", msg))
}

func Error(msg interface{}, keyvals ...interface{}) {
	Logger.Error(msg, keyvals...)
	notifyUI("ERROR", fmt.Sprintf("%v", msg))
}

func Fatal(msg interface{}, keyvals ...interface{}) {
	Logger.Fatal(msg, keyvals...)
	notifyUI("FATAL", fmt.Sprintf("%v", msg))
}

func Infof(format string, args ...interface{}) {
	Logger.Infof(format, args...)
	notifyUI("INFO", fmt.Sprintf(format, args...))
}

func Debugf(format string, args ...interface{}) {
	Logger.Debugf(format, args...)
	notifyUI("DEBUG", fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...interface{}) {
	Logger.Warnf(format, args...)
	notifyUI("WARN", fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...interface{}) {
	Logger.Errorf(format, args...)
	notifyUI("ERROR", fmt.Sprintf(format, args...))
}

func Fatalf(format string, args ...interface{}) {
	Logger.Fatalf(format, args...)
	notifyUI("FATAL", fmt.Sprintf(format, args...))
}

// SetLevel sets the log level from a string
func SetLevel(level string) {
	switch strings.ToUpper(level) {
	case "DEBUG":
		Logger.SetLevel(log.DebugLevel)
	case "INFO":
		Logger.SetLevel(log.InfoLevel)
	case "WARN", "WARNING":
		Logger.SetLevel(log.WarnLevel)
	case "ERROR":
		Logger.SetLevel(log.ErrorLevel)
	case "FATAL":
		Logger.SetLevel(log.FatalLevel)
	}
}

// SetOutput redirects the logger output to a different writer
func SetOutput(w io.Writer) {
	currentWriter = w
	Logger = log.NewWithOptions(w, log.Options{
		ReportTimestamp: true,
		TimeFormat:     "15:04:05",
	})
	// Restore the current log level
	currentLevel := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	if currentLevel == "" {
		currentLevel = "INFO"
	}
	SetLevel(currentLevel)
}

// SetPrefix sets a prefix for the logger
func SetPrefix(prefix string) {
	Logger = log.NewWithOptions(currentWriter, log.Options{
		ReportTimestamp: true,
		TimeFormat:     "15:04:05",
		Prefix:         prefix,
	})
	// Restore the current log level
	currentLevel := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	if currentLevel == "" {
		currentLevel = "INFO"
	}
	SetLevel(currentLevel)
}

// SetupFileLogging configures both the default log and internal logger to write to a file
// This is used by both client and server to avoid TUI interference
func SetupFileLogging(prefix string) (*os.File, error) {
	// Always use a user-specific log file to avoid permission issues
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home dir not available
		homeDir = "."
	}
	
	// Create logs directory in user's home
	logDir := filepath.Join(homeDir, ".local", "share", "waymon")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		// Fallback to simpler path
		logDir = filepath.Join(homeDir, ".waymon")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %v", err)
		}
	}
	
	logPath := filepath.Join(logDir, "waymon.log")
	
	// Open or create the log file with user-only permissions
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %v", logPath, err)
	}
	
	// Log where we're writing
	fmt.Fprintf(logFile, "\n%s %s: === New session started === (log: %s)\n", 
		time.Now().Format("15:04:05"), prefix, logPath)
	
	// Create file logger for charmbracelet/log
	fileLogger := log.NewWithOptions(logFile, log.Options{
		ReportTimestamp: true,
		TimeFormat:     "15:04:05",
		Prefix:         prefix,
	})
	
	// Set the default logger to write to file
	log.SetDefault(fileLogger)
	
	// Configure the internal logger to use the file with prefix
	// IMPORTANT: Preserve any existing UI notifier
	savedNotifier := uiNotifier
	
	currentWriter = logFile
	Logger = log.NewWithOptions(logFile, log.Options{
		ReportTimestamp: true,
		TimeFormat:     "15:04:05",
		Prefix:         prefix,
	})
	
	// Restore the UI notifier
	uiNotifier = savedNotifier
	
	// Set log level
	currentLevel := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	if currentLevel == "" {
		currentLevel = "INFO"
	}
	// Debug: Print what level we're setting (this will go to file)
	Logger.Infof("Setting log level to: %s (from env: %s)", currentLevel, os.Getenv("LOG_LEVEL"))
	SetLevel(currentLevel)
	
	// Test that file logging is working
	Info(prefix + ": File logging initialized")
	
	return logFile, nil
}

// Get returns the logger instance
func Get() *log.Logger {
	return Logger
}
