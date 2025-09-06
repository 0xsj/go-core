// internal/lib/monitoring/health/manager_test.go
package health

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/0xsj/go-core/internal/config"
	"github.com/0xsj/go-core/internal/lib/logger"
)

// Mock health checker for testing
type mockChecker struct {
	name      string
	critical  bool
	result    HealthResult
	callCount int
	mu        sync.Mutex
}

func (m *mockChecker) Name() string {
	return m.name
}

func (m *mockChecker) Critical() bool {
	return m.critical
}

func (m *mockChecker) Check(ctx context.Context) HealthResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.result
}

func (m *mockChecker) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func createTestLogger() logger.Logger {
	config := &logger.LoggerConfig{
		Level:      logger.LevelError, // Reduce noise in tests
		Output:     nil,               // Discard output
		Format:     logger.FormatPretty,
		ShowCaller: false,
		ShowColor:  false,
	}
	return logger.NewLogger(config)
}

func createTestConfig() *config.HealthConfig {
	return &config.HealthConfig{
		CheckInterval:    "100ms", // Fast for tests
		CheckTimeout:     "50ms",
		EnableBackground: true,
	}
}

func TestNewManager(t *testing.T) {
	cfg := createTestConfig()
	testLogger := createTestLogger()

	manager := NewManager(cfg, testLogger)

	if manager == nil {
		t.Error("Expected manager to be created")
	}
}

func TestManager_RegisterChecker(t *testing.T) {
	cfg := createTestConfig()
	testLogger := createTestLogger()
	manager := NewManager(cfg, testLogger)

	checker := &mockChecker{
		name:     "test",
		critical: true,
		result:   HealthResult{Status: StatusHealthy},
	}

	manager.RegisterChecker(checker)

	// Verify checker was registered
	detailed := manager.GetDetailedHealth()
	if len(detailed) != 1 {
		t.Errorf("Expected 1 checker, got %d", len(detailed))
	}

	if _, exists := detailed["test"]; !exists {
		t.Error("Expected test checker in detailed health")
	}
}

func TestManager_GetOverallHealth_Healthy(t *testing.T) {
	cfg := createTestConfig()
	testLogger := createTestLogger()
	manager := NewManager(cfg, testLogger)

	// Add healthy checkers
	manager.RegisterChecker(&mockChecker{
		name:     "healthy1",
		critical: true,
		result:   HealthResult{Status: StatusHealthy, Message: "OK"},
	})

	manager.RegisterChecker(&mockChecker{
		name:     "healthy2",
		critical: false,
		result:   HealthResult{Status: StatusHealthy, Message: "OK"},
	})

	// Start manager to trigger initial checks
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Wait for first check cycle
	time.Sleep(150 * time.Millisecond)

	overall := manager.GetOverallHealth()

	// Stop manager
	err = manager.Stop(context.Background())
	if err != nil {
		t.Errorf("Failed to stop manager: %v", err)
	}

	if overall.Status != StatusHealthy {
		t.Errorf("Expected healthy status, got %s", overall.Status)
	}

	if overall.Message != "All systems operational" {
		t.Errorf("Expected 'All systems operational', got %s", overall.Message)
	}
}

func TestManager_GetOverallHealth_CriticalFailure(t *testing.T) {
	cfg := createTestConfig()
	testLogger := createTestLogger()
	manager := NewManager(cfg, testLogger)

	// Add critical failing checker
	manager.RegisterChecker(&mockChecker{
		name:     "critical_fail",
		critical: true,
		result:   HealthResult{Status: StatusUnhealthy, Message: "Critical error"},
	})

	// Add healthy checker
	manager.RegisterChecker(&mockChecker{
		name:     "healthy",
		critical: false,
		result:   HealthResult{Status: StatusHealthy, Message: "OK"},
	})

	// Start manager to trigger checks
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Wait for first check cycle
	time.Sleep(150 * time.Millisecond)

	overall := manager.GetOverallHealth()

	// Stop manager
	err = manager.Stop(context.Background())
	if err != nil {
		t.Errorf("Failed to stop manager: %v", err)
	}

	if overall.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy status, got %s", overall.Status)
	}

	if overall.Message != "Critical systems failing" {
		t.Errorf("Expected 'Critical systems failing', got %s", overall.Message)
	}

	// Check details safely
	if unhealthyCheckers, exists := overall.Details["unhealthy_checkers"]; exists {
		checkers := unhealthyCheckers.([]string)
		if len(checkers) != 1 || checkers[0] != "critical_fail" {
			t.Errorf("Expected ['critical_fail'], got %v", checkers)
		}
	} else {
		t.Error("Expected unhealthy_checkers in details")
	}
}

func TestManager_StartStop(t *testing.T) {
	cfg := &config.HealthConfig{
		CheckInterval:    "1s",
		CheckTimeout:     "500ms",
		EnableBackground: false, // Disable background to avoid hanging
	}
	testLogger := createTestLogger()
	manager := NewManager(cfg, testLogger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Test start
	err := manager.Start(ctx)
	if err != nil {
		t.Errorf("Expected no error starting manager, got %v", err)
	}

	// Test double start
	err = manager.Start(ctx)
	if err == nil {
		t.Error("Expected error on double start")
	}

	// Test stop with timeout
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer stopCancel()

	err = manager.Stop(stopCtx)
	if err != nil {
		t.Errorf("Expected no error stopping manager, got %v", err)
	}

	// Test double stop
	err = manager.Stop(context.Background())
	if err != nil {
		t.Errorf("Expected no error on double stop, got %v", err)
	}
}

func TestManager_BackgroundChecking(t *testing.T) {
	cfg := &config.HealthConfig{
		CheckInterval:    "50ms", // Very fast for test
		CheckTimeout:     "25ms",
		EnableBackground: true,
	}
	testLogger := createTestLogger()
	manager := NewManager(cfg, testLogger)

	checker := &mockChecker{
		name:     "background_test",
		critical: true,
		result:   HealthResult{Status: StatusHealthy, Message: "OK"},
	}

	manager.RegisterChecker(checker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background checking
	err := manager.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Wait for multiple check cycles
	time.Sleep(200 * time.Millisecond)

	// Stop the manager
	err = manager.Stop(context.Background())
	if err != nil {
		t.Fatalf("Failed to stop manager: %v", err)
	}

	// Verify checker was called multiple times
	callCount := checker.GetCallCount()
	if callCount < 2 {
		t.Errorf("Expected at least 2 background checks, got %d", callCount)
	}
}

func TestManager_BackgroundDisabled(t *testing.T) {
	cfg := &config.HealthConfig{
		CheckInterval:    "50ms",
		CheckTimeout:     "25ms",
		EnableBackground: false, // Disabled
	}
	testLogger := createTestLogger()
	manager := NewManager(cfg, testLogger)

	checker := &mockChecker{
		name:     "no_background_test",
		critical: true,
		result:   HealthResult{Status: StatusHealthy, Message: "OK"},
	}

	manager.RegisterChecker(checker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start manager
	err := manager.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Wait
	time.Sleep(150 * time.Millisecond)

	// Stop the manager
	err = manager.Stop(context.Background())
	if err != nil {
		t.Fatalf("Failed to stop manager: %v", err)
	}

	// Verify checker was not called by background process
	callCount := checker.GetCallCount()
	if callCount > 0 {
		t.Errorf("Expected 0 background checks when disabled, got %d", callCount)
	}
}
