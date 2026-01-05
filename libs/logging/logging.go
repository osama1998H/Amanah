package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ContextKey is a custom type for context keys to avoid collisions
type ContextKey string

// Context keys for logging
const (
	CtxKeyTraceID   ContextKey = "trace_id"
	CtxKeySpanID    ContextKey = "span_id"
	CtxKeyRequestID ContextKey = "request_id"
	CtxKeyUserID    ContextKey = "user_id"
)

// Level represents log level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// String returns the string representation of the log level
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a log level string
func ParseLevel(s string) Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR":
		return LevelError
	case "FATAL":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// Format represents log output format
type Format int

const (
	FormatJSON Format = iota
	FormatText
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	Level       string                 `json:"level"`
	Message     string                 `json:"message"`
	Service     string                 `json:"service,omitempty"`
	TraceID     string                 `json:"trace_id,omitempty"`
	SpanID      string                 `json:"span_id,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	Caller      string                 `json:"caller,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Stack       string                 `json:"stack,omitempty"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
	Duration    *time.Duration         `json:"duration_ms,omitempty"`
}

// LoggerConfig holds logger configuration
type LoggerConfig struct {
	Level         Level
	Format        Format
	Output        io.Writer
	ServiceName   string
	IncludeCaller bool
	MaskPatterns  []string // Patterns to mask sensitive data
}

// DefaultLoggerConfig returns default logger configuration
func DefaultLoggerConfig() *LoggerConfig {
	return &LoggerConfig{
		Level:         LevelInfo,
		Format:        FormatJSON,
		Output:        os.Stdout,
		ServiceName:   "amanah",
		IncludeCaller: true,
		MaskPatterns: []string{
			`"password"\s*:\s*"[^"]*"`,
			`"secret"\s*:\s*"[^"]*"`,
			`"token"\s*:\s*"[^"]*"`,
			`"api_key"\s*:\s*"[^"]*"`,
			`"credit_card"\s*:\s*"[^"]*"`,
			`"ssn"\s*:\s*"[^"]*"`,
		},
	}
}

// Logger is a structured logger
type Logger struct {
	config       *LoggerConfig
	mu           sync.Mutex
	maskPatterns []*regexp.Regexp
	fields       map[string]interface{} // Default fields
}

// NewLogger creates a new logger
func NewLogger(config *LoggerConfig) *Logger {
	if config == nil {
		config = DefaultLoggerConfig()
	}

	l := &Logger{
		config: config,
		fields: make(map[string]interface{}),
	}

	// Compile mask patterns
	for _, pattern := range config.MaskPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			l.maskPatterns = append(l.maskPatterns, re)
		}
	}

	return l
}

// WithField returns a new logger with an additional field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := &Logger{
		config:       l.config,
		maskPatterns: l.maskPatterns,
		fields:       make(map[string]interface{}),
	}

	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value

	return newLogger
}

// WithFields returns a new logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newLogger := &Logger{
		config:       l.config,
		maskPatterns: l.maskPatterns,
		fields:       make(map[string]interface{}),
	}

	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// WithContext returns a new logger with context values
func (l *Logger) WithContext(ctx context.Context) *Logger {
	newLogger := l.WithFields(nil)

	// Extract common context values using typed keys
	if traceID := ctx.Value(CtxKeyTraceID); traceID != nil {
		newLogger.fields["trace_id"] = traceID
	}
	if spanID := ctx.Value(CtxKeySpanID); spanID != nil {
		newLogger.fields["span_id"] = spanID
	}
	if requestID := ctx.Value(CtxKeyRequestID); requestID != nil {
		newLogger.fields["request_id"] = requestID
	}
	if userID := ctx.Value(CtxKeyUserID); userID != nil {
		newLogger.fields["user_id"] = userID
	}

	return newLogger
}

