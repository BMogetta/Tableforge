package consumer

import (
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	sharedevents "github.com/recess/shared/events"
)

// --- mock disconnector -------------------------------------------------------

type mockDisconnector struct {
	disconnected []uuid.UUID
}

func (m *mockDisconnector) DisconnectPlayer(playerID uuid.UUID) {
	m.disconnected = append(m.disconnected, playerID)
}

// --- tests -------------------------------------------------------------------

func TestHandleSessionRevoked(t *testing.T) {
	disc := &mockDisconnector{}
	c := New(nil, disc, slog.Default())

	playerID := uuid.New()
	evt := sharedevents.PlayerSessionRevoked{
		Meta:      sharedevents.Meta{EventID: uuid.NewString()},
		PlayerID:  playerID.String(),
		SessionID: uuid.NewString(),
		Reason:    "banned",
	}
	payload, _ := json.Marshal(evt)

	msg := &redis.Message{
		Channel: channelSessionRevoked,
		Payload: string(payload),
	}

	if err := c.handle(t.Context(), msg); err != nil {
		t.Fatalf("handle: %v", err)
	}

	if len(disc.disconnected) != 1 {
		t.Fatalf("expected 1 disconnect, got %d", len(disc.disconnected))
	}
	if disc.disconnected[0] != playerID {
		t.Errorf("expected disconnect for %s, got %s", playerID, disc.disconnected[0])
	}
}

func TestHandleSessionRevoked_InvalidJSON(t *testing.T) {
	disc := &mockDisconnector{}
	c := New(nil, disc, slog.Default())

	msg := &redis.Message{
		Channel: channelSessionRevoked,
		Payload: "not json",
	}

	if err := c.handle(t.Context(), msg); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if len(disc.disconnected) != 0 {
		t.Error("should not disconnect on invalid JSON")
	}
}

func TestHandleSessionRevoked_InvalidPlayerID(t *testing.T) {
	disc := &mockDisconnector{}
	c := New(nil, disc, slog.Default())

	payload, _ := json.Marshal(map[string]string{
		"player_id": "not-a-uuid",
		"reason":    "banned",
	})

	msg := &redis.Message{
		Channel: channelSessionRevoked,
		Payload: string(payload),
	}

	if err := c.handle(t.Context(), msg); err == nil {
		t.Fatal("expected error for invalid player_id")
	}
	if len(disc.disconnected) != 0 {
		t.Error("should not disconnect on invalid player_id")
	}
}

func TestHandle_UnknownChannel(t *testing.T) {
	disc := &mockDisconnector{}
	c := New(nil, disc, slog.Default())

	msg := &redis.Message{
		Channel: "unknown.channel",
		Payload: "{}",
	}

	if err := c.handle(t.Context(), msg); err == nil {
		t.Fatal("expected error for unknown channel")
	}
}

// --- Additional coverage tests ------------------------------------------------

func TestHandleSessionRevoked_EmptyPlayerID(t *testing.T) {
	disc := &mockDisconnector{}
	c := New(nil, disc, slog.Default())

	payload, _ := json.Marshal(sharedevents.PlayerSessionRevoked{
		Meta:      sharedevents.Meta{EventID: uuid.NewString()},
		PlayerID:  "",
		SessionID: uuid.NewString(),
		Reason:    "banned",
	})

	msg := &redis.Message{
		Channel: channelSessionRevoked,
		Payload: string(payload),
	}

	if err := c.handle(t.Context(), msg); err == nil {
		t.Fatal("expected error for empty player_id")
	}
	if len(disc.disconnected) != 0 {
		t.Error("should not disconnect on empty player_id")
	}
}

func TestHandleSessionRevoked_EmptyJSON(t *testing.T) {
	disc := &mockDisconnector{}
	c := New(nil, disc, slog.Default())

	msg := &redis.Message{
		Channel: channelSessionRevoked,
		Payload: "{}",
	}

	// Empty JSON unmarshals to zero-value struct with empty player_id.
	if err := c.handle(t.Context(), msg); err == nil {
		t.Fatal("expected error for empty JSON (empty player_id)")
	}
	if len(disc.disconnected) != 0 {
		t.Error("should not disconnect on empty JSON")
	}
}

func TestHandleSessionRevoked_AllReasons(t *testing.T) {
	reasons := []string{"logout", "superseded", "banned"}

	for _, reason := range reasons {
		t.Run(reason, func(t *testing.T) {
			disc := &mockDisconnector{}
			c := New(nil, disc, slog.Default())

			playerID := uuid.New()
			evt := sharedevents.PlayerSessionRevoked{
				Meta:      sharedevents.Meta{EventID: uuid.NewString()},
				PlayerID:  playerID.String(),
				SessionID: uuid.NewString(),
				Reason:    reason,
			}
			payload, _ := json.Marshal(evt)

			msg := &redis.Message{
				Channel: channelSessionRevoked,
				Payload: string(payload),
			}

			if err := c.handle(t.Context(), msg); err != nil {
				t.Fatalf("handle: %v", err)
			}

			if len(disc.disconnected) != 1 {
				t.Fatalf("expected 1 disconnect, got %d", len(disc.disconnected))
			}
			if disc.disconnected[0] != playerID {
				t.Errorf("expected disconnect for %s, got %s", playerID, disc.disconnected[0])
			}
		})
	}
}

func TestHandle_MultipleUnknownChannels(t *testing.T) {
	channels := []string{
		"",
		"player.session",
		"player.session.revoked.extra",
		"game.session.finished",
	}

	for _, ch := range channels {
		t.Run(ch, func(t *testing.T) {
			disc := &mockDisconnector{}
			c := New(nil, disc, slog.Default())

			msg := &redis.Message{
				Channel: ch,
				Payload: "{}",
			}

			if err := c.handle(t.Context(), msg); err == nil {
				t.Fatalf("expected error for unknown channel %q", ch)
			}
		})
	}
}

func TestHandleSessionRevoked_MultipleConsecutiveCalls(t *testing.T) {
	disc := &mockDisconnector{}
	c := New(nil, disc, slog.Default())

	for i := 0; i < 3; i++ {
		playerID := uuid.New()
		evt := sharedevents.PlayerSessionRevoked{
			Meta:      sharedevents.Meta{EventID: uuid.NewString()},
			PlayerID:  playerID.String(),
			SessionID: uuid.NewString(),
			Reason:    "banned",
		}
		payload, _ := json.Marshal(evt)

		msg := &redis.Message{
			Channel: channelSessionRevoked,
			Payload: string(payload),
		}

		if err := c.handle(t.Context(), msg); err != nil {
			t.Fatalf("handle call %d: %v", i, err)
		}
	}

	if len(disc.disconnected) != 3 {
		t.Fatalf("expected 3 disconnects, got %d", len(disc.disconnected))
	}
}
