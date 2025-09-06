// internal/middleware/logging.go
package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/0xsj/go-core/internal/lib/logger"
)

// LoggingConfig holds configuration for request logging middleware
type LoggingConfig struct {
	Enabled         bool     `json:"enabled" env:"MIDDLEWARE_LOGGING_ENABLED" default:"true"`
	LogRequests     bool     `json:"log_requests" env:"MIDDLEWARE_LOGGING_REQUESTS" default:"true"`
	LogResponses    bool     `json:"log_responses" env:"MIDDLEWARE_LOGGING_RESPONSES" default:"true"`
	LogHeaders      bool     `json:"log_headers" env:"MIDDLEWARE_LOGGING_HEADERS" default:"false"`
	LogBody         bool     `json:"log_body" env:"MIDDLEWARE_LOGGING_BODY" default:"false"`
	MaxBodySize     int      `json:"max_body_size" env:"MIDDLEWARE_LOGGING_MAX_BODY_SIZE" default:"1024"`
	SkipPaths       []string `json:"skip_paths" env:"MIDDLEWARE_LOGGING_SKIP_PATHS" default:"/health,/metrics"`
	SlowRequestTime string   `json:"slow_request_time" env:"MIDDLEWARE_LOGGING_SLOW_REQUEST" default:"2s"`
}

// LoggingMiddleware provides structured request/response logging
type LoggingMiddleware struct {
	config          *LoggingConfig
	logger          logger.Logger
	slowRequestTime time.Duration
	skipPathsMap    map[string]bool
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(config *LoggingConfig, appLogger logger.Logger) *LoggingMiddleware {
	if config == nil {
		config = &LoggingConfig{
			Enabled:         true,
			LogRequests:     true,
			LogResponses:    true,
			LogHeaders:      false,
			LogBody:         false,
			MaxBodySize:     1024,
			SkipPaths:       []string{"/health", "/metrics"},
			SlowRequestTime: "2s",
		}
	}

	slowRequestTime, err := time.ParseDuration(config.SlowRequestTime)
	if err != nil {
		slowRequestTime = 2 * time.Second
	}

	// Convert skip paths to map for O(1) lookup
	skipPathsMap := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipPathsMap[strings.TrimSpace(path)] = true
	}

	return &LoggingMiddleware{
		config:          config,
		logger:          appLogger,
		slowRequestTime: slowRequestTime,
		skipPathsMap:    skipPathsMap,
	}
}

// Name returns the middleware name
func (m *LoggingMiddleware) Name() string {
	return "logging"
}

// Priority returns the middleware priority
func (m *LoggingMiddleware) Priority() int {
	return PriorityLogging
}

// Enabled checks if middleware should be applied
func (m *LoggingMiddleware) Enabled(ctx context.Context) bool {
	return m.config.Enabled
}

// Handler returns the HTTP middleware handler
func (m *LoggingMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip logging for configured paths
			if m.skipPathsMap[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Add request start time to context
			ctx := context.WithValue(r.Context(), ContextKeyRequestStart, start)
			r = r.WithContext(ctx)

			// Log incoming request
			if m.config.LogRequests {
				m.logRequest(r)
			}

			// Wrap response writer to capture response data
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				responseSize:   0,
			}

			// Process request
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			// Log response
			if m.config.LogResponses {
				m.logResponse(r, wrapped, duration)
			}

			// Log slow requests with warning level
			if duration > m.slowRequestTime {
				m.logSlowRequest(r, wrapped, duration)
			}
		})
	}
}

// logRequest logs the incoming HTTP request
func (m *LoggingMiddleware) logRequest(r *http.Request) {
	fields := []logger.Field{
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path),
		logger.String("remote_addr", r.RemoteAddr),
		logger.String("user_agent", r.UserAgent()),
	}

	if r.URL.RawQuery != "" {
		fields = append(fields, logger.String("query", r.URL.RawQuery))
	}

	if m.config.LogHeaders && len(r.Header) > 0 {
		fields = append(fields, logger.Any("headers", m.sanitizeHeaders(r.Header)))
	}

	contextLogger := m.logger.WithContext(r.Context())
	contextLogger.Info("HTTP request received", fields...)
}

// logResponse logs the HTTP response
func (m *LoggingMiddleware) logResponse(r *http.Request, w *responseWriter, duration time.Duration) {
	fields := []logger.Field{
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path),
		logger.Int("status_code", w.statusCode),
		logger.Int("response_size", int(w.responseSize)),
		logger.Duration("duration", duration),
		logger.String("duration_ms", strconv.FormatFloat(float64(duration.Nanoseconds())/1e6, 'f', 2, 64)),
	}

	contextLogger := m.logger.WithContext(r.Context())

	// Log level based on status code
	switch {
	case w.statusCode >= 500:
		contextLogger.Error("HTTP request completed with server error", fields...)
	case w.statusCode >= 400:
		contextLogger.Warn("HTTP request completed with client error", fields...)
	default:
		contextLogger.Info("HTTP request completed", fields...)
	}
}

// logSlowRequest logs requests that exceed the slow request threshold
func (m *LoggingMiddleware) logSlowRequest(r *http.Request, w *responseWriter, duration time.Duration) {
	fields := []logger.Field{
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path),
		logger.Int("status_code", w.statusCode),
		logger.Duration("duration", duration),
		logger.Duration("threshold", m.slowRequestTime),
		logger.String("remote_addr", r.RemoteAddr),
	}

	contextLogger := m.logger.WithContext(r.Context())
	contextLogger.Warn("Slow HTTP request detected", fields...)
}

// sanitizeHeaders removes sensitive headers from logging
func (m *LoggingMiddleware) sanitizeHeaders(headers http.Header) map[string][]string {
	sanitized := make(map[string][]string)
	sensitiveHeaders := map[string]bool{
		"authorization": true,
		"cookie":        true,
		"set-cookie":    true,
		"x-api-key":     true,
		"x-auth-token":  true,
	}

	for name, values := range headers {
		lowerName := strings.ToLower(name)
		if sensitiveHeaders[lowerName] {
			sanitized[name] = []string{"[REDACTED]"}
		} else {
			sanitized[name] = values
		}
	}

	return sanitized
}

// responseWriter wraps http.ResponseWriter to capture response metadata
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	responseSize int64
}

// WriteHeader captures the status code
func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response size
func (w *responseWriter) Write(data []byte) (int, error) {
	size, err := w.ResponseWriter.Write(data)
	w.responseSize += int64(size)
	return size, err
}
