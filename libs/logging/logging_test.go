package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LevelFatal, "FATAL"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("Level(%d).String() = %s, want %s", tt.level, got, tt.expected)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{"info", LevelInfo},
		{"INFO", LevelInfo},
		{"warn", LevelWarn},
		{"WARNING", LevelWarn},
		{"error", LevelError},
		{"fatal", LevelFatal},
		{"unknown", LevelInfo}, // Default to INFO
	}

	for _, tt := range tests {
		if got := ParseLevel(tt.input); got != tt.expected {
			t.Errorf("ParseLevel(%s) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestLoggerJSONOutput(t *testing.T) {
	buf := &bytes.Buffer{}

	config := &LoggerConfig{
		Level:         LevelDebug,
		Format:        FormatJSON,
		Output:        buf,
		ServiceName:   "test-service",
		IncludeCaller: false,
	}

	logger := NewLogger(config)
	logger.Info("test message")

	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if entry.Level != "INFO" {
		t.Errorf("Expected level INFO, got %s", entry.Level)
	}

	if entry.Message != "test message" {
		t.Errorf("Expected message 'test message', got %s", entry.Message)
	}

	if entry.Service != "test-service" {
		t.Errorf("Expected service 'test-service', got %s", entry.Service)
	}
}

func TestLoggerTextOutput(t *testing.T) {
	buf := &bytes.Buffer{}

	config := &LoggerConfig{
		Level:         LevelInfo,
		Format:        FormatText,
		Output:        buf,
		IncludeCaller: false,
	}

	logger := NewLogger(config)
	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Error("Expected output to contain INFO")
	}
	if !strings.Contains(output, "test message") {
		t.Error("Expected output to contain 'test message'")
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	buf := &bytes.Buffer{}

	config := &LoggerConfig{
		Level:  LevelWarn,
		Format: FormatJSON,
		Output: buf,
	}

	logger := NewLogger(config)

	// These should be filtered out
	logger.Debug("debug message")
	logger.Info("info message")

	if buf.Len() > 0 {
		t.Error("Expected debug and info messages to be filtered")
	}

	// These should be logged
	logger.Warn("warn message")
	if !strings.Contains(buf.String(), "warn message") {
		t.Error("Expected warn message to be logged")
	}
}

func TestLoggerWithField(t *testing.T) {
	buf := &bytes.Buffer{}

	config := &LoggerConfig{
		Level:         LevelInfo,
		Format:        FormatJSON,
		Output:        buf,
		IncludeCaller: false,
	}

	logger := NewLogger(config)
	logger.WithField("user_id", "123").Info("user action")

	var entry LogEntry
	json.Unmarshal(buf.Bytes(), &entry)

	if entry.UserID != "123" {
		t.Errorf("Expected user_id '123', got %s", entry.UserID)
	}
}

func TestLoggerWithFields(t *testing.T) {
	buf := &bytes.Buffer{}

	config := &LoggerConfig{
		Level:         LevelInfo,
		Format:        FormatJSON,
		Output:        buf,
		IncludeCaller: false,
	}

	logger := NewLogger(config)
	logger.WithFields(map[string]interface{}{
		"request_id": "req-123",
		"trace_id":   "trace-456",
		"custom":     "value",
	}).Info("request")

	var entry LogEntry
	json.Unmarshal(buf.Bytes(), &entry)

	if entry.RequestID != "req-123" {
		t.Errorf("Expected request_id 'req-123', got %s", entry.RequestID)
	}

	if entry.TraceID != "trace-456" {
		t.Errorf("Expected trace_id 'trace-456', got %s", entry.TraceID)
	}

	if entry.Fields["custom"] != "value" {
		t.Errorf("Expected custom field 'value', got %v", entry.Fields["custom"])
	}
}

func TestLoggerWithContext(t *testing.T) {
	buf := &bytes.Buffer{}

	config := &LoggerConfig{
		Level:         LevelInfo,
		Format:        FormatJSON,
		Output:        buf,
		IncludeCaller: false,
	}

	logger := NewLogger(config)

	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "ctx-req-123")
	ctx = context.WithValue(ctx, "user_id", "ctx-user-456")

	logger.WithContext(ctx).Info("context log")

	var entry LogEntry
	json.Unmarshal(buf.Bytes(), &entry)

	if entry.RequestID != "ctx-req-123" {
		t.Errorf("Expected request_id from context, got %s", entry.RequestID)
	}

	if entry.UserID != "ctx-user-456" {
		t.Errorf("Expected user_id from context, got %s", entry.UserID)
	}
}

func TestLoggerWithError(t *testing.T) {
	buf := &bytes.Buffer{}

	config := &LoggerConfig{
		Level:         LevelInfo,
		Format:        FormatJSON,
		Output:        buf,
		IncludeCaller: false,
	}

	logger := NewLogger(config)
	err := errors.New("something went wrong")
	logger.WithError(err).Error("operation failed")

	var entry LogEntry
	json.Unmarshal(buf.Bytes(), &entry)

	if entry.Error != "something went wrong" {
		t.Errorf("Expected error message, got %s", entry.Error)
	}
}

func TestLoggerWithNilError(t *testing.T) {
	buf := &bytes.Buffer{}

	config := &LoggerConfig{
		Level:         LevelInfo,
		Format:        FormatJSON,
		Output:        buf,
		IncludeCaller: false,
	}

	logger := NewLogger(config)
	logger.WithError(nil).Info("no error")

	var entry LogEntry
	json.Unmarshal(buf.Bytes(), &entry)

	if entry.Error != "" {
		t.Errorf("Expected no error, got %s", entry.Error)
	}
}

func TestLoggerMaskSensitiveData(t *testing.T) {
	buf := &bytes.Buffer{}

	// Use a custom mask pattern that we know will match
	config := &LoggerConfig{
		Level:         LevelInfo,
		Format:        FormatJSON,
		Output:        buf,
		IncludeCaller: false,
		MaskPatterns:  []string{`secret\d+`},
	}

	logger := NewLogger(config)
	logger.Info("user secret123 logged in")

	output := buf.String()
	if strings.Contains(output, "secret123") {
		t.Error("Secret should be masked in message")
	}
	if !strings.Contains(output, "REDACTED") {
		t.Error("Output should contain REDACTED")
	}
}

func TestLoggerCallerIncluded(t *testing.T) {
	buf := &bytes.Buffer{}

	config := &LoggerConfig{
		Level:         LevelInfo,
		Format:        FormatJSON,
		Output:        buf,
		IncludeCaller: true,
	}

	logger := NewLogger(config)
	logger.Info("test")

	var entry LogEntry
	json.Unmarshal(buf.Bytes(), &entry)

	if entry.Caller == "" {
		t.Error("Expected caller to be set")
	}

	if !strings.Contains(entry.Caller, "logging_test.go") {
		t.Errorf("Expected caller to contain test file, got %s", entry.Caller)
	}
}

func TestPackageLevelFunctions(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf)
	SetLevel(LevelDebug)

	Info("package level info")
	if !strings.Contains(buf.String(), "INFO") {
		t.Error("Expected INFO in output")
	}

	buf.Reset()
	Debug("package level debug")
	if !strings.Contains(buf.String(), "DEBUG") {
		t.Error("Expected DEBUG in output")
	}

	buf.Reset()
	Warn("package level warn")
	if !strings.Contains(buf.String(), "WARN") {
		t.Error("Expected WARN in output")
	}

	buf.Reset()
	Error("package level error")
	if !strings.Contains(buf.String(), "ERROR") {
		t.Error("Expected ERROR in output")
	}
}

