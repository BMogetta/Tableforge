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
	"github.com/recess/shared/testutil"
	sharedws "github.com/recess/shared/ws"
)

// --- mocks -------------------------------------------------------------------

type mockStore struct {
	params store.CreateParams
	result store.Notification
	err    error
	calls  int
	seen   map[string]bool // key "playerID:sourceEventID"
}

func (m *mockStore) Create(_ context.Context, p store.CreateParams) (store.Notification, error) {
	m.calls++
	m.params = p
	if m.seen == nil {
		m.seen = make(map[string]bool)
	}
	// Mirror the DB UNIQUE(player_id, source_event_id) WHERE source_event_id
	// IS NOT NULL so consumer tests exercise the dedupe short-circuit.
	if p.SourceEventID != nil {
		key := p.PlayerID.String() + ":" + p.SourceEventID.String()
		if m.seen[key] {
			return store.Notification{}, store.ErrDuplicateNotification
		}
		m.seen[key] = true
	}
	return m.result, m.err
}

type mockPublisher struct {
	playerID uuid.UUID
	event    sharedws.Event
	called   bool
}

func (m *mockPublisher) PublishToPlayer(_ context.Context, playerID uuid.UUID, event sharedws.Event) {
	m.called = true
	m.playerID = playerID
	m.event = event
}

func newTestConsumer(st *mockStore, pub *mockPublisher) *Consumer {
	return &Consumer{store: st, pub: pub, log: slog.Default()}
}

func newTestConsumerWithRedis(t *testing.T, st *mockStore, pub *mockPublisher) *Consumer {
	t.Helper()
	rdb, _ := testutil.NewTestRedis(t)
	return &Consumer{rdb: rdb, store: st, pub: pub, log: slog.Default()}
}

// --- friendship.requested ----------------------------------------------------

func TestHandleFriendshipRequested_CreatesNotification(t *testing.T) {
	st := &mockStore{result: store.Notification{ID: uuid.New()}}
	pub := &mockPublisher{}
	c := newTestConsumer(st, pub)

	addresseeID := uuid.New()
	evt := events.FriendshipRequested{
		RequesterID:      uuid.NewString(),
		RequesterUsername: "alice",
		AddresseeID:      addresseeID.String(),
	}
	payload, _ := json.Marshal(evt)
	msg := &redis.Message{Channel: channelFriendshipRequested, Payload: string(payload)}

	if err := c.handle(context.Background(), msg); err != nil {
		t.Fatalf("handle: %v", err)
	}

	if st.params.PlayerID != addresseeID {
		t.Errorf("expected notification for addressee %s, got %s", addresseeID, st.params.PlayerID)
	}
	if st.params.Type != store.NotificationTypeFriendRequest {
		t.Errorf("expected type friend_request, got %s", st.params.Type)
	}
	if st.params.ActionExpiresAt == nil {
		t.Error("expected action_expires_at to be set")
	}
	if !pub.called {
		t.Error("expected PublishToPlayer to be called")
	}
	if pub.playerID != addresseeID {
		t.Errorf("expected publish to %s, got %s", addresseeID, pub.playerID)
	}
	if pub.event.Type != sharedws.EventNotificationReceived {
		t.Errorf("expected event type notification_received, got %s", pub.event.Type)
	}
}

func TestHandleFriendshipRequested_InvalidJSON(t *testing.T) {
	c := newTestConsumer(&mockStore{}, &mockPublisher{})
	msg := &redis.Message{Channel: channelFriendshipRequested, Payload: "bad"}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleFriendshipRequested_InvalidAddresseeID(t *testing.T) {
	c := newTestConsumer(&mockStore{}, &mockPublisher{})
	evt := events.FriendshipRequested{AddresseeID: "bad"}
	payload, _ := json.Marshal(evt)
	msg := &redis.Message{Channel: channelFriendshipRequested, Payload: string(payload)}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error for invalid addressee_id")
	}
}

// --- friendship.accepted -----------------------------------------------------

