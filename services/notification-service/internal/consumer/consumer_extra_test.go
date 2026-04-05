package consumer

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/recess/notification-service/internal/store"
	"github.com/recess/shared/events"
	sharedws "github.com/recess/shared/ws"
)

// --- additional mocks (compatible with existing test mocks) -------------------

type extraMockStore struct {
	params store.CreateParams
	result store.Notification
}

func (m *extraMockStore) Create(_ context.Context, p store.CreateParams) (store.Notification, error) {
	m.params = p
	return m.result, nil
}

type extraMockPublisher struct {
	playerID uuid.UUID
	event    sharedws.Event
	called   bool
}

func (m *extraMockPublisher) PublishToPlayer(_ context.Context, playerID uuid.UUID, event sharedws.Event) {
	m.called = true
	m.playerID = playerID
	m.event = event
}

func newExtraConsumer(st *extraMockStore, pub *extraMockPublisher) *Consumer {
	return &Consumer{store: st, pub: pub, log: slog.Default()}
}

// --- achievement.unlocked ----------------------------------------------------

func TestHandleAchievementUnlocked(t *testing.T) {
	st := &extraMockStore{result: store.Notification{ID: uuid.New()}}
	pub := &extraMockPublisher{}
	c := newExtraConsumer(st, pub)

	playerID := uuid.New()
	evt := events.AchievementUnlocked{
		PlayerID:       playerID.String(),
		AchievementKey: "games_played",
		Tier:           2,
		TierName:       "Gold",
	}
	payload, _ := json.Marshal(evt)
	msg := &redis.Message{Channel: channelAchievementUnlocked, Payload: string(payload)}

	if err := c.handle(context.Background(), msg); err != nil {
		t.Fatalf("handle: %v", err)
	}

	if st.params.PlayerID != playerID {
		t.Errorf("expected notification for %s, got %s", playerID, st.params.PlayerID)
	}
	if st.params.Type != store.NotificationTypeAchievementUnlocked {
		t.Errorf("expected type achievement_unlocked, got %s", st.params.Type)
	}
	if !pub.called || pub.playerID != playerID {
		t.Error("expected PublishToPlayer for achievement unlock")
	}
}

func TestHandleAchievementUnlocked_InvalidJSON(t *testing.T) {
	c := newExtraConsumer(&extraMockStore{}, &extraMockPublisher{})
	msg := &redis.Message{Channel: channelAchievementUnlocked, Payload: "bad json"}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleAchievementUnlocked_InvalidPlayerID(t *testing.T) {
	c := newExtraConsumer(&extraMockStore{}, &extraMockPublisher{})
	evt := events.AchievementUnlocked{PlayerID: "bad-uuid"}
	payload, _ := json.Marshal(evt)
	msg := &redis.Message{Channel: channelAchievementUnlocked, Payload: string(payload)}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error for invalid player_id")
	}
}

// --- player.banned permanent (no expiry) -------------------------------------

func TestHandlePlayerBanned_Permanent(t *testing.T) {
	st := &extraMockStore{result: store.Notification{ID: uuid.New()}}
	pub := &extraMockPublisher{}
	c := newExtraConsumer(st, pub)

	playerID := uuid.New()
	evt := events.PlayerBanned{
		PlayerID:  playerID.String(),
		Reason:    "permanent",
		ExpiresAt: nil,
	}
	payload, _ := json.Marshal(evt)
	msg := &redis.Message{Channel: channelPlayerBanned, Payload: string(payload)}

	if err := c.handle(context.Background(), msg); err != nil {
		t.Fatalf("handle: %v", err)
	}

	if st.params.PlayerID != playerID {
		t.Errorf("expected notification for %s", playerID)
	}
	if st.params.Type != store.NotificationTypeBanIssued {
		t.Errorf("expected ban_issued, got %s", st.params.Type)
	}
}

// --- player.banned with expiry -----------------------------------------------

func TestHandlePlayerBanned_WithExpiry(t *testing.T) {
	st := &extraMockStore{result: store.Notification{ID: uuid.New()}}
	pub := &extraMockPublisher{}
	c := newExtraConsumer(st, pub)

	playerID := uuid.New()
	expiry := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
	evt := events.PlayerBanned{
		PlayerID:  playerID.String(),
		Reason:    "toxicity",
		ExpiresAt: &expiry,
	}
	payload, _ := json.Marshal(evt)
	msg := &redis.Message{Channel: channelPlayerBanned, Payload: string(payload)}

	if err := c.handle(context.Background(), msg); err != nil {
		t.Fatalf("handle: %v", err)
	}

	if st.params.Type != store.NotificationTypeBanIssued {
		t.Errorf("expected ban_issued, got %s", st.params.Type)
	}
	if !pub.called {
		t.Error("expected publish")
	}
}

// --- unknown channel ---------------------------------------------------------

func TestHandle_UnknownChannel_Extra(t *testing.T) {
	c := newExtraConsumer(&extraMockStore{}, &extraMockPublisher{})
	msg := &redis.Message{Channel: "some.unknown.channel", Payload: "{}"}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error for unknown channel")
	}
}

// --- friendship.accepted invalid IDs -----------------------------------------

func TestHandleFriendshipAccepted_InvalidRequesterID(t *testing.T) {
	c := newExtraConsumer(&extraMockStore{}, &extraMockPublisher{})
	evt := events.FriendshipAccepted{RequesterID: "bad", AddresseeID: uuid.NewString()}
	payload, _ := json.Marshal(evt)
	msg := &redis.Message{Channel: channelFriendshipAccepted, Payload: string(payload)}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error for invalid requester_id")
	}
}

func TestHandleFriendshipAccepted_InvalidAddresseeID(t *testing.T) {
	c := newExtraConsumer(&extraMockStore{}, &extraMockPublisher{})
	evt := events.FriendshipAccepted{RequesterID: uuid.NewString(), AddresseeID: "bad"}
	payload, _ := json.Marshal(evt)
	msg := &redis.Message{Channel: channelFriendshipAccepted, Payload: string(payload)}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error for invalid addressee_id")
	}
}
