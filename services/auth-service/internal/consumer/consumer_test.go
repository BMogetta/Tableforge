package consumer

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

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

// Unit: handle() decodes JSON, revokes sessions, publishes session.revoked.
func TestHandlePlayerBanned_PublishesSessionRevoked(t *testing.T) {
	rdb, _ := testutil.NewTestRedis(t)
	rev := &mockRevoker{}
	cons := New(rdb, slog.Default(), rev, "test-consumer")

	ctx := context.Background()

	// Subscribe to the output Pub/Sub channel before publishing.
	sub := rdb.Subscribe(ctx, channelSessionRevoked)
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
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

	if err := cons.handle(ctx, payload); err != nil {
		t.Fatalf("handle: %v", err)
	}

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

	if len(rev.revokedPlayers) != 1 || rev.revokedPlayers[0] != playerID {
		t.Errorf("expected revoker called with %s, got %v", playerID, rev.revokedPlayers)
	}
}

func TestHandlePlayerBanned_InvalidJSON(t *testing.T) {
	rdb, _ := testutil.NewTestRedis(t)
	cons := New(rdb, slog.Default(), &mockRevoker{}, "test-consumer")

	if err := cons.handle(context.Background(), []byte("not json")); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestHandlePlayerBanned_InvalidPlayerID(t *testing.T) {
	rdb, _ := testutil.NewTestRedis(t)
	cons := New(rdb, slog.Default(), &mockRevoker{}, "test-consumer")

	evt := sharedevents.PlayerBanned{
		Meta:     sharedevents.Meta{EventID: uuid.New().String(), Version: 1},
		PlayerID: "not-a-uuid",
	}
	payload, _ := json.Marshal(evt)

	if err := cons.handle(context.Background(), payload); err == nil {
		t.Error("expected error for invalid player_id")
	}
}
