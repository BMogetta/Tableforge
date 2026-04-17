package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	sharedevents "github.com/recess/shared/events"
	"github.com/recess/shared/testutil"
)

type mockDequeuer struct {
	calls      int
	calledWith uuid.UUID
	removed    bool
	err        error
}

func (m *mockDequeuer) Dequeue(_ context.Context, playerID uuid.UUID) (bool, error) {
	m.calls++
	m.calledWith = playerID
	return m.removed, m.err
}

func newTestConsumer(q *mockDequeuer) *Consumer {
	return &Consumer{queue: q, log: slog.Default()}
}

func newTestConsumerWithRedis(t *testing.T, q *mockDequeuer) *Consumer {
	t.Helper()
	rdb, _ := testutil.NewTestRedis(t)
	return &Consumer{rdb: rdb, queue: q, log: slog.Default()}
}

func playerBannedJSON(playerID string) string {
	evt := sharedevents.PlayerBanned{PlayerID: playerID, Reason: "cheating"}
	b, _ := json.Marshal(evt)
	return string(b)
}

func TestHandlePlayerBanned_Dequeues(t *testing.T) {
	mock := &mockDequeuer{removed: true}
	c := newTestConsumer(mock)

	playerID := uuid.New()
	msg := &redis.Message{Channel: channelPlayerBanned, Payload: playerBannedJSON(playerID.String())}

	if err := c.handle(context.Background(), msg); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if mock.calledWith != playerID {
		t.Errorf("expected Dequeue(%s), got Dequeue(%s)", playerID, mock.calledWith)
	}
}

func TestHandlePlayerBanned_NotInQueue(t *testing.T) {
	mock := &mockDequeuer{removed: false}
	c := newTestConsumer(mock)

	msg := &redis.Message{Channel: channelPlayerBanned, Payload: playerBannedJSON(uuid.NewString())}

	if err := c.handle(context.Background(), msg); err != nil {
		t.Fatalf("handle: %v", err)
	}
}

func TestHandlePlayerBanned_InvalidJSON(t *testing.T) {
	c := newTestConsumer(&mockDequeuer{})

	msg := &redis.Message{Channel: channelPlayerBanned, Payload: "not json"}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandlePlayerBanned_InvalidPlayerID(t *testing.T) {
	c := newTestConsumer(&mockDequeuer{})

	msg := &redis.Message{Channel: channelPlayerBanned, Payload: playerBannedJSON("bad-uuid")}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error for invalid player_id")
	}
}

func TestHandle_UnknownChannel(t *testing.T) {
	c := newTestConsumer(&mockDequeuer{})

	msg := &redis.Message{Channel: "unknown.channel", Payload: "{}"}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error for unknown channel")
	}
}

func TestHandlePlayerBanned_DequeueError(t *testing.T) {
	mock := &mockDequeuer{err: errors.New("redis down")}
	c := newTestConsumer(mock)

	playerID := uuid.New()
	msg := &redis.Message{Channel: channelPlayerBanned, Payload: playerBannedJSON(playerID.String())}

	err := c.handle(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error when Dequeue fails")
	}
	if mock.calledWith != playerID {
		t.Errorf("expected Dequeue(%s), got Dequeue(%s)", playerID, mock.calledWith)
	}
}

func TestHandlePlayerBanned_EmptyPlayerID(t *testing.T) {
	c := newTestConsumer(&mockDequeuer{})

	// Empty player_id should fail UUID parsing
	msg := &redis.Message{Channel: channelPlayerBanned, Payload: `{"player_id":"","reason":"test"}`}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error for empty player_id")
	}
}

func TestHandlePlayerBanned_WithEventID(t *testing.T) {
	mock := &mockDequeuer{removed: true}
	c := newTestConsumer(mock)

	playerID := uuid.New()
	payload := `{"event_id":"` + uuid.NewString() + `","player_id":"` + playerID.String() + `","reason":"cheating"}`
	msg := &redis.Message{Channel: channelPlayerBanned, Payload: payload}

	if err := c.handle(context.Background(), msg); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if mock.calledWith != playerID {
		t.Errorf("expected Dequeue(%s), got Dequeue(%s)", playerID, mock.calledWith)
	}
}

func TestHandle_UnknownChannel_ErrorMessage(t *testing.T) {
	c := newTestConsumer(&mockDequeuer{})

	msg := &redis.Message{Channel: "player.suspended", Payload: "{}"}
	err := c.handle(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error")
	}
	want := "unknown channel: player.suspended"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestHandlePlayerBanned_DedupeByEventID(t *testing.T) {
	mock := &mockDequeuer{removed: true}
	c := newTestConsumerWithRedis(t, mock)

	playerID := uuid.New()
	eventID := uuid.NewString()
	payload := `{"event_id":"` + eventID + `","player_id":"` + playerID.String() + `","reason":"cheating"}`
	msg := &redis.Message{Channel: channelPlayerBanned, Payload: payload}

	if err := c.handle(context.Background(), msg); err != nil {
		t.Fatalf("first handle: %v", err)
	}
	if err := c.handle(context.Background(), msg); err != nil {
		t.Fatalf("second handle: %v", err)
	}

	if mock.calls != 1 {
		t.Errorf("expected Dequeue to run once, got %d calls", mock.calls)
	}
}

func TestNew(t *testing.T) {
	c := New(nil, &mockDequeuer{}, slog.Default())
	if c == nil {
		t.Fatal("New returned nil")
	}
	if c.queue == nil {
		t.Error("queue should not be nil")
	}
}