// WithError returns a new logger with error information
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return l.WithField("error", err.Error())
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(LevelDebug, msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(LevelInfo, msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(LevelWarn, msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(LevelError, msg, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.log(LevelFatal, msg, args...)
	os.Exit(1)
}

// log performs the actual logging
func (l *Logger) log(level Level, msg string, args ...interface{}) {
	if level < l.config.Level {
		return
	}

	entry := &LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     level.String(),
		Message:   fmt.Sprintf(msg, args...),
		Service:   l.config.ServiceName,
		Fields:    make(map[string]interface{}),
	}

	// Copy fields
	for k, v := range l.fields {
		switch k {
		case "trace_id":
			entry.TraceID = fmt.Sprint(v)
		case "span_id":
			entry.SpanID = fmt.Sprint(v)
		case "request_id":
			entry.RequestID = fmt.Sprint(v)
		case "user_id":
			entry.UserID = fmt.Sprint(v)
		case "error":
			entry.Error = fmt.Sprint(v)
		case "duration":
			if d, ok := v.(time.Duration); ok {
				entry.Duration = &d
			}
		default:
			entry.Fields[k] = v
		}
	}

	// Add caller information
	if l.config.IncludeCaller {
		if _, file, line, ok := runtime.Caller(2); ok {
			// Shorten the file path
			parts := strings.Split(file, "/")
			if len(parts) > 2 {
				file = strings.Join(parts[len(parts)-2:], "/")
			}
			entry.Caller = fmt.Sprintf("%s:%d", file, line)
		}
	}

	// Format and output
	var output string
	if l.config.Format == FormatJSON {
		data, _ := json.Marshal(entry)
		output = string(data)
	} else {
		output = l.formatText(entry)
	}

	// Mask sensitive data
	output = l.maskSensitiveData(output)

	l.mu.Lock()
	fmt.Fprintln(l.config.Output, output)
	l.mu.Unlock()
}

// formatText formats the entry as text
func (l *Logger) formatText(entry *LogEntry) string {
	var sb strings.Builder

	sb.WriteString(entry.Timestamp.Format("2006-01-02T15:04:05.000Z"))
	sb.WriteString(" ")
	sb.WriteString(entry.Level)
	sb.WriteString(" ")

	if entry.RequestID != "" {
		sb.WriteString("[")
		sb.WriteString(entry.RequestID)
		sb.WriteString("] ")
	}

	sb.WriteString(entry.Message)

	if entry.Error != "" {
		sb.WriteString(" error=")
		sb.WriteString(entry.Error)
	}

	if entry.Caller != "" {
		sb.WriteString(" caller=")
		sb.WriteString(entry.Caller)
	}

	for k, v := range entry.Fields {
		sb.WriteString(" ")
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(fmt.Sprint(v))
	}

	return sb.String()
}

// maskSensitiveData masks sensitive data in the output
func (l *Logger) maskSensitiveData(s string) string {
	for _, pattern := range l.maskPatterns {
		s = pattern.ReplaceAllStringFunc(s, func(match string) string {
			// Keep the key, mask the value
			parts := strings.SplitN(match, ":", 2)
			if len(parts) == 2 {
				return parts[0] + `: "***REDACTED***"`
			}
			return "***REDACTED***"
		})
	}
	return s
}

// Global logger instance
var defaultLogger = NewLogger(nil)

// SetDefaultLogger sets the default logger
func SetDefaultLogger(l *Logger) {
	defaultLogger = l
}

// GetDefaultLogger returns the default logger
func GetDefaultLogger() *Logger {
	return defaultLogger
}

// Package-level convenience functions

// SetOutput sets the destination for the logger (backwards compatibility)
func SetOutput(w io.Writer) {
	defaultLogger.config.Output = w
}

// SetLevel sets the log level
func SetLevel(level Level) {
	defaultLogger.config.Level = level
}

// Debug logs a debug message
func Debug(msg string, args ...interface{}) {
	defaultLogger.Debug(msg, args...)
}

// Info logs an info message
func Info(msg string, args ...interface{}) {
	defaultLogger.Info(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...interface{}) {
	defaultLogger.Warn(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...interface{}) {
	defaultLogger.Error(msg, args...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, args ...interface{}) {
	defaultLogger.Fatal(msg, args...)
}

// WithField returns a logger with an additional field
func WithField(key string, value interface{}) *Logger {
	return defaultLogger.WithField(key, value)
}

// WithFields returns a logger with additional fields
func WithFields(fields map[string]interface{}) *Logger {
	return defaultLogger.WithFields(fields)
}

// WithContext returns a logger with context values
func WithContext(ctx context.Context) *Logger {
	return defaultLogger.WithContext(ctx)
}

// WithError returns a logger with error information
func WithError(err error) *Logger {
	return defaultLogger.WithError(err)
}

// RequestLogger logs HTTP requests
type RequestLogger struct {
	logger *Logger
}

// NewRequestLogger creates a new request logger
func NewRequestLogger(logger *Logger) *RequestLogger {
	if logger == nil {
		logger = defaultLogger
	}
	return &RequestLogger{logger: logger}
}

// LogRequest logs an incoming request
func (rl *RequestLogger) LogRequest(method, path, clientIP, requestID string) {
	rl.logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"method":     method,
		"path":       path,
		"client_ip":  clientIP,
	}).Info("request started")
}

// LogResponse logs a completed request
func (rl *RequestLogger) LogResponse(method, path, requestID string, statusCode int, duration time.Duration) {
	l := rl.logger.WithFields(map[string]interface{}{
		"request_id":  requestID,
		"method":      method,
		"path":        path,
		"status_code": statusCode,
		"duration_ms": duration.Milliseconds(),
	})

	if statusCode >= 500 {
		l.Error("request failed")
	} else if statusCode >= 400 {
		l.Warn("request completed with client error")
	} else {
		l.Info("request completed")
	}
}
