// internal/middleware/cors.go
package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig holds configuration for CORS middleware
type CORSConfig struct {
	Enabled            bool     `json:"enabled" env:"MIDDLEWARE_CORS_ENABLED" default:"true"`
	AllowedOrigins     []string `json:"allowed_origins" env:"MIDDLEWARE_CORS_ALLOWED_ORIGINS" default:"*"`
	AllowedMethods     []string `json:"allowed_methods" env:"MIDDLEWARE_CORS_ALLOWED_METHODS" default:"GET,POST,PUT,DELETE,OPTIONS"`
	AllowedHeaders     []string `json:"allowed_headers" env:"MIDDLEWARE_CORS_ALLOWED_HEADERS" default:"Content-Type,Authorization,X-Request-ID"`
	ExposedHeaders     []string `json:"exposed_headers" env:"MIDDLEWARE_CORS_EXPOSED_HEADERS" default:"X-Request-ID"`
	AllowCredentials   bool     `json:"allow_credentials" env:"MIDDLEWARE_CORS_ALLOW_CREDENTIALS" default:"false"`
	MaxAge             int      `json:"max_age" env:"MIDDLEWARE_CORS_MAX_AGE" default:"3600"`
	OptionsPassthrough bool     `json:"options_passthrough" env:"MIDDLEWARE_CORS_OPTIONS_PASSTHROUGH" default:"false"`
}

// CORSMiddleware handles Cross-Origin Resource Sharing
type CORSMiddleware struct {
	config *CORSConfig
}

// NewCORSMiddleware creates a new CORS middleware
func NewCORSMiddleware(config *CORSConfig) *CORSMiddleware {
	if config == nil {
		config = &CORSConfig{
			Enabled:            true,
			AllowedOrigins:     []string{"*"},
			AllowedMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:     []string{"Content-Type", "Authorization", "X-Request-ID"},
			ExposedHeaders:     []string{"X-Request-ID"},
			AllowCredentials:   false,
			MaxAge:             3600,
			OptionsPassthrough: false,
		}
	}

	return &CORSMiddleware{
		config: config,
	}
}

// Name returns the middleware name
func (m *CORSMiddleware) Name() string {
	return "cors"
}

// Priority returns the middleware priority
func (m *CORSMiddleware) Priority() int {
	return PriorityCORS
}

// Enabled checks if middleware should be applied
func (m *CORSMiddleware) Enabled(ctx context.Context) bool {
	return m.config.Enabled
}

// Handler returns the HTTP middleware handler
func (m *CORSMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				m.handlePreflight(w, r, origin)
				if !m.config.OptionsPassthrough {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}

			// Set CORS headers for actual requests
			m.setCORSHeaders(w, r, origin)

			next.ServeHTTP(w, r)
		})
	}
}

// handlePreflight handles OPTIONS preflight requests
func (m *CORSMiddleware) handlePreflight(w http.ResponseWriter, r *http.Request, origin string) {
	// Check if origin is allowed
	if !m.isOriginAllowed(origin) {
		return
	}

	// Set basic CORS headers
	w.Header().Set("Access-Control-Allow-Origin", m.getAllowedOrigin(origin))

	if m.config.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// Handle Access-Control-Request-Method
	if requestMethod := r.Header.Get("Access-Control-Request-Method"); requestMethod != "" {
		if m.isMethodAllowed(requestMethod) {
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(m.config.AllowedMethods, ", "))
		}
	}

	// Handle Access-Control-Request-Headers
	if requestHeaders := r.Header.Get("Access-Control-Request-Headers"); requestHeaders != "" {
		if m.areHeadersAllowed(requestHeaders) {
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(m.config.AllowedHeaders, ", "))
		}
	}

	// Set max age for preflight cache
	if m.config.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(m.config.MaxAge))
	}
}

// setCORSHeaders sets CORS headers for actual requests
func (m *CORSMiddleware) setCORSHeaders(w http.ResponseWriter, r *http.Request, origin string) {
	if !m.isOriginAllowed(origin) {
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", m.getAllowedOrigin(origin))

	if m.config.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	if len(m.config.ExposedHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(m.config.ExposedHeaders, ", "))
	}
}

// isOriginAllowed checks if the given origin is allowed
func (m *CORSMiddleware) isOriginAllowed(origin string) bool {
	if origin == "" {
		return true // Same-origin requests
	}

	for _, allowedOrigin := range m.config.AllowedOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			return true
		}

		// Support wildcard subdomains (e.g., *.example.com)
		if strings.HasPrefix(allowedOrigin, "*.") {
			domain := allowedOrigin[2:] // Remove "*."

			// Extract hostname from origin (remove protocol and port)
			originHost := origin
			if strings.HasPrefix(originHost, "http://") {
				originHost = originHost[7:]
			} else if strings.HasPrefix(originHost, "https://") {
				originHost = originHost[8:]
			}

			// Remove port if present
			if colonIndex := strings.Index(originHost, ":"); colonIndex != -1 {
				originHost = originHost[:colonIndex]
			}

			// Check if it matches the domain or is a subdomain
			if originHost == domain || strings.HasSuffix(originHost, "."+domain) {
				return true
			}
		}
	}

	return false
}

// getAllowedOrigin returns the appropriate Access-Control-Allow-Origin value
func (m *CORSMiddleware) getAllowedOrigin(origin string) string {
	if len(m.config.AllowedOrigins) == 1 && m.config.AllowedOrigins[0] == "*" {
		if m.config.AllowCredentials {
			return origin // Can't use * with credentials
		}
		return "*"
	}

	if m.isOriginAllowed(origin) {
		return origin
	}

	return ""
}

// isMethodAllowed checks if the given method is allowed
func (m *CORSMiddleware) isMethodAllowed(method string) bool {
	for _, allowedMethod := range m.config.AllowedMethods {
		if strings.EqualFold(allowedMethod, method) {
			return true
		}
	}
	return false
}

// areHeadersAllowed checks if the given headers are allowed
func (m *CORSMiddleware) areHeadersAllowed(headers string) bool {
	requestedHeaders := strings.Split(headers, ",")

	for _, header := range requestedHeaders {
		header = strings.TrimSpace(header)
		allowed := false

		for _, allowedHeader := range m.config.AllowedHeaders {
			if strings.EqualFold(allowedHeader, header) {
				allowed = true
				break
			}
		}

		if !allowed {
			return false
		}
	}

	return true
}
