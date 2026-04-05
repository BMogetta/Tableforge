package testutil

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// NewTestRedis spins up an in-memory Redis (miniredis) and returns a connected
// client. The server is automatically closed when the test ends.
// The optional *miniredis.Miniredis is returned for tests that need to
// manipulate time (FastForward) or inspect keys directly.
func NewTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return rdb, mr
}
