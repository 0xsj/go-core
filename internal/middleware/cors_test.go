// internal/middleware/cors_test.go
package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSMiddleware_Name(t *testing.T) {
	mw := NewCORSMiddleware(nil)
	if mw.Name() != "cors" {
		t.Errorf("Expected name 'cors', got %s", mw.Name())
	}
}

func TestCORSMiddleware_Priority(t *testing.T) {
	mw := NewCORSMiddleware(nil)
	if mw.Priority() != PriorityCORS {
		t.Errorf("Expected priority %d, got %d", PriorityCORS, mw.Priority())
	}
}

func TestCORSMiddleware_SimpleRequest(t *testing.T) {
	config := &CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
		ExposedHeaders: []string{"X-Request-ID"},
	}
	mw := NewCORSMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// Should allow the origin
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://example.com" {
		t.Errorf("Expected origin https://example.com, got %s", origin)
	}

	// Should expose headers
	exposed := w.Header().Get("Access-Control-Expose-Headers")
	if exposed != "X-Request-ID" {
		t.Errorf("Expected exposed headers X-Request-ID, got %s", exposed)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestCORSMiddleware_BlockedOrigin(t *testing.T) {
	config := &CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://allowed.com"},
	}
	mw := NewCORSMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://blocked.com")
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// Should not set CORS headers for blocked origin
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "" {
		t.Errorf("Expected no CORS headers for blocked origin, got %s", origin)
	}

	// Request should still succeed (CORS is enforced by browser)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestCORSMiddleware_PreflightRequest(t *testing.T) {
	config := &CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST", "PUT"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         3600,
	}
	mw := NewCORSMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for preflight")
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// Should return 204 No Content
	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	// Should set preflight headers
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://example.com" {
		t.Errorf("Expected origin https://example.com, got %s", origin)
	}

	methods := w.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "POST") {
		t.Errorf("Expected methods to contain POST, got %s", methods)
	}

	headers := w.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(headers, "Content-Type") {
		t.Errorf("Expected headers to contain Content-Type, got %s", headers)
	}

	maxAge := w.Header().Get("Access-Control-Max-Age")
	if maxAge != "3600" {
		t.Errorf("Expected max-age 3600, got %s", maxAge)
	}
}

func TestCORSMiddleware_WildcardOrigin(t *testing.T) {
	config := &CORSConfig{
		Enabled:          true,
		AllowedOrigins:   []string{"*"},
		AllowCredentials: false,
	}
	mw := NewCORSMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// Should allow any origin with wildcard
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("Expected wildcard origin *, got %s", origin)
	}
}

func TestCORSMiddleware_CredentialsWithWildcard(t *testing.T) {
	config := &CORSConfig{
		Enabled:          true,
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
	}
	mw := NewCORSMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://specific-origin.com")
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// With credentials, should return specific origin instead of wildcard
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://specific-origin.com" {
		t.Errorf("Expected specific origin with credentials, got %s", origin)
	}

	credentials := w.Header().Get("Access-Control-Allow-Credentials")
	if credentials != "true" {
		t.Errorf("Expected credentials true, got %s", credentials)
	}
}

func TestCORSMiddleware_SubdomainWildcard(t *testing.T) {
	config := &CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*.example.com"},
	}
	mw := NewCORSMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	tests := []struct {
		origin  string
		allowed bool
	}{
		{"https://api.example.com", true},
		{"https://app.example.com", true},
		{"https://example.com", true},
		{"https://evil.com", false},
		{"https://example.com.evil.com", false},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", tt.origin)
		w := httptest.NewRecorder()

		mw.Handler()(handler).ServeHTTP(w, req)

		origin := w.Header().Get("Access-Control-Allow-Origin")
		if tt.allowed && origin != tt.origin {
			t.Errorf("Expected origin %s to be allowed, got %s", tt.origin, origin)
		} else if !tt.allowed && origin != "" {
			t.Errorf("Expected origin %s to be blocked, but got %s", tt.origin, origin)
		}
	}
}

func TestCORSMiddleware_Disabled(t *testing.T) {
	config := &CORSConfig{
		Enabled: false,
	}
	mw := NewCORSMiddleware(config)

	if mw.Enabled(context.Background()) {
		t.Error("Expected middleware to be disabled")
	}
}
