package ratelimit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter holds the Redis client and policy settings.
type Limiter struct {
	rdb      *redis.Client
	limit    int
	window   time.Duration
	keyspace string
}

// New creates a Limiter with the given policy.
// keyspace is used to namespace Redis keys — use distinct values for each policy.
func New(rdb *redis.Client, limit int, window time.Duration) *Limiter {
	return &Limiter{rdb: rdb, limit: limit, window: window, keyspace: "rl"}
}

// WithKeyspace returns a new Limiter with a different keyspace and policy.
// Use this to create per-route limiters from a shared Redis client.
func (l *Limiter) WithKeyspace(keyspace string, limit int, window time.Duration) *Limiter {
	return &Limiter{
		rdb:      l.rdb,
		limit:    limit,
		window:   window,
		keyspace: keyspace,
	}
}

// Middleware returns an http.Handler that enforces the rate limit.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		allowed, remaining, reset, err := l.allow(r.Context(), ip)
		if err != nil {
			// Redis down — fail open to avoid blocking all traffic.
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(l.limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))

		if !allowed {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// allow implements a Redis sorted-set sliding window.
// Key: "{keyspace}:{ip}", members are request timestamps (nanoseconds as score+member).
func (l *Limiter) allow(ctx context.Context, ip string) (allowed bool, remaining int, resetAt int64, err error) {
	key := fmt.Sprintf("%s:%s", l.keyspace, ip)
	now := time.Now()
	windowStart := now.Add(-l.window)

	// Lua script: atomic remove-expired + count + conditional add.
	script := redis.NewScript(`
		local key    = KEYS[1]
		local now    = tonumber(ARGV[1])
		local win    = tonumber(ARGV[2])
		local limit  = tonumber(ARGV[3])
		local ttl    = tonumber(ARGV[4])
		local member = ARGV[5]

		redis.call('ZREMRANGEBYSCORE', key, '-inf', win)
		local count = redis.call('ZCARD', key)

		if count < limit then
			redis.call('ZADD', key, now, member)
			redis.call('PEXPIRE', key, ttl)
			return {1, limit - count - 1}
		end

		return {0, 0}
	`)

	nowNs := now.UnixNano()
	winNs := windowStart.UnixNano()
	ttlMs := l.window.Milliseconds()
	member := strconv.FormatInt(nowNs, 10)

	res, err := script.Run(ctx, l.rdb, []string{key},
		nowNs, winNs, l.limit, ttlMs, member,
	).Int64Slice()
	if err != nil {
		return false, 0, 0, err
	}

	resetAt = now.Add(l.window).Unix()
	if res[0] == 1 {
		return true, int(res[1]), resetAt, nil
	}
	return false, 0, resetAt, nil
}
