// Package logger provides structured logging for kimi-go.
package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Level represents log level.
type Level int

const (
	// DEBUG level for detailed debugging.
	DEBUG Level = iota
	// INFO level for general information.
	INFO
	// WARN level for warnings.
	WARN
	// ERROR level for errors.
	ERROR
	// FATAL level for fatal errors.
	FATAL
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Fields represents structured log fields.
type Fields map[string]interface{}

// Logger represents a logger instance.
type Logger struct {
	mu        sync.RWMutex
	level     Level
	output    io.Writer
	formatter Formatter
	fields    Fields
}

// Formatter formats log entries.
type Formatter interface {
	Format(level Level, msg string, fields Fields, timestamp time.Time) ([]byte, error)
}

// TextFormatter formats logs as text.
type TextFormatter struct {
	DisableColors bool
	FullTimestamp bool
}

// Format implements Formatter.
func (f *TextFormatter) Format(level Level, msg string, fields Fields, timestamp time.Time) ([]byte, error) {
	var buf []byte

	// Timestamp
	if f.FullTimestamp {
		buf = append(buf, timestamp.Format("2006-01-02 15:04:05")...)
		buf = append(buf, ' ')
	}

	// Level
	buf = append(buf, '[')
	buf = append(buf, level.String()...)
	buf = append(buf, "] "...)

	// Message
	buf = append(buf, msg...)

	// Fields
	if len(fields) > 0 {
		buf = append(buf, " | "...)
		first := true
		for k, v := range fields {
			if !first {
				buf = append(buf, ", "...)
			}
			buf = append(buf, k...)
			buf = append(buf, '=')
			buf = append(buf, fmt.Sprintf("%v", v)...)
			first = false
		}
	}

	buf = append(buf, '\n')
	return buf, nil
}

// JSONFormatter formats logs as JSON.
type JSONFormatter struct{}

// Format implements Formatter.
func (f *JSONFormatter) Format(level Level, msg string, fields Fields, timestamp time.Time) ([]byte, error) {
	data := make(map[string]interface{})
	data["timestamp"] = timestamp.Format(time.RFC3339)
	data["level"] = level.String()
	data["msg"] = msg

	for k, v := range fields {
		data[k] = v
	}

	return json.Marshal(data)
}

var (
	// defaultLogger is the default logger instance.
	defaultLogger *Logger
	// initOnce ensures default logger is initialized once.
	initOnce sync.Once
)

// initDefaultLogger initializes the default logger.
func initDefaultLogger() {
	defaultLogger = NewLogger()
	defaultLogger.SetLevel(INFO)
}

// GetLogger returns the default logger instance.
func GetLogger() *Logger {
	initOnce.Do(initDefaultLogger)
	return defaultLogger
}

// NewLogger creates a new logger instance.
func NewLogger() *Logger {
	return &Logger{
		level:     INFO,
		output:    os.Stdout,
		formatter: &TextFormatter{FullTimestamp: true},
		fields:    make(Fields),
	}
}

// SetLevel sets the log level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel gets the log level.
func (l *Logger) GetLevel() Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// SetOutput sets the output writer.
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

// SetFormatter sets the formatter.
func (l *Logger) SetFormatter(f Formatter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.formatter = f
}

// WithFields creates a new logger with additional fields.
func (l *Logger) WithFields(fields Fields) *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	newFields := make(Fields)
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &Logger{
		level:     l.level,
		output:    l.output,
		formatter: l.formatter,
		fields:    newFields,
	}
}

// log logs a message at the specified level.
func (l *Logger) log(level Level, msg string, fields Fields) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if level < l.level {
		return
	}

	// Merge fields
	allFields := make(Fields)
	for k, v := range l.fields {
		allFields[k] = v
	}
	for k, v := range fields {
		allFields[k] = v
	}

	// Format and write
	data, err := l.formatter.Format(level, msg, allFields, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to format log: %v\n", err)
		return
	}

	l.output.Write(data)
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string) {
	l.log(DEBUG, msg, nil)
}

// Debugf logs a formatted debug message.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, args...), nil)
}

// Info logs an info message.
func (l *Logger) Info(msg string) {
	l.log(INFO, msg, nil)
}

// Infof logs a formatted info message.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, args...), nil)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string) {
	l.log(WARN, msg, nil)
}

// Warnf logs a formatted warning message.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, args...), nil)
}

// Error logs an error message.
func (l *Logger) Error(msg string) {
	l.log(ERROR, msg, nil)
}

// Errorf logs a formatted error message.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, args...), nil)
}

// Fatal logs a fatal message and exits.
func (l *Logger) Fatal(msg string) {
	l.log(FATAL, msg, nil)
	os.Exit(1)
}

// Fatalf logs a formatted fatal message and exits.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, args...), nil)
	os.Exit(1)
}

// Convenience functions for the default logger.

// Debug logs a debug message to the default logger.
func Debug(msg string) { GetLogger().Debug(msg) }

// Debugf logs a formatted debug message to the default logger.
func Debugf(format string, args ...interface{}) { GetLogger().Debugf(format, args...) }

// Info logs an info message to the default logger.
func Info(msg string) { GetLogger().Info(msg) }

// Infof logs a formatted info message to the default logger.
func Infof(format string, args ...interface{}) { GetLogger().Infof(format, args...) }

// Warn logs a warning message to the default logger.
func Warn(msg string) { GetLogger().Warn(msg) }

// Warnf logs a formatted warning message to the default logger.
func Warnf(format string, args ...interface{}) { GetLogger().Warnf(format, args...) }

// Error logs an error message to the default logger.
func Error(msg string) { GetLogger().Error(msg) }

// Errorf logs a formatted error message to the default logger.
func Errorf(format string, args ...interface{}) { GetLogger().Errorf(format, args...) }

// Fatal logs a fatal message to the default logger and exits.
func Fatal(msg string) { GetLogger().Fatal(msg) }

// Fatalf logs a formatted fatal message to the default logger and exits.
func Fatalf(format string, args ...interface{}) { GetLogger().Fatalf(format, args...) }

// SetLevel sets the log level for the default logger.
func SetLevel(level Level) { GetLogger().SetLevel(level) }

// GetLevel gets the log level for the default logger.
func GetLevel() Level { return GetLogger().GetLevel() }

// SetupLogging configures logging with file output.
func SetupLogging(logDir string, level Level) error {
	// Create log directory
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFile := filepath.Join(logDir, fmt.Sprintf("kimi_%s.log", timestamp))

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Create multi-writer to output to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, file)

	logger := GetLogger()
	logger.SetOutput(multiWriter)
	logger.SetLevel(level)

	Infof("Logging initialized. Log file: %s", logFile)

	return nil
}
