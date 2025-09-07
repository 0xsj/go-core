// internal/middleware/security_headers.go
package middleware

import (
	"context"
	"net/http"
	"strconv"
)

// SecurityHeadersConfig holds configuration for security headers middleware
type SecurityHeadersConfig struct {
	Enabled                 bool   `json:"enabled" env:"MIDDLEWARE_SECURITY_ENABLED" default:"true"`
	ContentTypeOptions      bool   `json:"content_type_options" env:"MIDDLEWARE_SECURITY_CONTENT_TYPE_OPTIONS" default:"true"`
	FrameOptions            string `json:"frame_options" env:"MIDDLEWARE_SECURITY_FRAME_OPTIONS" default:"DENY"`
	XSSProtection           bool   `json:"xss_protection" env:"MIDDLEWARE_SECURITY_XSS_PROTECTION" default:"true"`
	ContentSecurityPolicy   string `json:"content_security_policy" env:"MIDDLEWARE_SECURITY_CSP" default:"default-src 'self'"`
	StrictTransportSecurity string `json:"strict_transport_security" env:"MIDDLEWARE_SECURITY_HSTS" default:"max-age=31536000; includeSubDomains"`
	ReferrerPolicy          string `json:"referrer_policy" env:"MIDDLEWARE_SECURITY_REFERRER_POLICY" default:"strict-origin-when-cross-origin"`
	PermissionsPolicy       string `json:"permissions_policy" env:"MIDDLEWARE_SECURITY_PERMISSIONS_POLICY" default:"geolocation=(), microphone=(), camera=()"`
	HSTSEnabled             bool   `json:"hsts_enabled" env:"MIDDLEWARE_SECURITY_HSTS_ENABLED" default:"false"`
	HSTSMaxAge              int    `json:"hsts_max_age" env:"MIDDLEWARE_SECURITY_HSTS_MAX_AGE" default:"31536000"`
	HSTSIncludeSubdomains   bool   `json:"hsts_include_subdomains" env:"MIDDLEWARE_SECURITY_HSTS_SUBDOMAINS" default:"true"`
	HSTSPreload             bool   `json:"hsts_preload" env:"MIDDLEWARE_SECURITY_HSTS_PRELOAD" default:"false"`
}

// SecurityHeadersMiddleware adds security headers to responses
type SecurityHeadersMiddleware struct {
	config *SecurityHeadersConfig
}

// NewSecurityHeadersMiddleware creates a new security headers middleware
func NewSecurityHeadersMiddleware(config *SecurityHeadersConfig) *SecurityHeadersMiddleware {
	if config == nil {
		config = &SecurityHeadersConfig{
			Enabled:                 true,
			ContentTypeOptions:      true,
			FrameOptions:            "DENY",
			XSSProtection:           true,
			ContentSecurityPolicy:   "default-src 'self'",
			StrictTransportSecurity: "max-age=31536000; includeSubDomains",
			ReferrerPolicy:          "strict-origin-when-cross-origin",
			PermissionsPolicy:       "geolocation=(), microphone=(), camera=()",
			HSTSEnabled:             false,
			HSTSMaxAge:              31536000,
			HSTSIncludeSubdomains:   true,
			HSTSPreload:             false,
		}
	}

	return &SecurityHeadersMiddleware{
		config: config,
	}
}

// Name returns the middleware name
func (m *SecurityHeadersMiddleware) Name() string {
	return "security_headers"
}

// Priority returns the middleware priority
func (m *SecurityHeadersMiddleware) Priority() int {
	return PrioritySecurity
}

// Enabled checks if middleware should be applied
func (m *SecurityHeadersMiddleware) Enabled(ctx context.Context) bool {
	return m.config.Enabled
}

// Handler returns the HTTP middleware handler
func (m *SecurityHeadersMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.setSecurityHeaders(w, r)
			next.ServeHTTP(w, r)
		})
	}
}

// setSecurityHeaders applies all configured security headers
func (m *SecurityHeadersMiddleware) setSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	// X-Content-Type-Options: Prevents MIME type sniffing
	if m.config.ContentTypeOptions {
		w.Header().Set("X-Content-Type-Options", "nosniff")
	}

	// X-Frame-Options: Prevents clickjacking
	if m.config.FrameOptions != "" {
		w.Header().Set("X-Frame-Options", m.config.FrameOptions)
	}

	// X-XSS-Protection: Enables XSS filtering (legacy, but still useful)
	if m.config.XSSProtection {
		w.Header().Set("X-XSS-Protection", "1; mode=block")
	}

	// Content-Security-Policy: Prevents XSS and data injection
	if m.config.ContentSecurityPolicy != "" {
		w.Header().Set("Content-Security-Policy", m.config.ContentSecurityPolicy)
	}

	// Referrer-Policy: Controls referrer information
	if m.config.ReferrerPolicy != "" {
		w.Header().Set("Referrer-Policy", m.config.ReferrerPolicy)
	}

	// Permissions-Policy: Controls browser features
	if m.config.PermissionsPolicy != "" {
		w.Header().Set("Permissions-Policy", m.config.PermissionsPolicy)
	}

	// Strict-Transport-Security: Enforces HTTPS (only if enabled and HTTPS)
	if m.config.HSTSEnabled && m.isHTTPS(r) {
		hstsValue := "max-age=" + strconv.Itoa(m.config.HSTSMaxAge)

		if m.config.HSTSIncludeSubdomains {
			hstsValue += "; includeSubDomains"
		}

		if m.config.HSTSPreload {
			hstsValue += "; preload"
		}

		w.Header().Set("Strict-Transport-Security", hstsValue)
	}
}

// isHTTPS determines if the request is over HTTPS
func (m *SecurityHeadersMiddleware) isHTTPS(r *http.Request) bool {
	// Check direct TLS connection
	if r.TLS != nil {
		return true
	}

	// Check X-Forwarded-Proto header (for reverse proxies)
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" {
		return true
	}

	// Check X-Forwarded-SSL header
	if ssl := r.Header.Get("X-Forwarded-SSL"); ssl == "on" {
		return true
	}

	return false
}
