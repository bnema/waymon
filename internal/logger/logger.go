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
	Logger        *log.Logger
	currentWriter io.Writer                   = os.Stderr
	uiNotifier    func(level, message string) // Callback to notify UI of new log entries
	logForwarder  func(level, message string) // Callback to forward logs to server
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

// SetLogForwarder sets a callback function to forward logs to server
func SetLogForwarder(forwarder func(level, message string)) {
	logForwarder = forwarder
}

// notifyUI calls the UI notifier if set
func notifyUI(level, message string) {
	if uiNotifier != nil {
		uiNotifier(level, message)
	}
}

// forwardLog calls the log forwarder if set
func forwardLog(level, message string) {
	if logForwarder != nil {
		logForwarder(level, message)
	}
}

// Convenience functions for common operations
func Info(msg interface{}, keyvals ...interface{}) {
	Logger.Info(msg, keyvals...)
	msgStr := fmt.Sprintf("%v", msg)
	notifyUI("INFO", msgStr)
	forwardLog("INFO", msgStr)
}

func Debug(msg interface{}, keyvals ...interface{}) {
	Logger.Debug(msg, keyvals...)
	if Logger.GetLevel() <= log.DebugLevel {
		msgStr := fmt.Sprintf("%v", msg)
		notifyUI("DEBUG", msgStr)
		forwardLog("DEBUG", msgStr)
	}
}

func Warn(msg interface{}, keyvals ...interface{}) {
	Logger.Warn(msg, keyvals...)
	msgStr := fmt.Sprintf("%v", msg)
	notifyUI("WARN", msgStr)
	forwardLog("WARN", msgStr)
}

func Error(msg interface{}, keyvals ...interface{}) {
	Logger.Error(msg, keyvals...)
	msgStr := fmt.Sprintf("%v", msg)
	notifyUI("ERROR", msgStr)
	forwardLog("ERROR", msgStr)
}

func Fatal(msg interface{}, keyvals ...interface{}) {
	Logger.Fatal(msg, keyvals...)
	msgStr := fmt.Sprintf("%v", msg)
	notifyUI("FATAL", msgStr)
	forwardLog("FATAL", msgStr)
}

func Infof(format string, args ...interface{}) {
	Logger.Infof(format, args...)
	msgStr := fmt.Sprintf(format, args...)
	notifyUI("INFO", msgStr)
	forwardLog("INFO", msgStr)
}

func Debugf(format string, args ...interface{}) {
	Logger.Debugf(format, args...)
	if Logger.GetLevel() <= log.DebugLevel {
		msgStr := fmt.Sprintf(format, args...)
		notifyUI("DEBUG", msgStr)
		forwardLog("DEBUG", msgStr)
	}
}

func Warnf(format string, args ...interface{}) {
	Logger.Warnf(format, args...)
	msgStr := fmt.Sprintf(format, args...)
	notifyUI("WARN", msgStr)
	forwardLog("WARN", msgStr)
}

func Errorf(format string, args ...interface{}) {
	Logger.Errorf(format, args...)
	msgStr := fmt.Sprintf(format, args...)
	notifyUI("ERROR", msgStr)
	forwardLog("ERROR", msgStr)
}

func Fatalf(format string, args ...interface{}) {
	Logger.Fatalf(format, args...)
	msgStr := fmt.Sprintf(format, args...)
	notifyUI("FATAL", msgStr)
	forwardLog("FATAL", msgStr)
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
		TimeFormat:      "15:04:05",
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
		TimeFormat:      "15:04:05",
		Prefix:          prefix,
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
	var logDir, logPath string

	// If running as root (sudo), use system log directory
	if os.Geteuid() == 0 && prefix == "SERVER" {
		logDir = "/var/log/waymon"
		logPath = filepath.Join(logDir, "waymon.log")

		// Create /var/log/waymon directory if it doesn't exist
		if err := os.MkdirAll(logDir, 0750); err != nil {
			return nil, fmt.Errorf("failed to create system log directory: %v", err)
		}
	} else {
		// Use user-specific log file for non-root or client mode
		homeDir, err := os.UserHomeDir()
		if err != nil {
			// Fallback to current directory if home dir not available
			homeDir = "."
		}

		// Create logs directory in user's home
		logDir = filepath.Join(homeDir, ".local", "share", "waymon")
		if err := os.MkdirAll(logDir, 0750); err != nil {
			// Fallback to simpler path
			logDir = filepath.Join(homeDir, ".waymon")
			if err := os.MkdirAll(logDir, 0750); err != nil {
				return nil, fmt.Errorf("failed to create log directory: %v", err)
			}
		}

		logPath = filepath.Join(logDir, "waymon.log")
	}

	// Open or create the log file with secure permissions
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600) //nolint:gosec // logPath is validated
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %v", logPath, err)
	}

	// Log where we're writing
	if _, err := fmt.Fprintf(logFile, "\n%s %s: === New session started === (log: %s)\n",
		time.Now().Format("15:04:05"), prefix, logPath); err != nil {
		// Cannot use logger here as we're in the logger package - use stderr
		fmt.Fprintf(os.Stderr, "Warning: Failed to write to log file: %v\n", err)
	}

	// Create file logger for charmbracelet/log
	fileLogger := log.NewWithOptions(logFile, log.Options{
		ReportTimestamp: true,
		TimeFormat:      "15:04:05",
		Prefix:          prefix,
	})

	// Set the default logger to write to file
	log.SetDefault(fileLogger)

	// Configure the internal logger to use the file with prefix
	// IMPORTANT: Preserve any existing UI notifier AND log level
	savedNotifier := uiNotifier
	savedLogLevel := Logger.GetLevel()

	currentWriter = logFile
	Logger = log.NewWithOptions(logFile, log.Options{
		ReportTimestamp: true,
		TimeFormat:      "15:04:05",
		Prefix:          prefix,
	})

	// Restore the UI notifier
	uiNotifier = savedNotifier

	// Preserve current log level if already set, otherwise use env var or default
	currentLevel := savedLogLevel.String()
	envLevel := strings.ToUpper(os.Getenv("LOG_LEVEL"))

	if envLevel != "" {
		// Environment variable takes precedence if set
		currentLevel = envLevel
		Logger.Infof("Setting log level to: %s (from LOG_LEVEL env var)", currentLevel)
	} else {
		// Keep existing level (may have been set from config)
		Logger.Infof("Keeping current log level: %s", currentLevel)
	}

	SetLevel(currentLevel)

	// Test that file logging is working
	Info(prefix + ": File logging initialized")

	return logFile, nil
}

// Get returns the logger instance
func Get() *log.Logger {
	return Logger
}