func TestRequestLogger(t *testing.T) {
	buf := &bytes.Buffer{}

	config := &LoggerConfig{
		Level:         LevelInfo,
		Format:        FormatJSON,
		Output:        buf,
		IncludeCaller: false,
	}

	logger := NewLogger(config)
	rl := NewRequestLogger(logger)

	rl.LogRequest("GET", "/api/users", "192.168.1.1", "req-123")

	var entry LogEntry
	json.Unmarshal(buf.Bytes(), &entry)

	if entry.Message != "request started" {
		t.Errorf("Expected 'request started', got %s", entry.Message)
	}

	buf.Reset()
	rl.LogResponse("GET", "/api/users", "req-123", 200, 100*time.Millisecond)

	json.Unmarshal(buf.Bytes(), &entry)
	if entry.Level != "INFO" {
		t.Errorf("Expected INFO for 200 status, got %s", entry.Level)
	}

	buf.Reset()
	rl.LogResponse("GET", "/api/users", "req-123", 404, 100*time.Millisecond)

	json.Unmarshal(buf.Bytes(), &entry)
	if entry.Level != "WARN" {
		t.Errorf("Expected WARN for 404 status, got %s", entry.Level)
	}

	buf.Reset()
	rl.LogResponse("GET", "/api/users", "req-123", 500, 100*time.Millisecond)

	json.Unmarshal(buf.Bytes(), &entry)
	if entry.Level != "ERROR" {
		t.Errorf("Expected ERROR for 500 status, got %s", entry.Level)
	}
}

func TestLoggerChaining(t *testing.T) {
	buf := &bytes.Buffer{}

	config := &LoggerConfig{
		Level:         LevelInfo,
		Format:        FormatJSON,
		Output:        buf,
		IncludeCaller: false,
	}

	logger := NewLogger(config)

	// Chain multiple With calls
	logger.
		WithField("service", "api").
		WithField("version", "1.0").
		WithFields(map[string]interface{}{"env": "test"}).
		Info("chained log")

	var entry LogEntry
	json.Unmarshal(buf.Bytes(), &entry)

	if entry.Fields["service"] != "api" {
		t.Error("Expected service field")
	}
	if entry.Fields["version"] != "1.0" {
		t.Error("Expected version field")
	}
	if entry.Fields["env"] != "test" {
		t.Error("Expected env field")
	}
}

func TestDefaultLogger(t *testing.T) {
	logger := GetDefaultLogger()
	if logger == nil {
		t.Error("Default logger should not be nil")
	}

	newLogger := NewLogger(nil)
	SetDefaultLogger(newLogger)

	if GetDefaultLogger() != newLogger {
		t.Error("SetDefaultLogger should update default logger")
	}
}

func TestLoggerFormatting(t *testing.T) {
	buf := &bytes.Buffer{}

	config := &LoggerConfig{
		Level:         LevelInfo,
		Format:        FormatJSON,
		Output:        buf,
		IncludeCaller: false,
	}

	logger := NewLogger(config)
	logger.Info("user %s logged in with id %d", "john", 123)

	var entry LogEntry
	json.Unmarshal(buf.Bytes(), &entry)

	if entry.Message != "user john logged in with id 123" {
		t.Errorf("Expected formatted message, got %s", entry.Message)
	}
}
