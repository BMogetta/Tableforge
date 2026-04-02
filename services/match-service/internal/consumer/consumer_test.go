package consumer

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	sharedevents "github.com/recess/shared/events"
)

type mockDequeuer struct {
	calledWith uuid.UUID
	removed    bool
	err        error
}

func (m *mockDequeuer) Dequeue(_ context.Context, playerID uuid.UUID) (bool, error) {
	m.calledWith = playerID
	return m.removed, m.err
}

func newTestConsumer(q *mockDequeuer) *Consumer {
	return &Consumer{queue: q, log: slog.Default()}
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
