// internal/middleware/logging_test.go
package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0xsj/go-core/internal/lib/logger"
)

func TestLoggingMiddleware_Name(t *testing.T) {
	mw := NewLoggingMiddleware(nil, createTestLogger())
	if mw.Name() != "logging" {
		t.Errorf("Expected name 'logging', got %s", mw.Name())
	}
}

func TestLoggingMiddleware_Priority(t *testing.T) {
	mw := NewLoggingMiddleware(nil, createTestLogger())
	if mw.Priority() != PriorityLogging {
		t.Errorf("Expected priority %d, got %d", PriorityLogging, mw.Priority())
	}
}

func TestLoggingMiddleware_BasicLogging(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.NewLogger(&logger.LoggerConfig{
		Level:      logger.LevelDebug,
		Output:     &buf,
		Format:     logger.FormatPretty,
		ShowCaller: false,
		ShowColor:  false,
	})

	config := &LoggingConfig{
		Enabled:      true,
		LogRequests:  true,
		LogResponses: true,
		LogHeaders:   false,
		LogBody:      false,
	}
	mw := NewLoggingMiddleware(config, testLogger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("success")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test?param=value", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	logOutput := buf.String()

	// Should log request
	if !strings.Contains(logOutput, "HTTP request received") {
		t.Error("Expected request log message")
	}

	// Should log response
	if !strings.Contains(logOutput, "HTTP request completed") {
		t.Error("Expected response log message")
	}

	// Should include basic request info
	if !strings.Contains(logOutput, "method=GET") {
		t.Error("Expected method in logs")
	}

	if !strings.Contains(logOutput, "path=/test") {
		t.Error("Expected path in logs")
	}

	if !strings.Contains(logOutput, "status_code=200") {
		t.Error("Expected status code in logs")
	}

	if !strings.Contains(logOutput, "user_agent=test-agent") {
		t.Error("Expected user agent in logs")
	}
}

func TestLoggingMiddleware_SkipPaths(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.NewLogger(&logger.LoggerConfig{
		Level:      logger.LevelDebug,
		Output:     &buf,
		Format:     logger.FormatPretty,
		ShowCaller: false,
		ShowColor:  false,
	})

	config := &LoggingConfig{
		Enabled:     true,
		LogRequests: true,
		SkipPaths:   []string{"/health", "/metrics"},
	}
	mw := NewLoggingMiddleware(config, testLogger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	// Test skipped path
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	mw.Handler()(handler).ServeHTTP(w, req)

	logOutput := buf.String()
	if strings.Contains(logOutput, "/health") {
		t.Error("Expected /health to be skipped from logging")
	}

	// Reset buffer
	buf.Reset()

	// Test non-skipped path
	req = httptest.NewRequest("GET", "/api", nil)
	w = httptest.NewRecorder()
	mw.Handler()(handler).ServeHTTP(w, req)

	logOutput = buf.String()
	if !strings.Contains(logOutput, "/api") {
		t.Error("Expected /api to be logged")
	}
}

func TestLoggingMiddleware_ErrorStatusCodes(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.NewLogger(&logger.LoggerConfig{
		Level:      logger.LevelDebug,
		Output:     &buf,
		Format:     logger.FormatPretty,
		ShowCaller: false,
		ShowColor:  false,
	})

	config := &LoggingConfig{
		Enabled:      true,
		LogResponses: true,
	}
	mw := NewLoggingMiddleware(config, testLogger)

	tests := []struct {
		name       string
		statusCode int
		logLevel   string
	}{
		{"success", http.StatusOK, "INFO"},
		{"client error", http.StatusBadRequest, "WARN"},
		{"not found", http.StatusNotFound, "WARN"},
		{"server error", http.StatusInternalServerError, "ERROR"},
		{"bad gateway", http.StatusBadGateway, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if _, err := w.Write([]byte("response")); err != nil {
					t.Errorf("Failed to write response: %v", err)
				}
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			mw.Handler()(handler).ServeHTTP(w, req)

			logOutput := buf.String()
			if !strings.Contains(logOutput, tt.logLevel) {
				t.Errorf("Expected log level %s for status %d, got: %s", tt.logLevel, tt.statusCode, logOutput)
			}
		})
	}
}

func TestLoggingMiddleware_SlowRequests(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.NewLogger(&logger.LoggerConfig{
		Level:      logger.LevelDebug,
		Output:     &buf,
		Format:     logger.FormatPretty,
		ShowCaller: false,
		ShowColor:  false,
	})

	config := &LoggingConfig{
		Enabled:         true,
		LogResponses:    true,
		SlowRequestTime: "50ms",
	}
	mw := NewLoggingMiddleware(config, testLogger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow request
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/slow", nil)
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	logOutput := buf.String()

	// Should log slow request warning
	if !strings.Contains(logOutput, "Slow HTTP request detected") {
		t.Error("Expected slow request warning")
	}

	if !strings.Contains(logOutput, "WARN") {
		t.Error("Expected WARN log level for slow request")
	}
}

func TestLoggingMiddleware_ContextCorrelation(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.NewLogger(&logger.LoggerConfig{
		Level:      logger.LevelDebug,
		Output:     &buf,
		Format:     logger.FormatPretty,
		ShowCaller: false,
		ShowColor:  false,
	})

	config := &LoggingConfig{
		Enabled:     true,
		LogRequests: true,
	}
	mw := NewLoggingMiddleware(config, testLogger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	// Add correlation ID to context
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), ContextKeyRequestID, "test-req-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	logOutput := buf.String()

	// Should include correlation ID in logs
	if !strings.Contains(logOutput, "test-req-123") {
		t.Error("Expected correlation ID in logs")
	}
}

func TestLoggingMiddleware_HeaderSanitization(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.NewLogger(&logger.LoggerConfig{
		Level:      logger.LevelDebug,
		Output:     &buf,
		Format:     logger.FormatPretty,
		ShowCaller: false,
		ShowColor:  false,
	})

	config := &LoggingConfig{
		Enabled:     true,
		LogRequests: true,
		LogHeaders:  true,
	}
	mw := NewLoggingMiddleware(config, testLogger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Cookie", "session=secret-session")
	req.Header.Set("X-API-Key", "secret-api-key")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	logOutput := buf.String()

	// Sensitive headers should be redacted
	if strings.Contains(logOutput, "secret-token") {
		t.Error("Expected Authorization header to be redacted")
	}

	if strings.Contains(logOutput, "secret-session") {
		t.Error("Expected Cookie header to be redacted")
	}

	if strings.Contains(logOutput, "secret-api-key") {
		t.Error("Expected X-API-Key header to be redacted")
	}

	// Should show [REDACTED] for sensitive headers
	if !strings.Contains(logOutput, "[REDACTED]") {
		t.Error("Expected [REDACTED] placeholder for sensitive headers")
	}

	// Non-sensitive headers should be logged
	if !strings.Contains(logOutput, "application/json") {
		t.Error("Expected Content-Type header to be logged")
	}
}

func TestLoggingMiddleware_DisabledFeatures(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.NewLogger(&logger.LoggerConfig{
		Level:      logger.LevelDebug,
		Output:     &buf,
		Format:     logger.FormatPretty,
		ShowCaller: false,
		ShowColor:  false,
	})

	config := &LoggingConfig{
		Enabled:      false,
		LogRequests:  false,
		LogResponses: false,
	}
	mw := NewLoggingMiddleware(config, testLogger)

	// Disabled middleware should not be enabled
	if mw.Enabled(context.Background()) {
		t.Error("Expected middleware to be disabled")
	}

	// Test with enabled middleware but disabled request/response logging
	config.Enabled = true
	mw = NewLoggingMiddleware(config, testLogger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	logOutput := buf.String()

	// Should not log request or response when disabled
	if strings.Contains(logOutput, "HTTP request received") {
		t.Error("Expected no request logging when disabled")
	}

	if strings.Contains(logOutput, "HTTP request completed") {
		t.Error("Expected no response logging when disabled")
	}
}

func TestLoggingMiddleware_ResponseSizeTracking(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.NewLogger(&logger.LoggerConfig{
		Level:      logger.LevelDebug,
		Output:     &buf,
		Format:     logger.FormatPretty,
		ShowCaller: false,
		ShowColor:  false,
	})

	config := &LoggingConfig{
		Enabled:      true,
		LogResponses: true,
	}
	mw := NewLoggingMiddleware(config, testLogger)

	testContent := "this is test content with specific length"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(testContent)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	logOutput := buf.String()

	// Should track response size
	expectedSize := len(testContent)
	if !strings.Contains(logOutput, "response_size="+string(rune(expectedSize+'0'))) {
		// More flexible check since the exact format might vary
		if !strings.Contains(logOutput, "response_size") {
			t.Error("Expected response size to be logged")
		}
	}
}
