// internal/middleware/recovery_test.go
package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0xsj/go-core/internal/lib/logger"
)

func TestRecoveryMiddleware_Name(t *testing.T) {
	mw := NewRecoveryMiddleware(nil, createTestLogger(), false)
	if mw.Name() != "recovery" {
		t.Errorf("Expected name 'recovery', got %s", mw.Name())
	}
}

func TestRecoveryMiddleware_Priority(t *testing.T) {
	mw := NewRecoveryMiddleware(nil, createTestLogger(), false)
	if mw.Priority() != PriorityRecovery {
		t.Errorf("Expected priority %d, got %d", PriorityRecovery, mw.Priority())
	}
}

func TestRecoveryMiddleware_PanicRecovery(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.NewLogger(&logger.LoggerConfig{
		Level:      logger.LevelDebug,
		Output:     &buf,
		Format:     logger.FormatPretty,
		ShowCaller: false,
		ShowColor:  false,
	})

	config := &RecoveryConfig{
		Enabled:           true,
		LogStackTrace:     true,
		IncludeStackInDev: true,
	}
	mw := NewRecoveryMiddleware(config, testLogger, true)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// Should return 500 status
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	// Should return JSON error response
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected JSON content type, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Internal Server Error") {
		t.Error("Expected error message in response body")
	}

	// In dev mode, should include stack trace
	if !strings.Contains(body, "stack_trace") {
		t.Error("Expected stack trace in dev mode response")
	}

	// Should log the panic
	logOutput := buf.String()
	if !strings.Contains(logOutput, "Panic recovered") {
		t.Error("Expected panic to be logged")
	}
}

func TestRecoveryMiddleware_ProductionResponse(t *testing.T) {
	config := &RecoveryConfig{
		Enabled:           true,
		LogStackTrace:     true,
		IncludeStackInDev: false,
	}
	mw := NewRecoveryMiddleware(config, createTestLogger(), false) // Production mode

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	body := w.Body.String()

	// Should not include stack trace in production
	if strings.Contains(body, "stack_trace") {
		t.Error("Expected no stack trace in production mode")
	}

	// Should still have generic error message
	if !strings.Contains(body, "Internal Server Error") {
		t.Error("Expected generic error message in production")
	}
}

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	mw := NewRecoveryMiddleware(nil, createTestLogger(), false)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("success")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// Should pass through normally
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "success" {
		t.Errorf("Expected 'success', got %s", w.Body.String())
	}
}

func createTestLogger() logger.Logger {
	return logger.NewLogger(&logger.LoggerConfig{
		Level:      logger.LevelDebug,
		Output:     bytes.NewBuffer(nil),
		Format:     logger.FormatPretty,
		ShowCaller: false,
		ShowColor:  false,
	})
}
