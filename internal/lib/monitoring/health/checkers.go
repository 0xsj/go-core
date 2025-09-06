// internal/lib/monitoring/health/checkers.go
package health

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// MemoryChecker monitors system memory usage
type MemoryChecker struct {
	maxHeapMB     uint64
	warnThreshold float64 // 0.8 = 80%
}

type DiskUsage struct {
	Total     uint64
	Used      uint64
	Available uint64
}

type DiskChecker struct {
	path                 string
	warnThresholdPercent float64
	critThresholdPercent float64
}

func (d *DiskChecker) getDiskUsage(path string) (*DiskUsage, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk stats for %s: %w", path, err)
	}

	blockSize := uint64(stat.Bsize)
	availableBlocks := stat.Bavail
	usedBlocks := stat.Blocks - stat.Bfree
	
	total := (availableBlocks + usedBlocks) * blockSize
	used := usedBlocks * blockSize
	available := availableBlocks * blockSize

	return &DiskUsage{
		Total:     total,
		Used:      used,
		Available: available,
	}, nil
}

func NewMemoryChecker(maxHeapMB uint64) *MemoryChecker {
	if maxHeapMB == 0 {
		maxHeapMB = 512 // Default 512MB
	}
	return &MemoryChecker{
		maxHeapMB:     maxHeapMB,
		warnThreshold: 0.8,
	}
}

func (m *MemoryChecker) Name() string {
	return "memory"
}

func (m *MemoryChecker) Critical() bool {
	return true // Memory exhaustion is critical
}

func (m *MemoryChecker) Check(ctx context.Context) HealthResult {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	heapMB := float64(memStats.HeapInuse) / 1024 / 1024
	maxMB := float64(m.maxHeapMB)
	usagePercent := heapMB / maxMB

	details := map[string]interface{}{
		"heap_mb":       fmt.Sprintf("%.2f", heapMB),
		"max_heap_mb":   m.maxHeapMB,
		"usage_percent": fmt.Sprintf("%.1f%%", usagePercent*100),
		"num_gc":        memStats.NumGC,
		"goroutines":    runtime.NumGoroutine(),
	}

	var status HealthStatus
	var message string

	switch {
	case usagePercent >= 0.95:
		status = StatusUnhealthy
		message = "Memory usage critical"
	case usagePercent >= m.warnThreshold:
		status = StatusDegraded
		message = "Memory usage high"
	default:
		status = StatusHealthy
		message = "Memory usage normal"
	}

	return HealthResult{
		Status:  status,
		Message: message,
		Details: details,
	}
}

// GoroutineChecker monitors goroutine count
type GoroutineChecker struct {
	maxGoroutines int
	warnThreshold int
}

func NewGoroutineChecker(maxGoroutines int) *GoroutineChecker {
	if maxGoroutines == 0 {
		maxGoroutines = 1000 // Default limit
	}
	warnThreshold := int(float64(maxGoroutines) * 0.8) // 80% of max

	return &GoroutineChecker{
		maxGoroutines: maxGoroutines,
		warnThreshold: warnThreshold,
	}
}

func (g *GoroutineChecker) Name() string {
	return "goroutines"
}

func (g *GoroutineChecker) Critical() bool {
	return true // Goroutine leaks can crash the app
}

func (g *GoroutineChecker) Check(ctx context.Context) HealthResult {
	count := runtime.NumGoroutine()

	details := map[string]interface{}{
		"current_count":  count,
		"max_allowed":    g.maxGoroutines,
		"warn_threshold": g.warnThreshold,
	}

	var status HealthStatus
	var message string

	switch {
	case count >= g.maxGoroutines:
		status = StatusUnhealthy
		message = "Too many goroutines"
	case count >= g.warnThreshold:
		status = StatusDegraded
		message = "High goroutine count"
	default:
		status = StatusHealthy
		message = "Goroutine count normal"
	}

	return HealthResult{
		Status:  status,
		Message: message,
		Details: details,
	}
}

// UptimeChecker tracks application uptime
type UptimeChecker struct {
	startTime time.Time
	once      sync.Once
}

func NewUptimeChecker() *UptimeChecker {
	return &UptimeChecker{}
}

func (u *UptimeChecker) Name() string {
	return "uptime"
}

func (u *UptimeChecker) Critical() bool {
	return false // Uptime is informational, not critical
}

func (u *UptimeChecker) Check(ctx context.Context) HealthResult {
	u.once.Do(func() {
		u.startTime = time.Now()
	})

	uptime := time.Since(u.startTime)

	details := map[string]interface{}{
		"uptime_seconds": int64(uptime.Seconds()),
		"uptime_human":   uptime.String(),
		"started_at":     u.startTime.Format(time.RFC3339),
	}

	return HealthResult{
		Status:  StatusHealthy,
		Message: fmt.Sprintf("Application running for %s", uptime.String()),
		Details: details,
	}
}

func NewDiskChecker(path string, warnPercent, critPercent float64) *DiskChecker {
	if path == "" {
		path = "/"
	}
	if warnPercent <= 0 {
		warnPercent = 80
	}
	if critPercent <= 0 {
		critPercent = 95
	}

	return &DiskChecker{
		path:                 path,
		warnThresholdPercent: warnPercent,
		critThresholdPercent: critPercent,
	}
}

func (d *DiskChecker) Name() string {
	return "disk_space"
}

func (d *DiskChecker) Critical() bool {
	return true // Disk space is critical
}

func (d *DiskChecker) Check(ctx context.Context) HealthResult {
	usage, err := d.getDiskUsage(d.path)
	if err != nil {
		return HealthResult{
			Status:  StatusUnknown,
			Message: fmt.Sprintf("Failed to check disk usage: %v", err),
			Error:   err,
		}
	}

	usedPercent := (float64(usage.Used) / float64(usage.Total)) * 100

	details := map[string]interface{}{
		"path":            d.path,
		"total_bytes":     usage.Total,
		"used_bytes":      usage.Used,
		"available_bytes": usage.Available,
		"used_percent":    fmt.Sprintf("%.1f%%", usedPercent),
		"total_gb":        fmt.Sprintf("%.2f", float64(usage.Total)/1024/1024/1024),
		"available_gb":    fmt.Sprintf("%.2f", float64(usage.Available)/1024/1024/1024),
	}

	var status HealthStatus
	var message string

	switch {
	case usedPercent >= d.critThresholdPercent:
		status = StatusUnhealthy
		message = fmt.Sprintf("Disk space critical: %.1f%% used", usedPercent)
	case usedPercent >= d.warnThresholdPercent:
		status = StatusDegraded
		message = fmt.Sprintf("Disk space high: %.1f%% used", usedPercent)
	default:
		status = StatusHealthy
		message = fmt.Sprintf("Disk space normal: %.1f%% used", usedPercent)
	}

	return HealthResult{
		Status:  status,
		Message: message,
		Details: details,
	}
}
