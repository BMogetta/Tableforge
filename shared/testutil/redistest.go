package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NewTestRedis spins up an in-memory Redis (miniredis) and returns a connected
// client. The server is automatically closed when the test ends.
// The optional *miniredis.Miniredis is returned for tests that need to
// manipulate time (FastForward) or inspect keys directly.
//
// Use this for the common case (commands, simple Pub/Sub, sorted sets).
// For Redis Streams (XADD / XREADGROUP / XAUTOCLAIM) prefer
// NewTestRedisContainer — miniredis lags on Streams edge cases.
func NewTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb, mr
}

// NewTestRedisContainer spins up a real Redis 7 container via
// testcontainers-go and returns a connected client. The container is
// terminated when the test ends.
//
// Use this when the test exercises commands that miniredis doesn't
// implement faithfully — Streams (XAUTOCLAIM in particular), Functions,
// or modules. Slower than NewTestRedis (~1s container start).
func NewTestRedisContainer(t *testing.T) *redis.Client {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor: wait.ForLog("Ready to accept connections").
			WithStartupTimeout(30 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("testutil: start redis container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("testutil: terminate redis container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("testutil: container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatalf("testutil: container port: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: host + ":" + port.Port()})
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("testutil: redis ping: %v", err)
	}
	return rdb
}
