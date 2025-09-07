package health

import (
	"context"
	"time"
)

type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusDegraded  HealthStatus = "degraded"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusUnknown   HealthStatus = "unknown"
)

type HealthResult struct {
	Status    HealthStatus   `json:"status"`
	Message   string         `json:"message,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	Latency   time.Duration  `json:"latency"`
	Timestamp time.Time      `json:"timestamp"`
	Error     error          `json:"-"`
}

type HealthChecker interface {
	Name() string
	Check(ctx context.Context) HealthResult
	Critical() bool
}

type HealthManager interface {
	RegisterChecker(checker HealthChecker)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetOverallHealth() HealthResult
	GetDetailedHealth() map[string]HealthResult
	RunChecksOnce()
}
