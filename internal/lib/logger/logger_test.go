// internal/lib/logger/logger_test.go
package logger

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestLogger_BasicLogging(t *testing.T) {
	var buf bytes.Buffer
	config := &LoggerConfig{
		Level:      LevelDebug,
		Output:     &buf,
		Format:     FormatPretty,
		ShowCaller: true,
		ShowColor:  true, // Disable colors for easier testing
	}

	logger := NewLogger(config)

	// Test different log levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warning message")
	logger.Error("error message")

	output := buf.String()

	// Check that all messages are present
	if !strings.Contains(output, "debug message") {
		t.Error("Debug message not found in output")
	}
	if !strings.Contains(output, "info message") {
		t.Error("Info message not found in output")
	}
	if !strings.Contains(output, "warning message") {
		t.Error("Warning message not found in output")
	}
	if !strings.Contains(output, "error message") {
		t.Error("Error message not found in output")
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	config := &LoggerConfig{
		Level:     LevelWarn,
		Output:    &buf,
		Format:    FormatPretty,
		ShowColor: false,
	}

	logger := NewLogger(config)

	logger.Debug("should not appear")
	logger.Info("should not appear")
	logger.Warn("should appear")
	logger.Error("should appear")

	output := buf.String()

	if strings.Contains(output, "should not appear") {
		t.Error("Debug/Info messages should be filtered out")
	}
	if !strings.Contains(output, "should appear") {
		t.Error("Warn/Error messages should appear")
	}
}

func TestLogger_StructuredFields(t *testing.T) {
	var buf bytes.Buffer
	config := &LoggerConfig{
		Level:     LevelInfo,
		Output:    &buf,
		Format:    FormatPretty,
		ShowColor: false,
	}

	logger := NewLogger(config)

	logger.Info("test message",
		String("user_id", "123"),
		Int("count", 42),
		Bool("active", true),
	)

	output := buf.String()

	if !strings.Contains(output, "user_id=123") {
		t.Error("String field not found")
	}
	if !strings.Contains(output, "count=42") {
		t.Error("Int field not found")
	}
	if !strings.Contains(output, "active=true") {
		t.Error("Bool field not found")
	}
}

func TestLogger_WithFields(t *testing.T) {
	var buf bytes.Buffer
	config := &LoggerConfig{
		Level:     LevelInfo,
		Output:    &buf,
		Format:    FormatPretty,
		ShowColor: false,
	}

	baseLogger := NewLogger(config)
	contextLogger := baseLogger.WithFields(
		String("service", "auth"),
		String("version", "1.0.0"),
	)

	contextLogger.Info("operation completed", String("duration", "100ms"))

	output := buf.String()

	if !strings.Contains(output, "service=auth") {
		t.Error("Context field 'service' not found")
	}
	if !strings.Contains(output, "version=1.0.0") {
		t.Error("Context field 'version' not found")
	}
	if !strings.Contains(output, "duration=100ms") {
		t.Error("Additional field 'duration' not found")
	}
}

func TestLogger_ContextualLogging(t *testing.T) {
	var buf bytes.Buffer
	config := &LoggerConfig{
		Level:     LevelInfo,
		Output:    &buf,
		Format:    FormatPretty,
		ShowColor: false,
	}

	logger := NewLogger(config)

	// Create context with correlation ID
	ctx := context.WithValue(context.Background(), ContextKeyRequestID, "req-123")
	ctx = context.WithValue(ctx, ContextKeyTraceID, "trace-456")

	contextLogger := logger.WithContext(ctx)
	contextLogger.Info("processing request")

	output := buf.String()

	if !strings.Contains(output, "request_id=req-123") {
		t.Error("Request ID from context not found")
	}
	if !strings.Contains(output, "trace_id=trace-456") {
		t.Error("Trace ID from context not found")
	}
}

func TestLogger_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	config := &LoggerConfig{
		Level:      LevelInfo,
		Output:     &buf,
		Format:     FormatJSON,
		ShowCaller: false,
	}

	logger := NewLogger(config)
	logger.Info("test message", String("key", "value"))

	output := buf.String()

	// Basic JSON structure checks
	if !strings.Contains(output, `"level":"INFO"`) {
		t.Error("JSON level not found")
	}
	if !strings.Contains(output, `"message":"test message"`) {
		t.Error("JSON message not found")
	}
	if !strings.Contains(output, `"key":"value"`) {
		t.Error("JSON field not found")
	}
}

