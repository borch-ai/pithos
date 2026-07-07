package logger

import (
	"os"

	"github.com/charmbracelet/log"
)

var globalLogger = log.NewWithOptions(os.Stderr, log.Options{
	ReportTimestamp: false,
})

// Init initializes the global logger with the specified debug mode.
func Init(debug bool) {
	level := log.InfoLevel
	if debug {
		level = log.DebugLevel
	}
	globalLogger.SetLevel(level)
}

// Debug logs a debug message with key-value pairs.
func Debug(msg interface{}, keyvals ...interface{}) {
	globalLogger.Debug(msg, keyvals...)
}

// Info logs an informational message with key-value pairs.
func Info(msg interface{}, keyvals ...interface{}) {
	globalLogger.Info(msg, keyvals...)
}

// Warn logs a warning message with key-value pairs.
func Warn(msg interface{}, keyvals ...interface{}) {
	globalLogger.Warn(msg, keyvals...)
}

// Error logs an error message with key-value pairs.
func Error(msg interface{}, keyvals ...interface{}) {
	globalLogger.Error(msg, keyvals...)
}

// GetLogger returns the underlying charmbracelet logger instance.
func GetLogger() *log.Logger {
	return globalLogger
}
