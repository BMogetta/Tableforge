package ratelimit_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tableforge/game-server/internal/platform/ratelimit"
)

type fakeRunner struct {
	result []int64
	err    error
}

func (f *fakeRunner) Run(
	_ context.Context,
	_ ratelimit.ScriptArgs,
) ([]int64, error) {
	return f.result, f.err
}

type fakeClock struct {
	t time.Time
}

func (f fakeClock) Now() time.Time {
	return f.t
}

func newTestLimiter(result []int64) (*ratelimit.Limiter, *fakeRunner) {
	runner := &fakeRunner{result: result}
	clock := fakeClock{t: time.Unix(1000, 0)}
	l := ratelimit.NewWithDeps(runner, clock, 5, time.Minute, "test")
	return l, runner
}

func TestLimiter_Blocked(t *testing.T) {
	l, _ := newTestLimiter([]int64{0, 0})

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	l.Middleware(next).ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if nextCalled {
		t.Fatal("next handler should not be called when rate limited")
	}
}

func TestLimiter_Blocked_JSONResponse(t *testing.T) {
	l, _ := newTestLimiter([]int64{0, 0})

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	l.Middleware(next).ServeHTTP(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if body["error"] != "rate limit exceeded" {
		t.Errorf("unexpected error field: %v", body["error"])
	}
	if _, ok := body["reset_at"]; !ok {
		t.Error("expected reset_at field in response")
	}
}

func TestLimiter_Allowed(t *testing.T) {
	l, _ := newTestLimiter([]int64{1, 4})

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	l.Middleware(next).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !nextCalled {
		t.Fatal("next handler should be called when allowed")
	}
}

func TestLimiter_RateLimitHeaders(t *testing.T) {
	l, _ := newTestLimiter([]int64{1, 3})

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	l.Middleware(next).ServeHTTP(w, req)

	if w.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("missing X-RateLimit-Limit header")
	}
	if w.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("missing X-RateLimit-Remaining header")
	}
	if w.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("missing X-RateLimit-Reset header")
	}
}

func TestLimiter_RedisError_FailOpen(t *testing.T) {
	runner := &fakeRunner{err: fmt.Errorf("redis unavailable")}
	clock := fakeClock{t: time.Unix(1000, 0)}
	l := ratelimit.NewWithDeps(runner, clock, 5, time.Minute, "test")

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	l.Middleware(next).ServeHTTP(w, req)

	if !nextCalled {
		t.Fatal("should fail open when Redis is unavailable")
	}
}