func TestLogger_WithError(t *testing.T) {
	var buf bytes.Buffer
	config := &LoggerConfig{
		Level:     LevelError,
		Output:    &buf,
		Format:    FormatPretty,
		ShowColor: false,
	}

	logger := NewLogger(config)
	err := errors.New("something went wrong")

	logger.WithError(err).Error("operation failed")

	output := buf.String()

	if !strings.Contains(output, "something went wrong") {
		t.Error("Error message not found in output")
	}
}

func TestLogger_CorrelationMethods(t *testing.T) {
	var buf bytes.Buffer
	config := &LoggerConfig{
		Level:     LevelInfo,
		Output:    &buf,
		Format:    FormatPretty,
		ShowColor: false,
	}

	logger := NewLogger(config)

	logger.WithRequestID("req-123").
		WithTraceID("trace-456").
		WithCorrelationID("corr-789").
		WithUserID("user-000").
		Info("user action")

	output := buf.String()

	if !strings.Contains(output, "request_id=req-123") {
		t.Error("Request ID not found")
	}
	if !strings.Contains(output, "trace_id=trace-456") {
		t.Error("Trace ID not found")
	}
	if !strings.Contains(output, "correlation_id=corr-789") {
		t.Error("Correlation ID not found")
	}
	if !strings.Contains(output, "user_id=user-000") {
		t.Error("User ID not found")
	}
}

func TestLogger_LevelControl(t *testing.T) {
	config := DefaultConfig()
	logger := NewLogger(config)

	// Test initial level
	if logger.GetLevel() != LevelInfo {
		t.Error("Default level should be Info")
	}

	// Test level change
	logger.SetLevel(LevelError)
	if logger.GetLevel() != LevelError {
		t.Error("Level should be changed to Error")
	}

	// Test level checking
	if logger.IsLevelEnabled(LevelDebug) {
		t.Error("Debug should be disabled when level is Error")
	}
	if !logger.IsLevelEnabled(LevelError) {
		t.Error("Error should be enabled when level is Error")
	}
}

// Benchmark tests
func BenchmarkLogger_Info(b *testing.B) {
	config := &LoggerConfig{
		Level:      LevelInfo,
		Output:     io.Discard,
		Format:     FormatPretty,
		ShowCaller: false,
		ShowColor:  false,
	}

	logger := NewLogger(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message", String("iteration", string(rune(i))))
	}
}

func BenchmarkLogger_InfoWithFields(b *testing.B) {
	config := &LoggerConfig{
		Level:      LevelInfo,
		Output:     io.Discard,
		Format:     FormatPretty,
		ShowCaller: false,
		ShowColor:  false,
	}

	logger := NewLogger(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message",
			String("field1", "value1"),
			Int("field2", i),
			Bool("field3", true),
		)
	}
}

func BenchmarkLogger_LevelFiltered(b *testing.B) {
	config := &LoggerConfig{
		Level:      LevelError,
		Output:     io.Discard,
		Format:     FormatPretty,
		ShowCaller: false,
		ShowColor:  false,
	}

	logger := NewLogger(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Debug("this should be filtered", String("iteration", string(rune(i))))
	}
}

func ExampleLogger_colorizedOutput() {
	config := &LoggerConfig{
		Level:      LevelDebug,
		Output:     os.Stdout,
		Format:     FormatPretty,
		ShowCaller: true,
		ShowColor:  true,
	}

	logger := NewLogger(config)

	logger.Debug("This is a debug message")
	logger.Info("Server starting successfully", String("port", "8080"))
	logger.Warn("High memory usage detected", Int("usage_percent", 85))
	logger.Error("Database connection failed", Err(errors.New("connection timeout")))

	ctx := context.WithValue(context.Background(), ContextKeyRequestID, "req-12345")
	ctx = context.WithValue(ctx, ContextKeyUserID, "user-67890")

	contextLogger := logger.WithContext(ctx)
	contextLogger.Info("User login successful", String("method", "oauth2"))

	logger.WithFields(
		String("component", "auth-service"),
		String("version", "1.2.3"),
	).Info("Service initialized")
}
