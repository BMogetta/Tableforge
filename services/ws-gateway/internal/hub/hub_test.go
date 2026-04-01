package hub

import (
	"testing"

	"github.com/google/uuid"
)

func newTestClient(h *Hub, roomID, playerID uuid.UUID, spectator bool) *Client {
	return &Client{
		hub:       h,
		RoomID:    roomID,
		PlayerID:  playerID,
		send:      make(chan []byte, 64),
		Spectator: spectator,
	}
}

func TestSubscribeUnsubscribeRoom(t *testing.T) {
	h := New(nil)
	roomID := uuid.New()
	c := newTestClient(h, roomID, uuid.New(), false)

	h.SubscribeRoom(roomID, c)

	h.mu.RLock()
	count := len(h.rooms[roomID])
	h.mu.RUnlock()

	if count != 1 {
		t.Fatalf("expected 1 client in room, got %d", count)
	}

	h.UnsubscribeRoom(roomID, c)

	h.mu.RLock()
	_, exists := h.rooms[roomID]
	h.mu.RUnlock()

	if exists {
		t.Error("expected room to be removed after last client unsubscribes")
	}
}

func TestSubscribeUnsubscribePlayer(t *testing.T) {
	h := New(nil)
	playerID := uuid.New()
	c := newTestClient(h, uuid.Nil, playerID, false)

	h.SubscribePlayer(playerID, c)

	h.pmu.RLock()
	count := len(h.players[playerID])
	h.pmu.RUnlock()

	if count != 1 {
		t.Fatalf("expected 1 client for player, got %d", count)
	}

	h.UnsubscribePlayer(playerID, c)

	h.pmu.RLock()
	_, exists := h.players[playerID]
	h.pmu.RUnlock()

	if exists {
		t.Error("expected player entry to be removed after last client unsubscribes")
	}
}

func TestDisconnectPlayer(t *testing.T) {
	h := New(nil)
	playerID := uuid.New()
	c1 := newTestClient(h, uuid.Nil, playerID, false)
	c2 := newTestClient(h, uuid.Nil, playerID, false)

	h.SubscribePlayer(playerID, c1)
	h.SubscribePlayer(playerID, c2)

	h.DisconnectPlayer(playerID)

	// Both clients should receive nil on their send channels
	for i, c := range []*Client{c1, c2} {
		select {
		case msg := <-c.send:
			if msg != nil {
				t.Errorf("client %d: expected nil signal, got data", i)
			}
		default:
			t.Errorf("client %d: expected nil signal on send channel", i)
		}
	}
}

func TestSpectatorCount(t *testing.T) {
	h := New(nil)
	roomID := uuid.New()

	participant := newTestClient(h, roomID, uuid.New(), false)
	spectator1 := newTestClient(h, roomID, uuid.New(), true)
	spectator2 := newTestClient(h, roomID, uuid.New(), true)

	h.SubscribeRoom(roomID, participant)
	h.SubscribeRoom(roomID, spectator1)
	h.SubscribeRoom(roomID, spectator2)

	count := h.SpectatorCount(roomID)
	if count != 2 {
		t.Errorf("expected 2 spectators, got %d", count)
	}
}

func TestSendDirect(t *testing.T) {
	h := New(nil)
	c := newTestClient(h, uuid.Nil, uuid.New(), false)

	data := []byte(`{"type":"test"}`)
	h.SendDirect(c, data)

	select {
	case msg := <-c.send:
		if string(msg) != string(data) {
			t.Errorf("expected %s, got %s", data, msg)
		}
	default:
		t.Error("expected message on send channel")
	}
}

func TestFanout_BlocksSpectatorsOnBlocklistedEvents(t *testing.T) {
	h := New(nil)
	roomID := uuid.New()

	participant := newTestClient(h, roomID, uuid.New(), false)
	spectator := newTestClient(h, roomID, uuid.New(), true)

	h.SubscribeRoom(roomID, participant)
	h.SubscribeRoom(roomID, spectator)

	// "rematch_vote" is in SpectatorBlocklist
	data := []byte(`{"type":"rematch_vote"}`)
	h.fanout(roomID, "rematch_vote", data)

	// Participant should receive it
	select {
	case <-participant.send:
		// ok
	default:
		t.Error("participant should receive rematch_vote")
	}

	// Spectator should NOT receive it (blocked)
	select {
	case <-spectator.send:
		t.Error("spectator should not receive rematch_vote (blocklisted)")
	default:
		// ok
	}
}

func TestMultipleClientsPerRoom(t *testing.T) {
	h := New(nil)
	roomID := uuid.New()

	c1 := newTestClient(h, roomID, uuid.New(), false)
	c2 := newTestClient(h, roomID, uuid.New(), false)

	h.SubscribeRoom(roomID, c1)
	h.SubscribeRoom(roomID, c2)

	h.mu.RLock()
	count := len(h.rooms[roomID])
	h.mu.RUnlock()

	if count != 2 {
		t.Errorf("expected 2 clients, got %d", count)
	}

	// Remove one — room should still exist
	h.UnsubscribeRoom(roomID, c1)

	h.mu.RLock()
	count = len(h.rooms[roomID])
	h.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 client after unsubscribe, got %d", count)
	}
}
