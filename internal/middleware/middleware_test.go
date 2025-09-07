// internal/middleware/middleware_test.go
package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Mock middleware for testing
type mockMiddleware struct {
	name     string
	priority int
	enabled  bool
	calls    int
}

func (m *mockMiddleware) Name() string {
	return m.name
}

func (m *mockMiddleware) Priority() int {
	return m.priority
}

func (m *mockMiddleware) Enabled(ctx context.Context) bool {
	return m.enabled
}

func (m *mockMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.calls++
			w.Header().Set("X-"+m.name, "called")
			next.ServeHTTP(w, r)
		})
	}
}

func TestNewChain(t *testing.T) {
	mw1 := &mockMiddleware{name: "first", priority: 20, enabled: true}
	mw2 := &mockMiddleware{name: "second", priority: 10, enabled: true}

	chain := NewChain(mw1, mw2)

	if len(chain.middleware) != 2 {
		t.Errorf("Expected 2 middleware, got %d", len(chain.middleware))
	}

	// Should be sorted by priority (lower first)
	if chain.middleware[0].Priority() != 10 {
		t.Errorf("Expected first middleware to have priority 10, got %d", chain.middleware[0].Priority())
	}

	if chain.middleware[1].Priority() != 20 {
		t.Errorf("Expected second middleware to have priority 20, got %d", chain.middleware[1].Priority())
	}
}

func TestChain_Handler(t *testing.T) {
	mw1 := &mockMiddleware{name: "First", priority: 10, enabled: true}
	mw2 := &mockMiddleware{name: "Second", priority: 20, enabled: true}

	chain := NewChain(mw1, mw2)

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("final")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	handler := chain.Handler(finalHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Both middleware should be called
	if mw1.calls != 1 {
		t.Errorf("Expected mw1 to be called once, got %d", mw1.calls)
	}

	if mw2.calls != 1 {
		t.Errorf("Expected mw2 to be called once, got %d", mw2.calls)
	}

	// Headers should be set
	if w.Header().Get("X-First") != "called" {
		t.Error("Expected X-First header to be set")
	}

	if w.Header().Get("X-Second") != "called" {
		t.Error("Expected X-Second header to be set")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "final" {
		t.Errorf("Expected 'final', got %s", w.Body.String())
	}
}

func TestChain_DisabledMiddleware(t *testing.T) {
	mw1 := &mockMiddleware{name: "Enabled", priority: 10, enabled: true}
	mw2 := &mockMiddleware{name: "Disabled", priority: 20, enabled: false}

	chain := NewChain(mw1, mw2)

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := chain.Handler(finalHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Only enabled middleware should be called
	if mw1.calls != 1 {
		t.Errorf("Expected enabled middleware to be called once, got %d", mw1.calls)
	}

	if mw2.calls != 0 {
		t.Errorf("Expected disabled middleware not to be called, got %d", mw2.calls)
	}

	// Only enabled middleware header should be set
	if w.Header().Get("X-Enabled") != "called" {
		t.Error("Expected X-Enabled header to be set")
	}

	if w.Header().Get("X-Disabled") != "" {
		t.Error("Expected X-Disabled header not to be set")
	}
}

func TestChain_Add(t *testing.T) {
	mw1 := &mockMiddleware{name: "first", priority: 20, enabled: true}
	chain := NewChain(mw1)

	mw2 := &mockMiddleware{name: "second", priority: 10, enabled: true}
	chain.Add(mw2)

	if len(chain.middleware) != 2 {
		t.Errorf("Expected 2 middleware after Add, got %d", len(chain.middleware))
	}

	// Should be re-sorted after add
	if chain.middleware[0].Priority() != 10 {
		t.Errorf("Expected first middleware to have priority 10 after sort, got %d", chain.middleware[0].Priority())
	}
}

func TestChain_List(t *testing.T) {
	mw1 := &mockMiddleware{name: "first", priority: 10, enabled: true}
	mw2 := &mockMiddleware{name: "second", priority: 20, enabled: true}

	chain := NewChain(mw1, mw2)
	list := chain.List()

	if len(list) != 2 {
		t.Errorf("Expected 2 middleware in list, got %d", len(list))
	}

	// Should be a copy, not the original slice
	list[0] = nil
	if chain.middleware[0] == nil {
		t.Error("List() should return a copy, not the original slice")
	}
}

func TestChain_NilFinalHandler(t *testing.T) {
	mw := &mockMiddleware{name: "test", priority: 10, enabled: true}
	chain := NewChain(mw)

	handler := chain.Handler(nil)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 with nil handler, got %d", w.Code)
	}
}
