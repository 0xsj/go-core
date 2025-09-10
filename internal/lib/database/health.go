// internal/lib/database/health.go (move from sqlx folder to database folder)
package database

import (
	"context"
	"time"

	"github.com/0xsj/go-core/internal/lib/monitoring/health"
)

// HealthChecker implements health.HealthChecker for database
type HealthChecker struct {
	db      Database
	timeout time.Duration
}

// NewHealthChecker creates a new database health checker
func NewHealthChecker(db Database, timeout time.Duration) *HealthChecker {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &HealthChecker{
		db:      db,
		timeout: timeout,
	}
}

func (h *HealthChecker) Name() string {
	return "database"
}

func (h *HealthChecker) Critical() bool {
	return true // Database is critical
}

func (h *HealthChecker) Check(ctx context.Context) health.HealthResult {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	start := time.Now()
	err := h.db.Ping(ctx)
	latency := time.Since(start)

	if err != nil {
		return health.HealthResult{
			Status:  health.StatusUnhealthy,
			Message: "Database connection failed",
			Error:   err,
			Latency: latency,
			Details: map[string]any{
				"driver": h.db.DriverName(),
				"error":  err.Error(),
			},
		}
	}

	stats := h.db.Stats()

	// Check for concerning patterns
	status := health.StatusHealthy
	message := "Database connection healthy"

	if stats.InUse >= stats.MaxOpenConnections-1 {
		status = health.StatusDegraded
		message = "Connection pool near exhaustion"
	} else if stats.WaitCount > 0 {
		status = health.StatusDegraded
		message = "Connections waiting in queue"
	}

	return health.HealthResult{
		Status:  status,
		Message: message,
		Latency: latency,
		Details: map[string]any{
			"driver":           h.db.DriverName(),
			"open_connections": stats.OpenConnections,
			"in_use":           stats.InUse,
			"idle":             stats.Idle,
			"wait_count":       stats.WaitCount,
			"max_connections":  stats.MaxOpenConnections,
		},
	}
}
