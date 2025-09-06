// internal/middleware/provider.go
package middleware

import (
	"sync"

	"github.com/0xsj/go-core/internal/config"
	"github.com/0xsj/go-core/internal/container"
	"github.com/0xsj/go-core/internal/lib/logger"
)

// Provider creates and manages middleware instances
type Provider struct {
	Container *container.Container
	chain     *Chain
	once      sync.Once
}

// NewProvider creates a new middleware provider
func NewProvider(c *container.Container) *Provider {
	return &Provider{
		Container: c,
	}
}

// Provide returns the configured middleware chain
func (p *Provider) Provide() *Chain {
	p.once.Do(func() {
		// Resolve dependencies from container
		cfg := container.Resolve[*config.Config](p.Container)
		appLogger := container.Resolve[logger.Logger](p.Container)

		// Create middleware instances
		requestIDMw := NewRequestIDMiddleware(&RequestIDConfig{
			Enabled:    cfg.Middleware.RequestID.Enabled,
			HeaderName: cfg.Middleware.RequestID.HeaderName,
			Generate:   cfg.Middleware.RequestID.Generate,
		})

		recoveryMw := NewRecoveryMiddleware(&RecoveryConfig{
			Enabled:           cfg.Middleware.Recovery.Enabled,
			LogStackTrace:     cfg.Middleware.Recovery.LogStackTrace,
			IncludeStackInDev: cfg.Middleware.Recovery.IncludeStackInDev,
		}, appLogger, cfg.App.IsDevelopment())

		loggingMw := NewLoggingMiddleware(&LoggingConfig{
			Enabled:         cfg.Middleware.Logging.Enabled,
			LogRequests:     cfg.Middleware.Logging.LogRequests,
			LogResponses:    cfg.Middleware.Logging.LogResponses,
			LogHeaders:      cfg.Middleware.Logging.LogHeaders,
			LogBody:         cfg.Middleware.Logging.LogBody,
			MaxBodySize:     cfg.Middleware.Logging.MaxBodySize,
			SkipPaths:       cfg.Middleware.Logging.SkipPaths,
			SlowRequestTime: cfg.Middleware.Logging.SlowRequestTime,
		}, appLogger)

		// Create middleware chain with automatic priority ordering
		p.chain = NewChain(requestIDMw, recoveryMw, loggingMw)
	})

	return p.chain
}
