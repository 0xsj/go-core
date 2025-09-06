// internal/lib/monitoring/health/checkers_test.go
package health

import (
	"context"
	"runtime"
	"testing"
)

func TestMemoryChecker_Name(t *testing.T) {
	checker := NewMemoryChecker(512)
	if checker.Name() != "memory" {
		t.Errorf("Expected name 'memory', got %s", checker.Name())
	}
}

func TestMemoryChecker_Critical(t *testing.T) {
	checker := NewMemoryChecker(512)
	if !checker.Critical() {
		t.Error("Memory checker should be critical")
	}
}

func TestMemoryChecker_Check_Healthy(t *testing.T) {
	checker := NewMemoryChecker(1024) // Large limit for test

	result := checker.Check(context.Background())

	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy status with large limit, got %s", result.Status)
	}

	if result.Message != "Memory usage normal" {
		t.Errorf("Expected normal message, got %s", result.Message)
	}

	// Check details
	if result.Details["max_heap_mb"] != uint64(1024) {
		t.Error("Expected max_heap_mb in details")
	}
}

func TestGoroutineChecker_Name(t *testing.T) {
	checker := NewGoroutineChecker(1000)
	if checker.Name() != "goroutines" {
		t.Errorf("Expected name 'goroutines', got %s", checker.Name())
	}
}

func TestGoroutineChecker_Critical(t *testing.T) {
	checker := NewGoroutineChecker(1000)
	if !checker.Critical() {
		t.Error("Goroutine checker should be critical")
	}
}

func TestGoroutineChecker_Check_Healthy(t *testing.T) {
	checker := NewGoroutineChecker(1000)

	result := checker.Check(context.Background())

	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy status, got %s", result.Status)
	}

	// Verify details
	details := result.Details
	if details["current_count"] == nil {
		t.Error("Expected current_count in details")
	}

	currentCount := details["current_count"].(int)
	if currentCount <= 0 {
		t.Error("Expected positive goroutine count")
	}
}

func TestGoroutineChecker_Check_Unhealthy(t *testing.T) {
	currentGoroutines := runtime.NumGoroutine()
	checker := NewGoroutineChecker(currentGoroutines - 1) // Set limit below current

	result := checker.Check(context.Background())

	if result.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy status, got %s", result.Status)
	}
}

func TestUptimeChecker_Name(t *testing.T) {
	checker := NewUptimeChecker()
	if checker.Name() != "uptime" {
		t.Errorf("Expected name 'uptime', got %s", checker.Name())
	}
}

func TestUptimeChecker_Critical(t *testing.T) {
	checker := NewUptimeChecker()
	if checker.Critical() {
		t.Error("Uptime checker should not be critical")
	}
}

func TestDiskChecker_Name(t *testing.T) {
	checker := NewDiskChecker("/", 80, 95)
	if checker.Name() != "disk_space" {
		t.Errorf("Expected name 'disk_space', got %s", checker.Name())
	}
}

func TestDiskChecker_Critical(t *testing.T) {
	checker := NewDiskChecker("/", 80, 95)
	if !checker.Critical() {
		t.Error("Disk checker should be critical")
	}
}

func TestDiskChecker_Check_ValidPath(t *testing.T) {
	checker := NewDiskChecker("/", 99, 99.9) // High thresholds to avoid test failures

	result := checker.Check(context.Background())

	// Should not error on valid path
	if result.Error != nil {
		t.Errorf("Expected no error for valid path, got %v", result.Error)
	}

	// Check required details
	details := result.Details
	requiredFields := []string{"path", "total_bytes", "used_bytes", "available_bytes", "used_percent"}
	for _, field := range requiredFields {
		if details[field] == nil {
			t.Errorf("Expected %s in details", field)
		}
	}
}

func TestDiskChecker_Check_InvalidPath(t *testing.T) {
	checker := NewDiskChecker("/nonexistent/path", 80, 95)

	result := checker.Check(context.Background())

	if result.Status != StatusUnknown {
		t.Errorf("Expected unknown status for invalid path, got %s", result.Status)
	}

	if result.Error == nil {
		t.Error("Expected error for invalid path")
	}
}

func TestNewDiskChecker_Defaults(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		warn         float64
		crit         float64
		expectedPath string
		expectedWarn float64
		expectedCrit float64
	}{
		{"empty path", "", 80, 95, "/", 80, 95},
		{"zero warn", "/", 0, 95, "/", 80, 95},
		{"zero crit", "/", 80, 0, "/", 80, 95},
		{"all defaults", "", 0, 0, "/", 80, 95},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewDiskChecker(tt.path, tt.warn, tt.crit)

			if checker.path != tt.expectedPath {
				t.Errorf("Expected path %s, got %s", tt.expectedPath, checker.path)
			}
			if checker.warnThresholdPercent != tt.expectedWarn {
				t.Errorf("Expected warn threshold %f, got %f", tt.expectedWarn, checker.warnThresholdPercent)
			}
			if checker.critThresholdPercent != tt.expectedCrit {
				t.Errorf("Expected crit threshold %f, got %f", tt.expectedCrit, checker.critThresholdPercent)
			}
		})
	}
}
