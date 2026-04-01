package api

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tableforge/services/user-service/internal/store"
)

// mockPublisher records calls so tests can assert on published events.
// It embeds nothing — it replaces *Publisher by living behind the same
// concrete type. Since handlers receive *Publisher and call methods on it,
// we cannot swap an interface. Instead, tests that need publish assertions
// should use the fakePublisher wrapper below, and tests that don't care
// about publishing can pass a nil *Publisher (only safe when the handler
// under test does not call pub).
//
// For handlers that DO call pub (AcceptFriend, IssueBan, LiftBan), we
// create a real Publisher backed by a no-op redis so the nil-deref is
// avoided. The fakeRedis approach is not needed here because we simply
// create a Publisher with a nil redis client and override its publish
// method at the call-site — but Go doesn't let us do that on a concrete
// type.
//
// Pragmatic solution: wrap the entire Router call with a Publisher whose
// redis client is nil. The publish method logs an error on failure and
// does NOT return it, so the handler still succeeds. This is acceptable
// for unit tests.

// stubPublisher returns a *Publisher with a dummy redis client.
// Publish calls will fail with connection errors that get logged and
// swallowed (the handler ignores publish errors), avoiding nil panics.
// DialTimeout is set to 1ms so tests don't block on connection attempts.
func stubPublisher() *Publisher {
	rdb := redis.NewClient(&redis.Options{
		Addr:        "localhost:0",
		DialTimeout: time.Millisecond,
		MaxRetries:  0,
	})
	return &Publisher{rdb: rdb}
}

// publishRecord captures one publish call for assertion purposes.
type publishRecord struct {
	Channel string
	Ban     *store.Ban
	BanID   *uuid.UUID
}

// fakePublisher records publish calls in-memory.
// Not used in this iteration because handlers call concrete *Publisher,
// but kept as a reference if Publisher is later extracted to an interface.
type fakePublisher struct {
	mu      sync.Mutex
	records []publishRecord
}

func (f *fakePublisher) record(channel string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records = append(f.records, publishRecord{Channel: channel})
}

func (f *fakePublisher) PublishPlayerBanned(_ context.Context, ban store.Ban) {
	f.record(channelPlayerBanned)
}

func (f *fakePublisher) PublishPlayerUnbanned(_ context.Context, banID, playerID, liftedBy uuid.UUID) {
	f.record(channelPlayerUnbanned)
}

func (f *fakePublisher) PublishFriendshipRequested(_ context.Context, requesterID, requesterUsername, addresseeID string) {
	f.record(channelFriendshipRequested)
}

func (f *fakePublisher) PublishFriendshipAccepted(_ context.Context, friendship store.Friendship) {
	f.record(channelFriendshipAccepted)
}
