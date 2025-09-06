// internal/middleware/security_headers_test.go
package middleware

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersMiddleware_Name(t *testing.T) {
	mw := NewSecurityHeadersMiddleware(nil)
	if mw.Name() != "security_headers" {
		t.Errorf("Expected name 'security_headers', got %s", mw.Name())
	}
}

func TestSecurityHeadersMiddleware_Priority(t *testing.T) {
	mw := NewSecurityHeadersMiddleware(nil)
	if mw.Priority() != PrioritySecurity {
		t.Errorf("Expected priority %d, got %d", PrioritySecurity, mw.Priority())
	}
}

func TestSecurityHeadersMiddleware_Enabled(t *testing.T) {
	tests := []struct {
		name    string
		config  *SecurityHeadersConfig
		enabled bool
	}{
		{"default config", nil, true},
		{"enabled true", &SecurityHeadersConfig{Enabled: true}, true},
		{"enabled false", &SecurityHeadersConfig{Enabled: false}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := NewSecurityHeadersMiddleware(tt.config)
			if mw.Enabled(context.Background()) != tt.enabled {
				t.Errorf("Expected enabled %v, got %v", tt.enabled, mw.Enabled(context.Background()))
			}
		})
	}
}

func TestSecurityHeadersMiddleware_BasicHeaders(t *testing.T) {
	config := &SecurityHeadersConfig{
		Enabled:               true,
		ContentTypeOptions:    true,
		FrameOptions:          "DENY",
		XSSProtection:         true,
		ContentSecurityPolicy: "default-src 'self'",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		PermissionsPolicy:     "geolocation=(), microphone=(), camera=()",
		HSTSEnabled:           false,
	}
	mw := NewSecurityHeadersMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// Check all security headers are set
	expectedHeaders := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"X-XSS-Protection":        "1; mode=block",
		"Content-Security-Policy": "default-src 'self'",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Permissions-Policy":      "geolocation=(), microphone=(), camera=()",
	}

	for header, expected := range expectedHeaders {
		actual := w.Header().Get(header)
		if actual != expected {
			t.Errorf("Expected %s: %s, got %s", header, expected, actual)
		}
	}

	// HSTS should not be set (disabled and HTTP)
	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts != "" {
		t.Errorf("Expected no HSTS header, got %s", hsts)
	}
}

func TestSecurityHeadersMiddleware_DisabledOptions(t *testing.T) {
	config := &SecurityHeadersConfig{
		Enabled:               true,
		ContentTypeOptions:    false,
		FrameOptions:          "",
		XSSProtection:         false,
		ContentSecurityPolicy: "",
		ReferrerPolicy:        "",
		PermissionsPolicy:     "",
	}
	mw := NewSecurityHeadersMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// Check that disabled headers are not set
	disabledHeaders := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Content-Security-Policy",
		"Referrer-Policy",
		"Permissions-Policy",
	}

	for _, header := range disabledHeaders {
		actual := w.Header().Get(header)
		if actual != "" {
			t.Errorf("Expected %s header not to be set, got %s", header, actual)
		}
	}
}

func TestSecurityHeadersMiddleware_HSTS_HTTPS(t *testing.T) {
	config := &SecurityHeadersConfig{
		Enabled:               true,
		HSTSEnabled:           true,
		HSTSMaxAge:            31536000,
		HSTSIncludeSubdomains: true,
		HSTSPreload:           true,
	}
	mw := NewSecurityHeadersMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	// Create HTTPS request - simulate TLS connection
	req := httptest.NewRequest("GET", "https://example.com/test", nil)
	// Simulate HTTPS by setting a non-nil TLS field
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// HSTS should be set for HTTPS
	hsts := w.Header().Get("Strict-Transport-Security")
	expected := "max-age=31536000; includeSubDomains; preload"
	if hsts != expected {
		t.Errorf("Expected HSTS %s, got %s", expected, hsts)
	}
}

func TestSecurityHeadersMiddleware_HSTS_HTTP(t *testing.T) {
	config := &SecurityHeadersConfig{
		Enabled:     true,
		HSTSEnabled: true,
		HSTSMaxAge:  31536000,
	}
	mw := NewSecurityHeadersMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	// Create HTTP request (no TLS)
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// HSTS should not be set for HTTP
	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts != "" {
		t.Errorf("Expected no HSTS header for HTTP, got %s", hsts)
	}
}

func TestSecurityHeadersMiddleware_HSTS_Proxy_Headers(t *testing.T) {
	config := &SecurityHeadersConfig{
		Enabled:               true,
		HSTSEnabled:           true,
		HSTSMaxAge:            86400,
		HSTSIncludeSubdomains: false,
		HSTSPreload:           false,
	}
	mw := NewSecurityHeadersMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	tests := []struct {
		name     string
		headers  map[string]string
		expected string
	}{
		{
			name:     "X-Forwarded-Proto https",
			headers:  map[string]string{"X-Forwarded-Proto": "https"},
			expected: "max-age=86400",
		},
		{
			name:     "X-Forwarded-SSL on",
			headers:  map[string]string{"X-Forwarded-SSL": "on"},
			expected: "max-age=86400",
		},
		{
			name:     "X-Forwarded-Proto http",
			headers:  map[string]string{"X-Forwarded-Proto": "http"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			w := httptest.NewRecorder()

			mw.Handler()(handler).ServeHTTP(w, req)

			hsts := w.Header().Get("Strict-Transport-Security")
			if hsts != tt.expected {
				t.Errorf("Expected HSTS %s, got %s", tt.expected, hsts)
			}
		})
	}
}

func TestSecurityHeadersMiddleware_CustomFrameOptions(t *testing.T) {
	tests := []struct {
		name         string
		frameOptions string
		expected     string
	}{
		{"DENY", "DENY", "DENY"},
		{"SAMEORIGIN", "SAMEORIGIN", "SAMEORIGIN"},
		{"ALLOW-FROM", "ALLOW-FROM https://example.com", "ALLOW-FROM https://example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &SecurityHeadersConfig{
				Enabled:      true,
				FrameOptions: tt.frameOptions,
			}
			mw := NewSecurityHeadersMiddleware(config)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte("ok")); err != nil {
					t.Errorf("Failed to write response: %v", err)
				}
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			mw.Handler()(handler).ServeHTTP(w, req)

			frameOptions := w.Header().Get("X-Frame-Options")
			if frameOptions != tt.expected {
				t.Errorf("Expected X-Frame-Options %s, got %s", tt.expected, frameOptions)
			}
		})
	}
}

func TestSecurityHeadersMiddleware_PassThrough(t *testing.T) {
	mw := NewSecurityHeadersMiddleware(nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Custom-Header", "custom-value")
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte("created")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	// Should preserve original response
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	if w.Body.String() != "created" {
		t.Errorf("Expected 'created', got %s", w.Body.String())
	}

	// Should preserve custom headers
	customHeader := w.Header().Get("Custom-Header")
	if customHeader != "custom-value" {
		t.Errorf("Expected custom header value, got %s", customHeader)
	}

	// Should also add security headers
	xss := w.Header().Get("X-XSS-Protection")
	if xss != "1; mode=block" {
		t.Errorf("Expected XSS protection header, got %s", xss)
	}
}
