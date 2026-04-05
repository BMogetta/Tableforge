package publisher

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/recess/shared/testutil"
	sharedws "github.com/recess/shared/ws"
)

func TestPublishToPlayer_PublishesCorrectChannel(t *testing.T) {
	rdb, mr := testutil.NewTestRedis(t)
	pub := New(rdb, slog.Default())
	ctx := context.Background()
	playerID := uuid.New()

	channel := sharedws.PlayerChannelKey(playerID)
	sub := rdb.Subscribe(ctx, channel)
	defer sub.Close()
	// Wait for subscription to be ready.
	sub.Receive(ctx)

	event := sharedws.Event{
		Type:    sharedws.EventNotificationReceived,
		Payload: map[string]any{"title": "hello"},
	}

	pub.PublishToPlayer(ctx, playerID, event)

	msg, err := sub.ReceiveMessage(ctx)
	if err != nil {
		t.Fatalf("ReceiveMessage: %v", err)
	}
	if msg.Channel != channel {
		t.Errorf("expected channel %s, got %s", channel, msg.Channel)
	}

	var got sharedws.Event
	if err := json.Unmarshal([]byte(msg.Payload), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Type != sharedws.EventNotificationReceived {
		t.Errorf("expected type notification, got %s", got.Type)
	}

	_ = mr // keep reference alive
}

func TestPublishToPlayer_RedisDown_DoesNotPanic(t *testing.T) {
	rdb, mr := testutil.NewTestRedis(t)
	pub := New(rdb, slog.Default())

	// Close Redis to simulate failure.
	mr.Close()

	// Should log error but not panic.
	pub.PublishToPlayer(context.Background(), uuid.New(), sharedws.Event{
		Type:    sharedws.EventNotificationReceived,
		Payload: map[string]any{"title": "test"},
	})
}
