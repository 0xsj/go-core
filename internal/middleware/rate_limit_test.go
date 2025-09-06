// internal/middleware/rate_limit_test.go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRateLimitMiddleware_Name(t *testing.T) {
	mw := NewRateLimitMiddleware(nil)
	if mw.Name() != "rate_limit" {
		t.Errorf("Expected name 'rate_limit', got %s", mw.Name())
	}
}

func TestRateLimitMiddleware_Priority(t *testing.T) {
	mw := NewRateLimitMiddleware(nil)
	if mw.Priority() != PriorityRateLimit {
		t.Errorf("Expected priority %d, got %d", PriorityRateLimit, mw.Priority())
	}
}

func TestRateLimitMiddleware_AllowedRequests(t *testing.T) {
	config := &RateLimitConfig{
		Enabled:        true,
		RequestsPerMin: 60,
		BurstSize:      5,
		WindowSize:     time.Minute,
		KeyBy:          "ip",
		HeadersEnabled: true,
	}
	mw := NewRateLimitMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	// Test burst capacity
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		mw.Handler()(handler).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}

		// Check rate limit headers
		limit := w.Header().Get("X-RateLimit-Limit")
		if limit != "60" {
			t.Errorf("Expected rate limit header '60', got %s", limit)
		}

		remaining := w.Header().Get("X-RateLimit-Remaining")
		expectedRemaining := strconv.Itoa(4 - i)
		if remaining != expectedRemaining {
			t.Errorf("Expected remaining %s, got %s", expectedRemaining, remaining)
		}
	}
}

func TestRateLimitMiddleware_ExceedsLimit(t *testing.T) {
	config := &RateLimitConfig{
		Enabled:        true,
		RequestsPerMin: 60,
		BurstSize:      2,
		WindowSize:     time.Minute,
		KeyBy:          "ip",
		HeadersEnabled: true,
	}
	mw := NewRateLimitMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	// Use up burst capacity
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		mw.Handler()(handler).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	mw.Handler()(handler).ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Rate Limit Exceeded") {
		t.Error("Expected rate limit error message")
	}
}

func TestRateLimitMiddleware_DifferentIPs(t *testing.T) {
	config := &RateLimitConfig{
		Enabled:        true,
		RequestsPerMin: 60,
		BurstSize:      1,
		WindowSize:     time.Minute,
		KeyBy:          "ip",
		HeadersEnabled: true,
	}
	mw := NewRateLimitMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	// Different IPs should have separate limits
	ips := []string{"192.168.1.1:12345", "192.168.1.2:12345", "192.168.1.3:12345"}

	for _, ip := range ips {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()

		mw.Handler()(handler).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("IP %s: Expected status 200, got %d", ip, w.Code)
		}
	}
}

func TestRateLimitMiddleware_SkipPaths(t *testing.T) {
	config := &RateLimitConfig{
		Enabled:        true,
		RequestsPerMin: 60,
		BurstSize:      1,
		WindowSize:     time.Minute,
		KeyBy:          "ip",
		SkipPaths:      []string{"/health", "/metrics"},
		HeadersEnabled: true,
	}
	mw := NewRateLimitMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	// Use up limit on regular path
	req := httptest.NewRequest("GET", "/api", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	mw.Handler()(handler).ServeHTTP(w, req)

	// Should be rate limited on regular path
	req = httptest.NewRequest("GET", "/api", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w = httptest.NewRecorder()
	mw.Handler()(handler).ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected rate limit on /api, got %d", w.Code)
	}

	// But health check should still work
	req = httptest.NewRequest("GET", "/health", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w = httptest.NewRecorder()
	mw.Handler()(handler).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected /health to be allowed, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_UserKeyStrategy(t *testing.T) {
	config := &RateLimitConfig{
		Enabled:        true,
		RequestsPerMin: 60,
		BurstSize:      2,
		WindowSize:     time.Minute,
		KeyBy:          "user",
		HeadersEnabled: true,
	}
	mw := NewRateLimitMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	// Same IP, different users should have separate limits
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	req1.Header.Set("X-User-ID", "user1")

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	req2.Header.Set("X-User-ID", "user2")

	// Use up user1's limit
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		mw.Handler()(handler).ServeHTTP(w, req1)
		if w.Code != http.StatusOK {
			t.Errorf("User1 request %d: Expected 200, got %d", i+1, w.Code)
		}
	}

	// User1 should be rate limited
	w := httptest.NewRecorder()
	mw.Handler()(handler).ServeHTTP(w, req1)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected user1 to be rate limited, got %d", w.Code)
	}

	// User2 should still be allowed
	w = httptest.NewRecorder()
	mw.Handler()(handler).ServeHTTP(w, req2)
	if w.Code != http.StatusOK {
		t.Errorf("Expected user2 to be allowed, got %d", w.Code)
	}
}

func TestTokenBucketLimiter_Allow(t *testing.T) {
	limiter := NewTokenBucketLimiter(60, 5, time.Minute)

	// Should allow initial burst
	for i := 0; i < 5; i++ {
		allowed, remaining, _ := limiter.Allow("test-key")
		if !allowed {
			t.Errorf("Request %d should be allowed", i+1)
		}
		expectedRemaining := 4 - i
		if remaining != expectedRemaining {
			t.Errorf("Expected %d remaining, got %d", expectedRemaining, remaining)
		}
	}

	// Should reject next request
	allowed, remaining, _ := limiter.Allow("test-key")
	if allowed {
		t.Error("Request should be rejected after burst")
	}
	if remaining != 0 {
		t.Errorf("Expected 0 remaining, got %d", remaining)
	}
}

func TestTokenBucketLimiter_DifferentKeys(t *testing.T) {
	limiter := NewTokenBucketLimiter(60, 2, time.Minute)

	// Use up key1's tokens
	limiter.Allow("key1")
	limiter.Allow("key1")

	// key1 should be exhausted
	allowed, _, _ := limiter.Allow("key1")
	if allowed {
		t.Error("key1 should be exhausted")
	}

	// key2 should still have tokens
	allowed, _, _ = limiter.Allow("key2")
	if !allowed {
		t.Error("key2 should have tokens")
	}
}
