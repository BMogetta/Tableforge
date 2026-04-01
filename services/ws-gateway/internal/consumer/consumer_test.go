package consumer

import (
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	sharedevents "github.com/tableforge/shared/events"
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
