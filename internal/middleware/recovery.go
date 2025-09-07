// internal/middleware/recovery.go
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/0xsj/go-core/internal/lib/logger"
)

// RecoveryConfig holds configuration for panic recovery middleware
type RecoveryConfig struct {
	Enabled           bool `json:"enabled" env:"MIDDLEWARE_RECOVERY_ENABLED" default:"true"`
	LogStackTrace     bool `json:"log_stack_trace" env:"MIDDLEWARE_RECOVERY_STACK_TRACE" default:"true"`
	IncludeStackInDev bool `json:"include_stack_in_dev" env:"MIDDLEWARE_RECOVERY_DEV_STACK" default:"true"`
}

// RecoveryMiddleware handles panic recovery with structured logging
type RecoveryMiddleware struct {
	config *RecoveryConfig
	logger logger.Logger
	isDev  bool
}

// NewRecoveryMiddleware creates a new recovery middleware
func NewRecoveryMiddleware(config *RecoveryConfig, appLogger logger.Logger, isDev bool) *RecoveryMiddleware {
	if config == nil {
		config = &RecoveryConfig{
			Enabled:           true,
			LogStackTrace:     true,
			IncludeStackInDev: true,
		}
	}

	return &RecoveryMiddleware{
		config: config,
		logger: appLogger,
		isDev:  isDev,
	}
}

// Name returns the middleware name
func (m *RecoveryMiddleware) Name() string {
	return "recovery"
}

// Priority returns the middleware priority (highest - should be first)
func (m *RecoveryMiddleware) Priority() int {
	return PriorityRecovery
}

// Enabled checks if middleware should be applied
func (m *RecoveryMiddleware) Enabled(ctx context.Context) bool {
	return m.config.Enabled
}

// Handler returns the HTTP middleware handler
func (m *RecoveryMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					m.handlePanic(w, r, err)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// handlePanic processes recovered panics and sends appropriate responses
func (m *RecoveryMiddleware) handlePanic(w http.ResponseWriter, r *http.Request, err any) {
	// Log the panic with context
	fields := []logger.Field{
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path),
		logger.String("remote_addr", r.RemoteAddr),
		logger.String("user_agent", r.UserAgent()),
		logger.Any("panic", err),
	}

	// Add stack trace if configured
	if m.config.LogStackTrace {
		fields = append(fields, logger.String("stack_trace", string(debug.Stack())))
	}

	contextLogger := m.logger.WithContext(r.Context())
	contextLogger.Error("Panic recovered in HTTP handler", fields...)

	// Send response based on environment
	m.sendErrorResponse(w, r, err)
}

func (m *RecoveryMiddleware) sendErrorResponse(w http.ResponseWriter, r *http.Request, err any) {
	// Set content type before writing status
	w.Header().Set("Content-Type", "application/json")

	// Don't write header if already written
	if w.Header().Get("Content-Length") != "" {
		return
	}

	w.WriteHeader(http.StatusInternalServerError)

	var response string
	if m.isDev && m.config.IncludeStackInDev {
		// Development response with more details
		response = fmt.Sprintf(`{
	"error": "Internal Server Error",
	"message": "A panic occurred while processing the request",
	"details": "%v",
	"stack_trace": "%s"
}`, err, string(debug.Stack()))
	} else {
		// Production response - minimal information
		response = `{
	"error": "Internal Server Error", 
	"message": "An unexpected error occurred"
}`
	}

	if _, writeErr := w.Write([]byte(response)); writeErr != nil {
		// Log write error but don't panic - we're already in recovery
		contextLogger := m.logger.WithContext(r.Context())
		contextLogger.Error("Failed to write panic recovery response", logger.Err(writeErr))
	}
}
