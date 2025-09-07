// internal/middleware/request_id.go
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// RequestIDConfig holds configuration for request ID middleware
type RequestIDConfig struct {
	Enabled    bool   `json:"enabled" env:"MIDDLEWARE_REQUEST_ID_ENABLED" default:"true"`
	HeaderName string `json:"header_name" env:"MIDDLEWARE_REQUEST_ID_HEADER" default:"X-Request-ID"`
	Generate   bool   `json:"generate" env:"MIDDLEWARE_REQUEST_ID_GENERATE" default:"true"`
}

// RequestIDMiddleware generates and propagates request IDs
type RequestIDMiddleware struct {
	config *RequestIDConfig
}

// NewRequestIDMiddleware creates a new request ID middleware
func NewRequestIDMiddleware(config *RequestIDConfig) *RequestIDMiddleware {
	if config == nil {
		config = &RequestIDConfig{
			Enabled:    true,
			HeaderName: "X-Request-ID",
			Generate:   true,
		}
	}

	return &RequestIDMiddleware{
		config: config,
	}
}

// Name returns the middleware name
func (m *RequestIDMiddleware) Name() string {
	return "request_id"
}

// Priority returns the middleware priority
func (m *RequestIDMiddleware) Priority() int {
	return PriorityRequestID
}

// Enabled checks if middleware should be applied
func (m *RequestIDMiddleware) Enabled(ctx context.Context) bool {
	return m.config.Enabled
}

// Handler returns the HTTP middleware handler
func (m *RequestIDMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := m.getOrGenerateRequestID(r)

			// Add to response header
			w.Header().Set(m.config.HeaderName, requestID)

			// Add to request context
			ctx := context.WithValue(r.Context(), ContextKeyRequestID, requestID)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// getOrGenerateRequestID gets existing request ID from header or generates new one
func (m *RequestIDMiddleware) getOrGenerateRequestID(r *http.Request) string {
	// Try to get from existing header first
	if requestID := r.Header.Get(m.config.HeaderName); requestID != "" {
		return requestID
	}

	// Generate new one if configured to do so
	if m.config.Generate {
		return m.generateRequestID()
	}

	return ""
}

// generateRequestID creates a new random request ID
func (m *RequestIDMiddleware) generateRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a simple timestamp-based ID if crypto/rand fails
		return "req-" + hex.EncodeToString([]byte("fallback"))[:8]
	}
	return "req-" + hex.EncodeToString(bytes)[:16]
}
