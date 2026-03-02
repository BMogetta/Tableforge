package ratelimit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tableforge/server/internal/ratelimit"
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

func TestLimiter_Blocked(t *testing.T) {
	runner := &fakeRunner{
		result: []int64{0, 0},
	}
	clock := fakeClock{t: time.Unix(1000, 0)}

	l := ratelimit.NewWithDeps(
		runner,
		clock,
		5,
		time.Minute,
		"test",
	)

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
		t.Fatal("next handler should not be called")
	}
}
