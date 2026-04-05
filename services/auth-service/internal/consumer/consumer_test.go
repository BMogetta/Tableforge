package consumer

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/alicebob/miniredis/v2"
	sharedevents "github.com/recess/shared/events"
	"github.com/recess/shared/testutil"
)

type mockRevoker struct {
	revokedPlayers []uuid.UUID
}

func (m *mockRevoker) RevokeAllSessions(_ context.Context, playerID uuid.UUID) error {
	m.revokedPlayers = append(m.revokedPlayers, playerID)
	return nil
}

func setup(t *testing.T) (*Consumer, *miniredis.Miniredis, *redis.Client, *mockRevoker) {
	t.Helper()
	rdb, mr := testutil.NewTestRedis(t)
	rev := &mockRevoker{}
	c := New(rdb, slog.Default(), rev)
	return c, mr, rdb, rev
}

func TestHandlePlayerBanned_PublishesSessionRevoked(t *testing.T) {
	cons, _, rdb, rev := setup(t)
	ctx := context.Background()

	// Subscribe to the output channel before publishing.
	sub := rdb.Subscribe(ctx, channelSessionRevoked)
	defer sub.Close()
	// Wait for subscription to be active.
	_, err := sub.Receive(ctx)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	playerID := uuid.New()
	evt := sharedevents.PlayerBanned{
		Meta:     sharedevents.Meta{EventID: uuid.New().String(), Version: 1},
		PlayerID: playerID.String(),
		BanID:    uuid.New().String(),
		BannedBy: uuid.New().String(),
		Reason:   "toxic behavior",
	}
	payload, _ := json.Marshal(evt)

	// Call handle directly (bypass Redis subscription loop).
	err = cons.handle(ctx, &redis.Message{
		Channel: channelPlayerBanned,
		Payload: string(payload),
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}

	// Read the published revocation event.
	ch := sub.Channel()
	select {
	case msg := <-ch:
		var revoked sharedevents.PlayerSessionRevoked
		if err := json.Unmarshal([]byte(msg.Payload), &revoked); err != nil {
			t.Fatalf("unmarshal revoked: %v", err)
		}
		if revoked.PlayerID != playerID.String() {
			t.Errorf("PlayerID = %q, want %q", revoked.PlayerID, playerID.String())
		}
		if revoked.Reason != "banned" {
			t.Errorf("Reason = %q, want %q", revoked.Reason, "banned")
		}
		if revoked.SessionID != "" {
			t.Errorf("SessionID = %q, want empty", revoked.SessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session revoked event")
	}

	// Verify DB sessions were revoked.
	if len(rev.revokedPlayers) != 1 || rev.revokedPlayers[0] != playerID {
		t.Errorf("expected revoker to be called with %s, got %v", playerID, rev.revokedPlayers)
	}
}

func TestHandlePlayerBanned_InvalidJSON(t *testing.T) {
	cons, _, _, _ := setup(t)
	err := cons.handle(context.Background(), &redis.Message{
		Channel: channelPlayerBanned,
		Payload: "not json",
	})
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestHandlePlayerBanned_InvalidPlayerID(t *testing.T) {
	cons, _, _, _ := setup(t)
	evt := sharedevents.PlayerBanned{
		Meta:     sharedevents.Meta{EventID: uuid.New().String(), Version: 1},
		PlayerID: "not-a-uuid",
	}
	payload, _ := json.Marshal(evt)

	err := cons.handle(context.Background(), &redis.Message{
		Channel: channelPlayerBanned,
		Payload: string(payload),
	})
	if err == nil {
		t.Error("expected error for invalid player_id")
	}
}

func TestHandle_UnknownChannel(t *testing.T) {
	cons, _, _, _ := setup(t)
	err := cons.handle(context.Background(), &redis.Message{
		Channel: "unknown.channel",
		Payload: "{}",
	})
	if err == nil {
		t.Error("expected error for unknown channel")
	}
}
