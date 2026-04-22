package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func newTestPublisher(t *testing.T) (*Publisher, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewPublisher(rdb), rdb
}

func subscribe(t *testing.T, rdb *redis.Client, channel string, fn func()) string {
	t.Helper()
	ctx := context.Background()
	sub := rdb.Subscribe(ctx, channel)
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	fn()
	msg, err := sub.ReceiveMessage(ctx)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	return msg.Payload
}

func TestPublishRoomEvent(t *testing.T) {
	pub, rdb := newTestPublisher(t)
	roomID := uuid.New()
	channel := "ws:room:" + roomID.String()

	payload := map[string]any{"text": "hello"}
	msg := subscribe(t, rdb, channel, func() {
		pub.PublishRoomEvent(context.Background(), roomID, eventChatMessage, payload)
	})

	// Outer envelope: filteredEnvelope{S: false, Data: ...}
	var env filteredEnvelope
	if err := json.Unmarshal([]byte(msg), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.S {
		t.Error("S should be false (deliver to all)")
	}

	// Inner event
	var evt event
	if err := json.Unmarshal(env.Data, &evt); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if evt.Type != eventChatMessage {
		t.Errorf("type: got %q, want %q", evt.Type, eventChatMessage)
	}
}

func TestPublishPlayerEvent(t *testing.T) {
	pub, rdb := newTestPublisher(t)
	playerID := uuid.New()
	channel := "ws:player:" + playerID.String()

	payload := map[string]any{"from": "alice"}
	msg := subscribe(t, rdb, channel, func() {
		pub.PublishPlayerEvent(context.Background(), playerID, eventDMReceived, payload)
	})

	var env filteredEnvelope
	if err := json.Unmarshal([]byte(msg), &env); err != nil {
		t.Fatal(err)
	}

	var evt event
	if err := json.Unmarshal(env.Data, &evt); err != nil {
		t.Fatal(err)
	}
	if evt.Type != eventDMReceived {
		t.Errorf("type: got %q, want %q", evt.Type, eventDMReceived)
	}
}

func TestPublisher_NilRedis_NoPanic(t *testing.T) {
	pub := &Publisher{rdb: nil}
	// Should not panic.
	pub.PublishRoomEvent(context.Background(), uuid.New(), eventChatMessage, "test")
}
