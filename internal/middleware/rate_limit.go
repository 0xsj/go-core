// internal/middleware/rate_limit.go
package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimitConfig holds configuration for rate limiting middleware
type RateLimitConfig struct {
	Enabled        bool          `json:"enabled" env:"MIDDLEWARE_RATE_LIMIT_ENABLED" default:"true"`
	RequestsPerMin int           `json:"requests_per_min" env:"MIDDLEWARE_RATE_LIMIT_RPM" default:"60"`
	BurstSize      int           `json:"burst_size" env:"MIDDLEWARE_RATE_LIMIT_BURST" default:"10"`
	WindowSize     time.Duration `json:"window_size" env:"MIDDLEWARE_RATE_LIMIT_WINDOW" default:"1m"`
	KeyBy          string        `json:"key_by" env:"MIDDLEWARE_RATE_LIMIT_KEY_BY" default:"ip"`
	SkipPaths      []string      `json:"skip_paths" env:"MIDDLEWARE_RATE_LIMIT_SKIP_PATHS" default:"/health"`
	HeadersEnabled bool          `json:"headers_enabled" env:"MIDDLEWARE_RATE_LIMIT_HEADERS" default:"true"`
}

// RateLimitMiddleware provides rate limiting functionality
type RateLimitMiddleware struct {
	config    *RateLimitConfig
	limiter   *TokenBucketLimiter
	skipPaths map[string]bool
}

// NewRateLimitMiddleware creates a new rate limiting middleware
func NewRateLimitMiddleware(config *RateLimitConfig) *RateLimitMiddleware {
	if config == nil {
		config = &RateLimitConfig{
			Enabled:        true,
			RequestsPerMin: 60,
			BurstSize:      10,
			WindowSize:     time.Minute,
			KeyBy:          "ip",
			SkipPaths:      []string{"/health"},
			HeadersEnabled: true,
		}
	}

	// Convert skip paths to map for O(1) lookup
	skipPaths := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipPaths[path] = true
	}

	limiter := NewTokenBucketLimiter(config.RequestsPerMin, config.BurstSize, config.WindowSize)

	return &RateLimitMiddleware{
		config:    config,
		limiter:   limiter,
		skipPaths: skipPaths,
	}
}

// Name returns the middleware name
func (m *RateLimitMiddleware) Name() string {
	return "rate_limit"
}

// Priority returns the middleware priority
func (m *RateLimitMiddleware) Priority() int {
	return PriorityRateLimit
}

// Enabled checks if middleware should be applied
func (m *RateLimitMiddleware) Enabled(ctx context.Context) bool {
	return m.config.Enabled
}

// Handler returns the HTTP middleware handler
func (m *RateLimitMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting for configured paths
			if m.skipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Get rate limit key
			key := m.getRateLimitKey(r)

			// Check rate limit
			allowed, remaining, resetTime := m.limiter.Allow(key)

			// Add rate limit headers if enabled
			if m.config.HeadersEnabled {
				m.setRateLimitHeaders(w, remaining, resetTime)
			}

			if !allowed {
				m.handleRateLimitExceeded(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getRateLimitKey extracts the key for rate limiting
func (m *RateLimitMiddleware) getRateLimitKey(r *http.Request) string {
	switch m.config.KeyBy {
	case "ip":
		return m.getClientIP(r)
	case "user":
		if userID := r.Header.Get("X-User-ID"); userID != "" {
			return "user:" + userID
		}
		return m.getClientIP(r) // Fallback to IP
	case "api_key":
		if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
			return "api:" + apiKey
		}
		return m.getClientIP(r) // Fallback to IP
	default:
		return m.getClientIP(r)
	}
}

// getClientIP extracts the real client IP
func (m *RateLimitMiddleware) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if ip := net.ParseIP(xff); ip != nil {
			return ip.String()
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if ip := net.ParseIP(xri); ip != nil {
			return ip.String()
		}
	}

	// Use remote address as fallback
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

// setRateLimitHeaders adds rate limiting headers to the response
func (m *RateLimitMiddleware) setRateLimitHeaders(w http.ResponseWriter, remaining int, resetTime time.Time) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(m.config.RequestsPerMin))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))
}

func (m *RateLimitMiddleware) handleRateLimitExceeded(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)

	response := fmt.Sprintf(`{
	"error": "Rate Limit Exceeded",
	"message": "Too many requests. Maximum %d requests per minute allowed.",
	"retry_after": %d
}`, m.config.RequestsPerMin, int(m.config.WindowSize.Seconds()))

	if _, err := w.Write([]byte(response)); err != nil {
		// Log the error but don't panic - we're already handling an error condition
		// In a real implementation, you might want to log this to your logger
		_ = err // Acknowledge we're intentionally ignoring this specific error
	}
}

// TokenBucketLimiter implements token bucket rate limiting
type TokenBucketLimiter struct {
	rate          int           // tokens per window
	burst         int           // bucket size
	window        time.Duration // time window
	buckets       map[string]*bucket
	mu            sync.RWMutex
	cleanupTicker *time.Ticker
}

type bucket struct {
	tokens    int
	lastSeen  time.Time
	resetTime time.Time
}

// NewTokenBucketLimiter creates a new token bucket limiter
func NewTokenBucketLimiter(rate, burst int, window time.Duration) *TokenBucketLimiter {
	limiter := &TokenBucketLimiter{
		rate:          rate,
		burst:         burst,
		window:        window,
		buckets:       make(map[string]*bucket),
		cleanupTicker: time.NewTicker(time.Minute),
	}

	// Start cleanup goroutine
	go limiter.cleanup()

	return limiter
}

// Allow checks if a request is allowed for the given key
func (l *TokenBucketLimiter) Allow(key string) (allowed bool, remaining int, resetTime time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, exists := l.buckets[key]

	if !exists || now.Sub(b.resetTime) >= 0 {
		// Create new bucket or reset existing one
		b = &bucket{
			tokens:    l.burst - 1, // Consume one token for this request
			lastSeen:  now,
			resetTime: now.Add(l.window),
		}
		l.buckets[key] = b
		return true, b.tokens, b.resetTime
	}

	// Update last seen
	b.lastSeen = now

	if b.tokens > 0 {
		b.tokens--
		return true, b.tokens, b.resetTime
	}

	return false, 0, b.resetTime
}

// cleanup removes old buckets to prevent memory leaks
func (l *TokenBucketLimiter) cleanup() {
	for range l.cleanupTicker.C {
		l.mu.Lock()
		now := time.Now()

		for key, bucket := range l.buckets {
			// Remove buckets that haven't been seen for 2x the window duration
			if now.Sub(bucket.lastSeen) > l.window*2 {
				delete(l.buckets, key)
			}
		}

		l.mu.Unlock()
	}
}
