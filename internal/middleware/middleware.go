// internal/middleware/middleware.go
package middleware

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/0xsj/go-core/internal/lib/logger"
)

// Use logger's context keys directly for correlation
const (
	ContextKeyRequestID     = logger.ContextKeyRequestID
	ContextKeyCorrelationID = logger.ContextKeyCorrelationID
	ContextKeyTraceID       = logger.ContextKeyTraceID
	ContextKeyUserID        = logger.ContextKeyUserID
	ContextKeySessionID     = logger.ContextKeySessionID
)

// Additional context keys specific to middleware
type contextKey string

const (
	ContextKeyRequestStart contextKey = "request_start"
	ContextKeyRemoteIP     contextKey = "remote_ip"
	ContextKeyUserAgent    contextKey = "user_agent"
)

// Priority constants for middleware ordering
const (
	PriorityRecovery    = 10  // Panic recovery (first)
	PriorityRequestID   = 20  // Request ID generation
	PrioritySecurity    = 30  // Security headers
	PriorityCORS        = 40  // CORS handling
	PriorityRateLimit   = 50  // Rate limiting
	PriorityAuth        = 60  // Authentication
	PriorityLogging     = 70  // Request logging
	PriorityMetrics     = 80  // Metrics collection
	PriorityUserDefined = 100 // User-defined middleware
)

// Middleware represents a HTTP middleware component
type Middleware interface {
	Name() string
	Enabled(ctx context.Context) bool
	Handler() func(http.Handler) http.Handler
	Priority() int
}

// MiddlewareFunc is the standard Go middleware function type
type MiddlewareFunc func(http.Handler) http.Handler

// RequestMetadata contains correlation and timing information
type RequestMetadata struct {
	RequestID     string
	CorrelationID string
	TraceID       string
	UserID        string
	SessionID     string
	StartTime     time.Time
	RemoteIP      string
	UserAgent     string
	Method        string
	Path          string
	Query         string
}

// ResponseMetadata contains response information for logging
type ResponseMetadata struct {
	StatusCode    int
	ContentLength int64
	Duration      time.Duration
	Error         error
}

// Chain manages an ordered collection of middleware
type Chain struct {
	middleware []Middleware
}

// NewChain creates a new middleware chain
func NewChain(middleware ...Middleware) *Chain {
	chain := &Chain{
		middleware: make([]Middleware, len(middleware)),
	}
	copy(chain.middleware, middleware)

	// Sort by priority
	sort.Slice(chain.middleware, func(i, j int) bool {
		return chain.middleware[i].Priority() < chain.middleware[j].Priority()
	})

	return chain
}

// Add appends middleware to the chain and re-sorts
func (c *Chain) Add(middleware ...Middleware) *Chain {
	c.middleware = append(c.middleware, middleware...)

	// Re-sort by priority
	sort.Slice(c.middleware, func(i, j int) bool {
		return c.middleware[i].Priority() < c.middleware[j].Priority()
	})

	return c
}

// Handler builds the final HTTP handler with all middleware applied
func (c *Chain) Handler(final http.Handler) http.Handler {
	if final == nil {
		final = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
	}

	// Apply middleware in reverse order (last middleware wraps first)
	for i := len(c.middleware) - 1; i >= 0; i-- {
		mw := c.middleware[i]
		final = c.conditionalHandler(mw, final)
	}

	return final
}

// conditionalHandler wraps middleware with enabled check
func (c *Chain) conditionalHandler(mw Middleware, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if mw.Enabled(r.Context()) {
			mw.Handler()(next).ServeHTTP(w, r)
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

// List returns all middleware in priority order
func (c *Chain) List() []Middleware {
	result := make([]Middleware, len(c.middleware))
	copy(result, c.middleware)
	return result
}
