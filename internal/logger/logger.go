package logger

import (
	"os"
	"strings"

	"github.com/charmbracelet/log"
)

var Logger *log.Logger

func init() {
	Logger = log.New(os.Stderr)

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

// Convenience functions for common operations
func Info(msg interface{}, keyvals ...interface{}) {
	Logger.Info(msg, keyvals...)
}

func Debug(msg interface{}, keyvals ...interface{}) {
	Logger.Debug(msg, keyvals...)
}

func Warn(msg interface{}, keyvals ...interface{}) {
	Logger.Warn(msg, keyvals...)
}

func Error(msg interface{}, keyvals ...interface{}) {
	Logger.Error(msg, keyvals...)
}

func Fatal(msg interface{}, keyvals ...interface{}) {
	Logger.Fatal(msg, keyvals...)
}

func Infof(format string, args ...interface{}) {
	Logger.Infof(format, args...)
}

func Debugf(format string, args ...interface{}) {
	Logger.Debugf(format, args...)
}

func Warnf(format string, args ...interface{}) {
	Logger.Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	Logger.Errorf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
	Logger.Fatalf(format, args...)
}
