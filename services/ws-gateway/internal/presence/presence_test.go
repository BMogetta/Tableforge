package presence

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/recess/shared/testutil"
	"github.com/redis/go-redis/v9"
)

func TestKey(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	got := key(id)
	want := "presence:11111111-1111-1111-1111-111111111111"
	if got != want {
		t.Errorf("key(%s) = %q, want %q", id, got, want)
	}
}

func TestListOnline_Empty(t *testing.T) {
	s := &Store{} // rdb is nil but won't be called for empty slice
	result, err := s.ListOnline(context.TODO(), []uuid.UUID{})
	if err != nil {
		t.Fatalf("ListOnline empty: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func newTestStore(t *testing.T) (*Store, *redis.Client) {
	t.Helper()
	rdb, _ := testutil.NewTestRedis(t)
	return New(rdb), rdb
}

func TestNew(t *testing.T) {
	s, _ := newTestStore(t)
	if s == nil {
		t.Fatal("expected non-nil Store")
	}
	if s.rdb == nil {
		t.Fatal("expected non-nil rdb client")
	}
}

func TestSetAndIsOnline(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	id := uuid.New()

	// Not online yet.
	online, err := s.IsOnline(ctx, id)
	if err != nil {
		t.Fatalf("IsOnline: %v", err)
	}
	if online {
		t.Error("expected offline before Set")
	}

	// Set presence.
	if err := s.Set(ctx, id); err != nil {
		t.Fatalf("Set: %v", err)
	}

	online, err = s.IsOnline(ctx, id)
	if err != nil {
		t.Fatalf("IsOnline after Set: %v", err)
	}
	if !online {
		t.Error("expected online after Set")
	}
}

func TestSetTTL(t *testing.T) {
	rdb, mr := testutil.NewTestRedis(t)
	s := New(rdb)
	ctx := context.Background()
	id := uuid.New()

	if err := s.Set(ctx, id); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Key should exist with TTL close to 30s.
	got := mr.TTL(key(id))
	if got < 25*time.Second || got > 31*time.Second {
		t.Errorf("expected TTL ~30s, got %v", got)
	}
}

func TestDel(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	id := uuid.New()

	if err := s.Set(ctx, id); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Del(ctx, id); err != nil {
		t.Fatalf("Del: %v", err)
	}

	online, err := s.IsOnline(ctx, id)
	if err != nil {
		t.Fatalf("IsOnline after Del: %v", err)
	}
	if online {
		t.Error("expected offline after Del")
	}
}

func TestDelNonExistent(t *testing.T) {
	s, _ := newTestStore(t)
	// Deleting a key that doesn't exist should not error.
	if err := s.Del(context.Background(), uuid.New()); err != nil {
		t.Fatalf("Del non-existent: %v", err)
	}
}

func TestListOnline_Mixed(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	online1 := uuid.New()
	online2 := uuid.New()
	offline := uuid.New()

	if err := s.Set(ctx, online1); err != nil {
		t.Fatalf("Set online1: %v", err)
	}
	if err := s.Set(ctx, online2); err != nil {
		t.Fatalf("Set online2: %v", err)
	}

	result, err := s.ListOnline(ctx, []uuid.UUID{online1, online2, offline})
	if err != nil {
		t.Fatalf("ListOnline: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	if !result[online1] {
		t.Error("expected online1 to be online")
	}
	if !result[online2] {
		t.Error("expected online2 to be online")
	}
	if result[offline] {
		t.Error("expected offline to be offline")
	}
}

func TestListOnline_SinglePlayer(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	id := uuid.New()

	if err := s.Set(ctx, id); err != nil {
		t.Fatalf("Set: %v", err)
	}

	result, err := s.ListOnline(ctx, []uuid.UUID{id})
	if err != nil {
		t.Fatalf("ListOnline: %v", err)
	}
	if !result[id] {
		t.Error("expected player to be online")
	}
}

func TestSetRefreshTTL(t *testing.T) {
	rdb, mr := testutil.NewTestRedis(t)
	s := New(rdb)
	ctx := context.Background()
	id := uuid.New()

	if err := s.Set(ctx, id); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Fast-forward 20s, then refresh.
	mr.FastForward(20 * time.Second)
	if err := s.Set(ctx, id); err != nil {
		t.Fatalf("Set refresh: %v", err)
	}

	// TTL should be reset to ~30s, not ~10s.
	got := mr.TTL(key(id))
	if got < 25*time.Second {
		t.Errorf("expected TTL refreshed to ~30s, got %v", got)
	}
}

func TestTTLExpiry(t *testing.T) {
	rdb, mr := testutil.NewTestRedis(t)
	s := New(rdb)
	ctx := context.Background()
	id := uuid.New()

	if err := s.Set(ctx, id); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Fast-forward past TTL.
	mr.FastForward(31 * time.Second)

	online, err := s.IsOnline(ctx, id)
	if err != nil {
		t.Fatalf("IsOnline after expiry: %v", err)
	}
	if online {
		t.Error("expected offline after TTL expiry")
	}
}