func TestHandleFriendshipAccepted_CreatesNotification(t *testing.T) {
	st := &mockStore{result: store.Notification{ID: uuid.New()}}
	pub := &mockPublisher{}
	c := newTestConsumer(st, pub)

	requesterID := uuid.New()
	addresseeID := uuid.New()
	evt := events.FriendshipAccepted{
		RequesterID:       requesterID.String(),
		AddresseeID:       addresseeID.String(),
		AddresseeUsername: "bob",
	}
	payload, _ := json.Marshal(evt)
	msg := &redis.Message{Channel: channelFriendshipAccepted, Payload: string(payload)}

	if err := c.handle(context.Background(), msg); err != nil {
		t.Fatalf("handle: %v", err)
	}

	// Notification goes to the requester (the one who sent the original request).
	if st.params.PlayerID != requesterID {
		t.Errorf("expected notification for requester %s, got %s", requesterID, st.params.PlayerID)
	}
	if st.params.Type != store.NotificationTypeFriendRequestAccepted {
		t.Errorf("expected type friend_request_accepted, got %s", st.params.Type)
	}
	if !pub.called || pub.playerID != requesterID {
		t.Error("expected PublishToPlayer to requester")
	}
}

func TestHandleFriendshipAccepted_InvalidJSON(t *testing.T) {
	c := newTestConsumer(&mockStore{}, &mockPublisher{})
	msg := &redis.Message{Channel: channelFriendshipAccepted, Payload: "bad"}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error")
	}
}

// --- player.banned -----------------------------------------------------------

func TestHandlePlayerBanned_CreatesNotification(t *testing.T) {
	st := &mockStore{result: store.Notification{ID: uuid.New()}}
	pub := &mockPublisher{}
	c := newTestConsumer(st, pub)

	playerID := uuid.New()
	expiry := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	evt := events.PlayerBanned{
		PlayerID:  playerID.String(),
		Reason:    "cheating",
		ExpiresAt: &expiry,
	}
	payload, _ := json.Marshal(evt)
	msg := &redis.Message{Channel: channelPlayerBanned, Payload: string(payload)}

	if err := c.handle(context.Background(), msg); err != nil {
		t.Fatalf("handle: %v", err)
	}

	if st.params.PlayerID != playerID {
		t.Errorf("expected notification for %s, got %s", playerID, st.params.PlayerID)
	}
	if st.params.Type != store.NotificationTypeBanIssued {
		t.Errorf("expected type ban_issued, got %s", st.params.Type)
	}
	if !pub.called || pub.playerID != playerID {
		t.Error("expected PublishToPlayer to banned player")
	}
}

func TestHandlePlayerBanned_PermanentBan(t *testing.T) {
	st := &mockStore{result: store.Notification{ID: uuid.New()}}
	pub := &mockPublisher{}
	c := newTestConsumer(st, pub)

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
	if !pub.called {
		t.Error("expected PublishToPlayer even for permanent bans")
	}
}

func TestHandlePlayerBanned_InvalidJSON(t *testing.T) {
	c := newTestConsumer(&mockStore{}, &mockPublisher{})
	msg := &redis.Message{Channel: channelPlayerBanned, Payload: "bad"}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error")
	}
}

func TestHandlePlayerBanned_InvalidPlayerID(t *testing.T) {
	c := newTestConsumer(&mockStore{}, &mockPublisher{})
	evt := events.PlayerBanned{PlayerID: "bad"}
	payload, _ := json.Marshal(evt)
	msg := &redis.Message{Channel: channelPlayerBanned, Payload: string(payload)}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error for invalid player_id")
	}
}

// --- dedupe ------------------------------------------------------------------

func TestHandle_DedupesByEventID(t *testing.T) {
	st := &mockStore{result: store.Notification{ID: uuid.New()}}
	pub := &mockPublisher{}
	c := newTestConsumerWithRedis(t, st, pub)

	addresseeID := uuid.New()
	eventID := uuid.NewString()
	evt := events.FriendshipRequested{
		Meta:              events.Meta{EventID: eventID},
		RequesterID:       uuid.NewString(),
		RequesterUsername: "alice",
		AddresseeID:       addresseeID.String(),
	}
	payload, _ := json.Marshal(evt)
	msg := &redis.Message{Channel: channelFriendshipRequested, Payload: string(payload)}

	if err := c.handle(context.Background(), msg); err != nil {
		t.Fatalf("first handle: %v", err)
	}
	if err := c.handle(context.Background(), msg); err != nil {
		t.Fatalf("second handle: %v", err)
	}

	if st.calls != 1 {
		t.Errorf("expected Create to run once, got %d calls", st.calls)
	}
}

// --- unknown channel ---------------------------------------------------------

func TestHandle_UnknownChannel(t *testing.T) {
	c := newTestConsumer(&mockStore{}, &mockPublisher{})
	msg := &redis.Message{Channel: "unknown.channel", Payload: "{}"}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error for unknown channel")
	}
}
