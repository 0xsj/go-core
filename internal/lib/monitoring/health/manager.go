// internal/lib/monitoring/health/manager.go
package health

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/0xsj/go-core/internal/config"
	"github.com/0xsj/go-core/internal/lib/logger"
)

type manager struct {
	config   *config.HealthConfig
	logger   logger.Logger
	checkers map[string]HealthChecker
	results  map[string]HealthResult
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewManager(cfg *config.HealthConfig, appLogger logger.Logger) HealthManager {
	return &manager{
		config:   cfg,
		logger:   appLogger,
		checkers: make(map[string]HealthChecker),
		results:  make(map[string]HealthResult),
	}
}

func (m *manager) RegisterChecker(checker HealthChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkers[checker.Name()] = checker

	// Initialize with unknown status
	m.results[checker.Name()] = HealthResult{
		Status:    StatusUnknown,
		Message:   "Not yet checked",
		Timestamp: time.Now(),
	}

	m.logger.Info("Health checker registered",
		logger.String("checker", checker.Name()),
		logger.Bool("critical", checker.Critical()),
	)
}

func (m *manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		return fmt.Errorf("health manager already started")
	}

	m.ctx, m.cancel = context.WithCancel(ctx)

	if m.config.EnableBackground {
		m.wg.Add(1)
		go m.backgroundChecker()
	}

	m.logger.Info("Health manager started",
		logger.Bool("background_enabled", m.config.EnableBackground),
		logger.String("check_interval", m.config.CheckInterval),
	)

	return nil
}

func (m *manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel == nil {
		return nil // Not started
	}

	m.cancel()
	m.wg.Wait()
	m.cancel = nil

	m.logger.Info("Health manager stopped")
	return nil
}

func (m *manager) GetOverallHealth() HealthResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	overallStatus := StatusHealthy
	var unhealthyCheckers []string
	var degradedCheckers []string
	totalLatency := time.Duration(0)
	checkCount := 0

	for name, result := range m.results {
		checkCount++
		totalLatency += result.Latency

		switch result.Status {
		case StatusUnhealthy:
			unhealthyCheckers = append(unhealthyCheckers, name)
			checker := m.checkers[name]
			if checker.Critical() {
				overallStatus = StatusUnhealthy
			} else if overallStatus == StatusHealthy {
				overallStatus = StatusDegraded
			}
		case StatusDegraded:
			degradedCheckers = append(degradedCheckers, name)
			if overallStatus == StatusHealthy {
				overallStatus = StatusDegraded
			}
		case StatusUnknown:
			if overallStatus == StatusHealthy {
				overallStatus = StatusUnknown
			}
		}
	}

	message := "All systems operational"
	details := make(map[string]interface{})

	if len(unhealthyCheckers) > 0 {
		details["unhealthy_checkers"] = unhealthyCheckers
		if overallStatus == StatusUnhealthy {
			message = "Critical systems failing"
		} else {
			message = "Non-critical systems failing"
		}
	}

	if len(degradedCheckers) > 0 {
		details["degraded_checkers"] = degradedCheckers
		if message == "All systems operational" {
			message = "Some systems degraded"
		}
	}

	if checkCount > 0 {
		details["average_latency_ms"] = float64(totalLatency.Nanoseconds()) / float64(checkCount) / 1e6
	}

	details["total_checkers"] = len(m.checkers)
	details["last_check"] = time.Now()

	return HealthResult{
		Status:    overallStatus,
		Message:   message,
		Details:   details,
		Latency:   totalLatency / time.Duration(max(checkCount, 1)),
		Timestamp: time.Now(),
	}
}

func (m *manager) GetDetailedHealth() map[string]HealthResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]HealthResult, len(m.results))
	for name, healthResult := range m.results {
		result[name] = healthResult
	}
	return result
}

func (m *manager) backgroundChecker() {
	defer m.wg.Done()

	checkInterval, err := time.ParseDuration(m.config.CheckInterval)
	if err != nil {
		checkInterval = 30 * time.Second
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// Run initial check immediately
	m.runAllChecks()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.runAllChecks()
		}
	}
}

func (m *manager) runAllChecks() {
	m.mu.RLock()
	checkers := make(map[string]HealthChecker, len(m.checkers))
	for name, checker := range m.checkers {
		checkers[name] = checker
	}
	m.mu.RUnlock()

	checkTimeout, err := time.ParseDuration(m.config.CheckTimeout)
	if err != nil {
		checkTimeout = 5 * time.Second
	}

	var wg sync.WaitGroup
	results := make(chan struct {
		name   string
		result HealthResult
	}, len(checkers))

	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker HealthChecker) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(m.ctx, checkTimeout)
			defer cancel()

			start := time.Now()
			result := checker.Check(ctx)
			result.Latency = time.Since(start)
			result.Timestamp = time.Now()

			// Handle timeout
			if ctx.Err() == context.DeadlineExceeded {
				result = HealthResult{
					Status:    StatusUnknown,
					Message:   "Health check timed out",
					Latency:   result.Latency,
					Timestamp: time.Now(),
					Error:     ctx.Err(),
				}
			}

			results <- struct {
				name   string
				result HealthResult
			}{name, result}
		}(name, checker)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	m.mu.Lock()
	for result := range results {
		m.results[result.name] = result.result

		// Log failed checks
		if result.result.Status == StatusUnhealthy || result.result.Status == StatusUnknown {
			m.logger.Warn("Health check failed",
				logger.String("checker", result.name),
				logger.String("status", string(result.result.Status)),
				logger.String("message", result.result.Message),
				logger.Duration("latency", result.result.Latency),
			)
		}
	}
	m.mu.Unlock()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
