package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/recess/services/user-service/internal/store"
	"github.com/recess/shared/events"
)

func newTestPublisher(t *testing.T) (*Publisher, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewPublisher(rdb), rdb
}

// subscribe listens on channel, calls fn (which publishes), and returns the first message.
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

func TestPublishPlayerBanned(t *testing.T) {
	pub, rdb := newTestPublisher(t)
	expires := time.Now().Add(24 * time.Hour)
	reason := "cheating"
	ban := store.Ban{
		ID:        uuid.New(),
		PlayerID:  uuid.New(),
		BannedBy:  uuid.New(),
		Reason:    &reason,
		ExpiresAt: &expires,
	}

	msg := subscribe(t, rdb, channelPlayerBanned, func() {
		pub.PublishPlayerBanned(context.Background(), ban)
	})

	var evt events.PlayerBanned
	if err := json.Unmarshal([]byte(msg), &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if evt.PlayerID != ban.PlayerID.String() {
		t.Errorf("player_id: got %s, want %s", evt.PlayerID, ban.PlayerID)
	}
	if evt.BanID != ban.ID.String() {
		t.Errorf("ban_id: got %s, want %s", evt.BanID, ban.ID)
	}
	if evt.Reason != reason {
		t.Errorf("reason: got %q, want %q", evt.Reason, reason)
	}
	if evt.ExpiresAt == nil {
		t.Error("expires_at should not be nil")
	}
	if evt.Meta.EventID == "" {
		t.Error("meta.event_id should be set")
	}
}

func TestPublishPlayerBanned_NoExpiry(t *testing.T) {
	pub, rdb := newTestPublisher(t)
	ban := store.Ban{
		ID:       uuid.New(),
		PlayerID: uuid.New(),
		BannedBy: uuid.New(),
	}

	msg := subscribe(t, rdb, channelPlayerBanned, func() {
		pub.PublishPlayerBanned(context.Background(), ban)
	})

	var evt events.PlayerBanned
	if err := json.Unmarshal([]byte(msg), &evt); err != nil {
		t.Fatal(err)
	}
	if evt.ExpiresAt != nil {
		t.Error("expires_at should be nil for permanent ban")
	}
}

func TestPublishPlayerUnbanned(t *testing.T) {
	pub, rdb := newTestPublisher(t)
	banID := uuid.New()
	playerID := uuid.New()
	liftedBy := uuid.New()

	msg := subscribe(t, rdb, channelPlayerUnbanned, func() {
		pub.PublishPlayerUnbanned(context.Background(), banID, playerID, liftedBy)
	})

	var evt events.PlayerUnbanned
	if err := json.Unmarshal([]byte(msg), &evt); err != nil {
		t.Fatal(err)
	}
	if evt.PlayerID != playerID.String() {
		t.Errorf("player_id mismatch")
	}
	if evt.LiftedBy != liftedBy.String() {
		t.Errorf("lifted_by mismatch")
	}
}

func TestPublishFriendshipRequested(t *testing.T) {
	pub, rdb := newTestPublisher(t)

	msg := subscribe(t, rdb, channelFriendshipRequested, func() {
		pub.PublishFriendshipRequested(context.Background(), "req-id", "alice", "addr-id")
	})

	var evt events.FriendshipRequested
	if err := json.Unmarshal([]byte(msg), &evt); err != nil {
		t.Fatal(err)
	}
	if evt.RequesterID != "req-id" {
		t.Errorf("requester_id: got %s", evt.RequesterID)
	}
	if evt.RequesterUsername != "alice" {
		t.Errorf("requester_username: got %s", evt.RequesterUsername)
	}
	if evt.AddresseeID != "addr-id" {
		t.Errorf("addressee_id: got %s", evt.AddresseeID)
	}
}

func TestPublishFriendshipAccepted(t *testing.T) {
	pub, rdb := newTestPublisher(t)
	f := store.Friendship{
		RequesterID: uuid.New(),
		AddresseeID: uuid.New(),
	}

	msg := subscribe(t, rdb, channelFriendshipAccepted, func() {
		pub.PublishFriendshipAccepted(context.Background(), f)
	})

	var evt events.FriendshipAccepted
	if err := json.Unmarshal([]byte(msg), &evt); err != nil {
		t.Fatal(err)
	}
	if evt.RequesterID != f.RequesterID.String() {
		t.Errorf("requester_id mismatch")
	}
}

func TestPublishBroadcast(t *testing.T) {
	pub, rdb := newTestPublisher(t)

	msg := subscribe(t, rdb, channelBroadcastSent, func() {
		pub.PublishBroadcast(context.Background(), "hello world", "info", "admin-id")
	})

	var evt events.AdminBroadcastSent
	if err := json.Unmarshal([]byte(msg), &evt); err != nil {
		t.Fatal(err)
	}
	if evt.Message != "hello world" {
		t.Errorf("message: got %q", evt.Message)
	}
	if evt.BroadcastType != "info" {
		t.Errorf("broadcast_type: got %q", evt.BroadcastType)
	}
}

func TestPublishAchievementUnlocked(t *testing.T) {
	pub, rdb := newTestPublisher(t)

	msg := subscribe(t, rdb, channelAchievementUnlocked, func() {
		pub.PublishAchievementUnlocked(context.Background(), "player-1", "first_win", 1, "Bronze")
	})

	var evt events.AchievementUnlocked
	if err := json.Unmarshal([]byte(msg), &evt); err != nil {
		t.Fatal(err)
	}
	if evt.PlayerID != "player-1" {
		t.Errorf("player_id: got %s", evt.PlayerID)
	}
	if evt.AchievementKey != "first_win" {
		t.Errorf("achievement_key: got %s", evt.AchievementKey)
	}
	if evt.Tier != 1 {
		t.Errorf("tier: got %d", evt.Tier)
	}
}
