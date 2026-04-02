package redisutil

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// Connect parses the URL, instruments tracing, pings, and returns a client.
// Fatals on connection failure — services can't run without Redis.
func Connect(ctx context.Context, rawURL string) *redis.Client {
	opts, err := redis.ParseURL(rawURL)
	if err != nil {
		slog.Error("redis: parse url", "error", err)
		os.Exit(1)
	}

	rdb := redis.NewClient(opts)

	if err := redisotel.InstrumentTracing(rdb); err != nil {
		slog.Error("redis: instrument tracing", "error", err)
		os.Exit(1)
	}

	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Error("redis: ping", "error", err)
		os.Exit(1)
	}

	slog.Info("redis: connected")
	return rdb
}

// MustConnect is an alias for Connect — explicit name for use in main().
var MustConnect = Connect

// NewClient is the lower-level variant that returns an error instead of fataling.
// Use this in tests or when you need custom error handling.
func NewClient(ctx context.Context, rawURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	rdb := redis.NewClient(opts)

	if err := redisotel.InstrumentTracing(rdb); err != nil {
		return nil, fmt.Errorf("instrument tracing: %w", err)
	}

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	return rdb, nil
}
