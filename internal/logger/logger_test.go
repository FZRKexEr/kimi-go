// Package logger provides structured logging for kimi-go.
package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{FATAL, "FATAL"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.want {
				t.Errorf("Level.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	logger := NewLogger()
	if logger == nil {
		t.Fatal("NewLogger() returned nil")
	}

	if logger.GetLevel() != INFO {
		t.Errorf("Expected default level INFO, got %v", logger.GetLevel())
	}
}

func TestLoggerSetLevel(t *testing.T) {
	logger := NewLogger()

	logger.SetLevel(DEBUG)
	if logger.GetLevel() != DEBUG {
		t.Errorf("Expected level DEBUG, got %v", logger.GetLevel())
	}

	logger.SetLevel(ERROR)
	if logger.GetLevel() != ERROR {
		t.Errorf("Expected level ERROR, got %v", logger.GetLevel())
	}
}

func TestLoggerDebug(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger()
	logger.SetOutput(&buf)
	logger.SetLevel(DEBUG)

	logger.Debug("debug message")

	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Errorf("Expected output to contain 'debug message', got: %s", output)
	}

	if !strings.Contains(output, "DEBUG") {
		t.Errorf("Expected output to contain 'DEBUG', got: %s", output)
	}
}

func TestLoggerDebugFiltered(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger()
	logger.SetOutput(&buf)
	logger.SetLevel(INFO) // DEBUG should be filtered

	logger.Debug("debug message")

	output := buf.String()
	if output != "" {
		t.Errorf("Expected no output, got: %s", output)
	}
}

func TestLoggerInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger()
	logger.SetOutput(&buf)

	logger.Info("info message")

	output := buf.String()
	if !strings.Contains(output, "info message") {
		t.Errorf("Expected output to contain 'info message', got: %s", output)
	}

	if !strings.Contains(output, "INFO") {
		t.Errorf("Expected output to contain 'INFO', got: %s", output)
	}
}

func TestLoggerWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger()
	logger.SetOutput(&buf)

	logger.Warn("warn message")

	output := buf.String()
	if !strings.Contains(output, "warn message") {
		t.Errorf("Expected output to contain 'warn message', got: %s", output)
	}

	if !strings.Contains(output, "WARN") {
		t.Errorf("Expected output to contain 'WARN', got: %s", output)
	}
}

func TestLoggerError(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger()
	logger.SetOutput(&buf)

	logger.Error("error message")

	output := buf.String()
	if !strings.Contains(output, "error message") {
		t.Errorf("Expected output to contain 'error message', got: %s", output)
	}

	if !strings.Contains(output, "ERROR") {
		t.Errorf("Expected output to contain 'ERROR', got: %s", output)
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger()
	logger.SetOutput(&buf)

	fieldLogger := logger.WithFields(Fields{
		"request_id": "12345",
		"user":       "testuser",
	})

	fieldLogger.Info("message with fields")

	output := buf.String()
	if !strings.Contains(output, "request_id") {
		t.Errorf("Expected output to contain 'request_id', got: %s", output)
	}

	if !strings.Contains(output, "12345") {
		t.Errorf("Expected output to contain '12345', got: %s", output)
	}

	if !strings.Contains(output, "user") {
		t.Errorf("Expected output to contain 'user', got: %s", output)
	}

	if !strings.Contains(output, "testuser") {
		t.Errorf("Expected output to contain 'testuser', got: %s", output)
	}
}

func TestLoggerDebugf(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger()
	logger.SetOutput(&buf)
	logger.SetLevel(DEBUG)

	logger.Debugf("formatted %s %d", "string", 42)

	output := buf.String()
	if !strings.Contains(output, "formatted string 42") {
		t.Errorf("Expected output to contain 'formatted string 42', got: %s", output)
	}
}

func TestLoggerInfof(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger()
	logger.SetOutput(&buf)

	logger.Infof("formatted %s %d", "string", 42)

	output := buf.String()
	if !strings.Contains(output, "formatted string 42") {
		t.Errorf("Expected output to contain 'formatted string 42', got: %s", output)
	}
}

func TestGetLogger(t *testing.T) {
	// GetLogger should always return the same instance
	logger1 := GetLogger()
	logger2 := GetLogger()

	if logger1 != logger2 {
		t.Error("GetLogger should return the same instance")
	}
}

func TestSetLevel(t *testing.T) {
	SetLevel(DEBUG)
	if GetLevel() != DEBUG {
		t.Errorf("Expected level DEBUG, got %v", GetLevel())
	}

	SetLevel(INFO)
	if GetLevel() != INFO {
		t.Errorf("Expected level INFO, got %v", GetLevel())
	}
}
