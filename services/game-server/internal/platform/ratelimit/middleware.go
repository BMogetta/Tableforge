package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter enforces a sliding-window rate limit backed by Redis.
type Limiter struct {
	runner   ScriptRunner
	clock    Clock
	limit    int
	window   time.Duration
	keyspace string
}

// New creates a production-ready Limiter.
// It uses a real Redis-backed script runner and the system clock.
func New(rdb *redis.Client, limit int, window time.Duration) *Limiter {
	return &Limiter{
		runner:   newRedisScriptRunner(rdb),
		clock:    RealClock{},
		limit:    limit,
		window:   window,
		keyspace: "rl",
	}
}

// NewWithDeps allows injecting custom dependencies (used in unit tests).
func NewWithDeps(
	runner ScriptRunner,
	clock Clock,
	limit int,
	window time.Duration,
	keyspace string,
) *Limiter {
	return &Limiter{
		runner:   runner,
		clock:    clock,
		limit:    limit,
		window:   window,
		keyspace: keyspace,
	}
}

// WithKeyspace returns a new Limiter sharing the same runner and clock
// but using a different keyspace and policy.
func (l *Limiter) WithKeyspace(keyspace string, limit int, window time.Duration) *Limiter {
	return &Limiter{
		runner:   l.runner,
		clock:    l.clock,
		limit:    limit,
		window:   window,
		keyspace: keyspace,
	}
}

// Middleware wraps an HTTP handler with rate limiting.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		allowed, remaining, reset, err := l.allow(r.Context(), ip)
		if err != nil {
			// Redis unavailable — fail open to avoid blocking all traffic.
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(l.limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))

		if !allowed {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{
				"error":    "rate limit exceeded",
				"reset_at": reset,
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// allow executes the sliding-window rate limiting logic.
func (l *Limiter) allow(
	ctx context.Context,
	ip string,
) (allowed bool, remaining int, resetAt int64, err error) {

	key := fmt.Sprintf("%s:%s", l.keyspace, ip)

	now := l.clock.Now()
	windowStart := now.Add(-l.window)

	nowNs := now.UnixNano()
	winNs := windowStart.UnixNano()
	ttlMs := l.window.Milliseconds()
	member := strconv.FormatInt(nowNs, 10)

	res, err := l.runner.Run(ctx, ScriptArgs{
		Key:    key,
		NowNs:  nowNs,
		WinNs:  winNs,
		Limit:  l.limit,
		TTLms:  ttlMs,
		Member: member,
	})
	if err != nil {
		return false, 0, 0, err
	}

	resetAt = now.Add(l.window).Unix()

	if len(res) > 0 && res[0] == 1 {
		return true, int(res[1]), resetAt, nil
	}

	return false, 0, resetAt, nil
}
