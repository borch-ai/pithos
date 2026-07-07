package logger

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
)

func TestLoggerInitAndLevels(t *testing.T) {
	var buf bytes.Buffer

	// Backup original level
	origLevel := globalLogger.GetLevel()
	defer func() {
		globalLogger.SetOutput(os.Stderr)
		globalLogger.SetLevel(origLevel)
	}()

	globalLogger.SetOutput(&buf)

	// Test default / Info level (Init(false))
	Init(false)
	if globalLogger.GetLevel() != log.InfoLevel {
		t.Errorf("expected InfoLevel, got %v", globalLogger.GetLevel())
	}

	buf.Reset()
	Debug("should not appear")
	if buf.Len() > 0 {
		t.Errorf("expected no debug log to appear when level is Info, got: %q", buf.String())
	}

	buf.Reset()
	Info("info message", "key", "val")
	output := buf.String()
	if !strings.Contains(output, "info message") || !strings.Contains(output, "key=val") {
		t.Errorf("expected log to contain info message and key=val, got: %q", output)
	}

	// Test Debug level (Init(true))
	Init(true)
	if globalLogger.GetLevel() != log.DebugLevel {
		t.Errorf("expected DebugLevel, got %v", globalLogger.GetLevel())
	}

	buf.Reset()
	Debug("debug message", "foo", "bar")
	output = buf.String()
	if !strings.Contains(output, "debug message") || !strings.Contains(output, "foo=bar") {
		t.Errorf("expected log to contain debug message and foo=bar, got: %q", output)
	}

	// Test Warn and Error levels
	buf.Reset()
	Warn("warn message")
	output = buf.String()
	if !strings.Contains(output, "warn message") {
		t.Errorf("expected log to contain warn message, got: %q", output)
	}

	buf.Reset()
	Error("error message")
	output = buf.String()
	if !strings.Contains(output, "error message") {
		t.Errorf("expected log to contain error message, got: %q", output)
	}
}

func TestGetLogger(t *testing.T) {
	l := GetLogger()
	if l != globalLogger {
		t.Errorf("expected GetLogger to return globalLogger instance")
	}
}
