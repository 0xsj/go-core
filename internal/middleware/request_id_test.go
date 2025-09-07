// internal/middleware/request_id_test.go
package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestIDMiddleware_Name(t *testing.T) {
	mw := NewRequestIDMiddleware(nil)
	if mw.Name() != "request_id" {
		t.Errorf("Expected name 'request_id', got %s", mw.Name())
	}
}

func TestRequestIDMiddleware_Priority(t *testing.T) {
	mw := NewRequestIDMiddleware(nil)
	if mw.Priority() != PriorityRequestID {
		t.Errorf("Expected priority %d, got %d", PriorityRequestID, mw.Priority())
	}
}

func TestRequestIDMiddleware_Enabled(t *testing.T) {
	tests := []struct {
		name    string
		config  *RequestIDConfig
		enabled bool
	}{
		{"default config", nil, true},
		{"enabled true", &RequestIDConfig{Enabled: true}, true},
		{"enabled false", &RequestIDConfig{Enabled: false}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := NewRequestIDMiddleware(tt.config)
			if mw.Enabled(context.Background()) != tt.enabled {
				t.Errorf("Expected enabled %v, got %v", tt.enabled, mw.Enabled(context.Background()))
			}
		})
	}
}

func TestRequestIDMiddleware_GenerateID(t *testing.T) {
	config := &RequestIDConfig{
		Enabled:    true,
		HeaderName: "X-Request-ID",
		Generate:   true,
	}
	mw := NewRequestIDMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// Should generate and set request ID header
	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Error("Expected request ID header to be set")
	}

	if !strings.HasPrefix(requestID, "req-") {
		t.Errorf("Expected request ID to have 'req-' prefix, got %s", requestID)
	}

	if len(requestID) != 20 { // "req-" + 16 hex chars
		t.Errorf("Expected request ID length 20, got %d", len(requestID))
	}
}

func TestRequestIDMiddleware_UseExistingID(t *testing.T) {
	config := &RequestIDConfig{
		Enabled:    true,
		HeaderName: "X-Request-ID",
		Generate:   true,
	}
	mw := NewRequestIDMiddleware(config)

	existingID := "existing-123"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", existingID)
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// Should use existing request ID
	requestID := w.Header().Get("X-Request-ID")
	if requestID != existingID {
		t.Errorf("Expected to use existing ID %s, got %s", existingID, requestID)
	}
}

func TestRequestIDMiddleware_ContextPropagation(t *testing.T) {
	config := &RequestIDConfig{
		Enabled:    true,
		HeaderName: "X-Request-ID",
		Generate:   true,
	}
	mw := NewRequestIDMiddleware(config)

	var contextRequestID interface{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextRequestID = r.Context().Value(ContextKeyRequestID)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	headerRequestID := w.Header().Get("X-Request-ID")

	if contextRequestID == nil {
		t.Error("Expected request ID to be set in context")
	}

	if contextRequestID.(string) != headerRequestID {
		t.Errorf("Expected context ID %s to match header ID %s", contextRequestID, headerRequestID)
	}
}

func TestRequestIDMiddleware_CustomHeaderName(t *testing.T) {
	config := &RequestIDConfig{
		Enabled:    true,
		HeaderName: "X-Custom-ID",
		Generate:   true,
	}
	mw := NewRequestIDMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// Should use custom header name
	customID := w.Header().Get("X-Custom-ID")
	if customID == "" {
		t.Error("Expected custom header to be set")
	}

	// Default header should not be set
	defaultID := w.Header().Get("X-Request-ID")
	if defaultID != "" {
		t.Error("Expected default header not to be set")
	}
}

func TestRequestIDMiddleware_GenerateDisabled(t *testing.T) {
	config := &RequestIDConfig{
		Enabled:    true,
		HeaderName: "X-Request-ID",
		Generate:   false,
	}
	mw := NewRequestIDMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// Should not generate request ID when generation is disabled
	requestID := w.Header().Get("X-Request-ID")
	if requestID != "" {
		t.Errorf("Expected no request ID when generation disabled, got %s", requestID)
	}
}
