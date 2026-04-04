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

// --- mock broadcaster --------------------------------------------------------

type mockBroadcaster struct {
	messages [][]byte
}

func (m *mockBroadcaster) BroadcastAll(data []byte) {
	m.messages = append(m.messages, data)
}

// --- tests -------------------------------------------------------------------

func TestHandleSessionRevoked(t *testing.T) {
	disc := &mockDisconnector{}
	c := New(nil, disc, &mockBroadcaster{}, slog.Default())

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
	c := New(nil, disc, &mockBroadcaster{}, slog.Default())

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
	c := New(nil, disc, &mockBroadcaster{}, slog.Default())

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
	c := New(nil, disc, &mockBroadcaster{}, slog.Default())

	msg := &redis.Message{
		Channel: "unknown.channel",
		Payload: "{}",
	}

	if err := c.handle(t.Context(), msg); err == nil {
		t.Fatal("expected error for unknown channel")
	}
}

func TestHandleBroadcastSent(t *testing.T) {
	disc := &mockDisconnector{}
	bc := &mockBroadcaster{}
	c := New(nil, disc, bc, slog.Default())

	evt := sharedevents.AdminBroadcastSent{
		Meta:          sharedevents.Meta{EventID: uuid.NewString()},
		Message:       "Server restart in 5 min",
		BroadcastType: "warning",
		SentBy:        uuid.NewString(),
	}
	payload, _ := json.Marshal(evt)

	msg := &redis.Message{
		Channel: channelBroadcastSent,
		Payload: string(payload),
	}

	if err := c.handle(t.Context(), msg); err != nil {
		t.Fatalf("handle: %v", err)
	}

	if len(bc.messages) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(bc.messages))
	}

	// Verify the WS event structure
	var wsEvt map[string]any
	if err := json.Unmarshal(bc.messages[0], &wsEvt); err != nil {
		t.Fatalf("unmarshal ws event: %v", err)
	}
	if wsEvt["type"] != "broadcast" {
		t.Errorf("expected type=broadcast, got %v", wsEvt["type"])
	}
	p, _ := wsEvt["payload"].(map[string]any)
	if p["message"] != "Server restart in 5 min" {
		t.Errorf("expected message='Server restart in 5 min', got %v", p["message"])
	}
	if p["broadcast_type"] != "warning" {
		t.Errorf("expected broadcast_type=warning, got %v", p["broadcast_type"])
	}
}

func TestHandleBroadcastSent_InvalidJSON(t *testing.T) {
	bc := &mockBroadcaster{}
	c := New(nil, &mockDisconnector{}, bc, slog.Default())

	msg := &redis.Message{
		Channel: channelBroadcastSent,
		Payload: "not json",
	}

	if err := c.handle(t.Context(), msg); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if len(bc.messages) != 0 {
		t.Error("should not broadcast on invalid JSON")
	}
}
