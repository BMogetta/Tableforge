package hub

import (
	"testing"
	"time"

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

func TestFanout_DeliversToAllClients(t *testing.T) {
	h := New(nil)
	roomID := uuid.New()

	c1 := newTestClient(h, roomID, uuid.New(), false)
	c2 := newTestClient(h, roomID, uuid.New(), false)
	spec := newTestClient(h, roomID, uuid.New(), true)

	h.SubscribeRoom(roomID, c1)
	h.SubscribeRoom(roomID, c2)
	h.SubscribeRoom(roomID, spec)

	data := []byte(`{"type":"move_applied","payload":{}}`)
	h.fanout(roomID, "move_applied", data)

	// All three should receive.
	for i, c := range []*Client{c1, c2, spec} {
		select {
		case msg := <-c.send:
			if string(msg) != string(data) {
				t.Errorf("client %d: payload mismatch", i)
			}
		default:
			t.Errorf("client %d: expected message", i)
		}
	}
}

func TestFanout_DropsSlowClient(t *testing.T) {
	h := New(nil)
	roomID := uuid.New()

	fast := newTestClient(h, roomID, uuid.New(), false)
	// Slow client: unbuffered channel that's already full.
	slow := &Client{
		hub:      h,
		RoomID:   roomID,
		PlayerID: uuid.New(),
		send:     make(chan []byte), // unbuffered — will block
	}

	h.SubscribeRoom(roomID, fast)
	h.SubscribeRoom(roomID, slow)

	data := []byte(`{"type":"move_applied"}`)
	h.fanout(roomID, "move_applied", data)

	// Fast client gets the message.
	select {
	case <-fast.send:
	default:
		t.Error("fast client should receive message")
	}

	// Slow client's channel should be closed (dropped).
	_, open := <-slow.send
	if open {
		t.Error("slow client channel should be closed")
	}
}

func TestFanoutSpectatorsOnly(t *testing.T) {
	h := New(nil)
	roomID := uuid.New()

	participant := newTestClient(h, roomID, uuid.New(), false)
	spectator := newTestClient(h, roomID, uuid.New(), true)

	h.SubscribeRoom(roomID, participant)
	h.SubscribeRoom(roomID, spectator)

	data := []byte(`{"type":"spectator_joined"}`)
	h.fanoutSpectatorsOnly(roomID, "spectator_joined", data)

	// Only spectator receives.
	select {
	case <-spectator.send:
	default:
		t.Error("spectator should receive message")
	}

	select {
	case <-participant.send:
		t.Error("participant should not receive spectator-only message")
	default:
	}
}

func TestFanoutSpectatorsOnly_RespectsBlocklist(t *testing.T) {
	h := New(nil)
	roomID := uuid.New()

	spectator := newTestClient(h, roomID, uuid.New(), true)
	h.SubscribeRoom(roomID, spectator)

	// rematch_vote is blocklisted — should not be delivered even to spectators-only fanout.
	data := []byte(`{"type":"rematch_vote"}`)
	h.fanoutSpectatorsOnly(roomID, "rematch_vote", data)

	select {
	case <-spectator.send:
		t.Error("blocklisted event should not be delivered via fanoutSpectatorsOnly")
	default:
	}
}

func TestFanoutPlayer(t *testing.T) {
	h := New(nil)
	playerID := uuid.New()
	otherID := uuid.New()

	c1 := newTestClient(h, uuid.Nil, playerID, false)
	c2 := newTestClient(h, uuid.Nil, playerID, false)
	other := newTestClient(h, uuid.Nil, otherID, false)

	h.SubscribePlayer(playerID, c1)
	h.SubscribePlayer(playerID, c2)
	h.SubscribePlayer(otherID, other)

	data := []byte(`{"type":"dm_received","payload":{}}`)
	h.fanoutPlayer(playerID, data)

	// Both of playerID's clients receive.
	for i, c := range []*Client{c1, c2} {
		select {
		case msg := <-c.send:
			if string(msg) != string(data) {
				t.Errorf("client %d: payload mismatch", i)
			}
		default:
			t.Errorf("client %d: expected message", i)
		}
	}

	// Other player does NOT receive.
	select {
	case <-other.send:
		t.Error("other player should not receive message")
	default:
	}
}

func TestFanoutPlayer_DropsSlowClient(t *testing.T) {
	h := New(nil)
	playerID := uuid.New()

	fast := newTestClient(h, uuid.Nil, playerID, false)
	slow := &Client{
		hub:      h,
		PlayerID: playerID,
		send:     make(chan []byte),
	}

	h.SubscribePlayer(playerID, fast)
	h.SubscribePlayer(playerID, slow)

	h.fanoutPlayer(playerID, []byte(`{"type":"dm_received"}`))

	select {
	case <-fast.send:
	default:
		t.Error("fast client should receive message")
	}

	_, open := <-slow.send
	if open {
		t.Error("slow client channel should be closed")
	}
}

func TestGracefulShutdown(t *testing.T) {
	h := New(nil)
	roomID := uuid.New()
	playerID := uuid.New()

	roomClient := newTestClient(h, roomID, uuid.New(), false)
	playerClient := newTestClient(h, uuid.Nil, playerID, false)

	h.SubscribeRoom(roomID, roomClient)
	h.SubscribePlayer(playerID, playerClient)

	h.gracefulShutdown(5 * time.Second)

	// Both channels should be closed.
	_, open := <-roomClient.send
	if open {
		t.Error("room client channel should be closed after shutdown")
	}

	_, open = <-playerClient.send
	if open {
		t.Error("player client channel should be closed after shutdown")
	}
}

func TestSpectatorCount_EmptyRoom(t *testing.T) {
	h := New(nil)
	count := h.SpectatorCount(uuid.New())
	if count != 0 {
		t.Errorf("expected 0 for non-existent room, got %d", count)
	}
}

func TestDisconnectPlayer_NoClients(t *testing.T) {
	h := New(nil)
	// Should not panic when no clients exist.
	h.DisconnectPlayer(uuid.New())
}

func TestSendDirect_FullChannel(t *testing.T) {
	h := New(nil)
	c := &Client{
		hub:      h,
		PlayerID: uuid.New(),
		send:     make(chan []byte), // unbuffered
	}

	// Should not block — drops silently.
	h.SendDirect(c, []byte(`{"type":"test"}`))

	select {
	case <-c.send:
		t.Error("unbuffered channel should not have received (no reader)")
	default:
	}
}

// --- Additional coverage tests ------------------------------------------------

func TestDuplicateSubscribeRoom(t *testing.T) {
	h := New(nil)
	roomID := uuid.New()
	c := newTestClient(h, roomID, uuid.New(), false)

	h.SubscribeRoom(roomID, c)
	h.SubscribeRoom(roomID, c) // duplicate — map set semantics, count stays 1

	h.mu.RLock()
	count := len(h.rooms[roomID])
	h.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 client after duplicate subscribe, got %d", count)
	}
}

func TestDuplicateSubscribePlayer(t *testing.T) {
	h := New(nil)
	playerID := uuid.New()
	c := newTestClient(h, uuid.Nil, playerID, false)

	h.SubscribePlayer(playerID, c)
	h.SubscribePlayer(playerID, c)

	h.pmu.RLock()
	count := len(h.players[playerID])
	h.pmu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 client after duplicate subscribe, got %d", count)
	}
}

func TestUnsubscribeRoom_NonExistentRoom(t *testing.T) {
	h := New(nil)
	c := newTestClient(h, uuid.New(), uuid.New(), false)

	// Should not panic.
	h.UnsubscribeRoom(uuid.New(), c)
}

func TestUnsubscribeRoom_NonExistentClient(t *testing.T) {
	h := New(nil)
	roomID := uuid.New()
	c1 := newTestClient(h, roomID, uuid.New(), false)
	c2 := newTestClient(h, roomID, uuid.New(), false)

	h.SubscribeRoom(roomID, c1)
	h.UnsubscribeRoom(roomID, c2) // c2 was never subscribed

	h.mu.RLock()
	count := len(h.rooms[roomID])
	h.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 client to remain, got %d", count)
	}
}

func TestUnsubscribePlayer_NonExistentPlayer(t *testing.T) {
	h := New(nil)
	c := newTestClient(h, uuid.Nil, uuid.New(), false)

	// Should not panic.
	h.UnsubscribePlayer(uuid.New(), c)
}

func TestUnsubscribePlayer_NonExistentClient(t *testing.T) {
	h := New(nil)
	playerID := uuid.New()
	c1 := newTestClient(h, uuid.Nil, playerID, false)
	c2 := newTestClient(h, uuid.Nil, playerID, false)

	h.SubscribePlayer(playerID, c1)
	h.UnsubscribePlayer(playerID, c2) // c2 never subscribed

	h.pmu.RLock()
	count := len(h.players[playerID])
	h.pmu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 client to remain, got %d", count)
	}
}

func TestFanout_EmptyRoom(t *testing.T) {
	h := New(nil)
	// Should not panic when fanning out to a room with no clients.
	h.fanout(uuid.New(), "move_applied", []byte(`{"type":"move_applied"}`))
}

func TestFanoutPlayer_NoClients(t *testing.T) {
	h := New(nil)
	// Should not panic when fanning out to a player with no clients.
	h.fanoutPlayer(uuid.New(), []byte(`{"type":"dm_received"}`))
}

func TestFanoutSpectatorsOnly_EmptyRoom(t *testing.T) {
	h := New(nil)
	// Should not panic.
	h.fanoutSpectatorsOnly(uuid.New(), "spectator_joined", []byte(`{"type":"spectator_joined"}`))
}

func TestFanoutSpectatorsOnly_DropsSlowSpectator(t *testing.T) {
	h := New(nil)
	roomID := uuid.New()

	fast := newTestClient(h, roomID, uuid.New(), true)
	slow := &Client{
		hub:       h,
		RoomID:    roomID,
		PlayerID:  uuid.New(),
		send:      make(chan []byte), // unbuffered — will block
		Spectator: true,
	}

	h.SubscribeRoom(roomID, fast)
	h.SubscribeRoom(roomID, slow)

	data := []byte(`{"type":"spectator_joined"}`)
	h.fanoutSpectatorsOnly(roomID, "spectator_joined", data)

	select {
	case <-fast.send:
	default:
		t.Error("fast spectator should receive message")
	}

	_, open := <-slow.send
	if open {
		t.Error("slow spectator channel should be closed")
	}
}

func TestFanout_AllBlocklistedEvents(t *testing.T) {
	blocklisted := []struct {
		eventType string
	}{
		{"rematch_vote"},
		{"pause_vote_update"},
		{"session_suspended"},
		{"resume_vote_update"},
		{"session_resumed"},
	}

	for _, tc := range blocklisted {
		t.Run(tc.eventType, func(t *testing.T) {
			h := New(nil)
			roomID := uuid.New()

			participant := newTestClient(h, roomID, uuid.New(), false)
			spectator := newTestClient(h, roomID, uuid.New(), true)

			h.SubscribeRoom(roomID, participant)
			h.SubscribeRoom(roomID, spectator)

			data := []byte(`{"type":"` + tc.eventType + `"}`)
			h.fanout(roomID, EventType(tc.eventType), data)

			select {
			case <-participant.send:
			default:
				t.Error("participant should receive blocklisted event")
			}

			select {
			case <-spectator.send:
				t.Errorf("spectator should NOT receive blocklisted event %s", tc.eventType)
			default:
			}
		})
	}
}

func TestFanout_NonBlocklistedDeliveredToSpectators(t *testing.T) {
	nonBlocked := []EventType{
		"move_applied",
		"game_over",
		"player_joined",
		"chat_message",
		"spectator_joined",
	}

	for _, et := range nonBlocked {
		t.Run(string(et), func(t *testing.T) {
			h := New(nil)
			roomID := uuid.New()

			spectator := newTestClient(h, roomID, uuid.New(), true)
			h.SubscribeRoom(roomID, spectator)

			data := []byte(`{"type":"` + string(et) + `"}`)
			h.fanout(roomID, et, data)

			select {
			case <-spectator.send:
			default:
				t.Errorf("spectator should receive non-blocklisted event %s", et)
			}
		})
	}
}

func TestGracefulShutdown_EmptyHub(t *testing.T) {
	h := New(nil)
	// Should not panic.
	h.gracefulShutdown(5 * time.Second)
}

func TestDisconnectPlayer_FullChannel(t *testing.T) {
	h := New(nil)
	playerID := uuid.New()

	// Client with unbuffered send channel (blocks on send).
	c := &Client{
		hub:      h,
		PlayerID: playerID,
		send:     make(chan []byte),
	}

	h.SubscribePlayer(playerID, c)
	h.DisconnectPlayer(playerID)

	// When send blocks, DisconnectPlayer falls through to close(c.send).
	_, open := <-c.send
	if open {
		t.Error("channel should be closed when send blocks")
	}
}

func TestMultipleDisconnectPlayer(t *testing.T) {
	h := New(nil)
	playerID := uuid.New()
	c := newTestClient(h, uuid.Nil, playerID, false)

	h.SubscribePlayer(playerID, c)

	// First disconnect sends nil signal.
	h.DisconnectPlayer(playerID)

	select {
	case msg := <-c.send:
		if msg != nil {
			t.Error("expected nil signal")
		}
	default:
		t.Error("expected nil signal on send channel")
	}

	// Second call should not panic (clients still in map until unsubscribed by ReadPump).
	h.DisconnectPlayer(playerID)
}

func TestSpectatorCount_NoSpectators(t *testing.T) {
	h := New(nil)
	roomID := uuid.New()

	c1 := newTestClient(h, roomID, uuid.New(), false)
	c2 := newTestClient(h, roomID, uuid.New(), false)

	h.SubscribeRoom(roomID, c1)
	h.SubscribeRoom(roomID, c2)

	count := h.SpectatorCount(roomID)
	if count != 0 {
		t.Errorf("expected 0 spectators in room with only participants, got %d", count)
	}
}

func TestFanoutSpectatorsOnly_ParticipantsExcluded(t *testing.T) {
	h := New(nil)
	roomID := uuid.New()

	p1 := newTestClient(h, roomID, uuid.New(), false)
	p2 := newTestClient(h, roomID, uuid.New(), false)
	s1 := newTestClient(h, roomID, uuid.New(), true)

	h.SubscribeRoom(roomID, p1)
	h.SubscribeRoom(roomID, p2)
	h.SubscribeRoom(roomID, s1)

	data := []byte(`{"type":"spectator_joined"}`)
	h.fanoutSpectatorsOnly(roomID, "spectator_joined", data)

	select {
	case <-s1.send:
	default:
		t.Error("spectator should receive spectator-only message")
	}

	for i, p := range []*Client{p1, p2} {
		select {
		case <-p.send:
			t.Errorf("participant %d should not receive spectator-only message", i)
		default:
		}
	}
}

func TestSubscribeUnsubscribe_MultipleRooms(t *testing.T) {
	h := New(nil)
	room1 := uuid.New()
	room2 := uuid.New()

	c1 := newTestClient(h, room1, uuid.New(), false)
	c2 := newTestClient(h, room2, uuid.New(), false)

	h.SubscribeRoom(room1, c1)
	h.SubscribeRoom(room2, c2)

	h.mu.RLock()
	r1Count := len(h.rooms[room1])
	r2Count := len(h.rooms[room2])
	h.mu.RUnlock()

	if r1Count != 1 || r2Count != 1 {
		t.Errorf("expected 1 client per room, got room1=%d room2=%d", r1Count, r2Count)
	}

	h.UnsubscribeRoom(room1, c1)

	h.mu.RLock()
	_, room1Exists := h.rooms[room1]
	r2Count = len(h.rooms[room2])
	h.mu.RUnlock()

	if room1Exists {
		t.Error("room1 should be cleaned up")
	}
	if r2Count != 1 {
		t.Errorf("room2 should still have 1 client, got %d", r2Count)
	}
}

func TestGracefulShutdown_MultipleRoomsAndPlayers(t *testing.T) {
	h := New(nil)

	room1 := uuid.New()
	room2 := uuid.New()
	player1 := uuid.New()
	player2 := uuid.New()

	rc1 := newTestClient(h, room1, uuid.New(), false)
	rc2 := newTestClient(h, room2, uuid.New(), true)
	pc1 := newTestClient(h, uuid.Nil, player1, false)
	pc2 := newTestClient(h, uuid.Nil, player2, false)

	h.SubscribeRoom(room1, rc1)
	h.SubscribeRoom(room2, rc2)
	h.SubscribePlayer(player1, pc1)
	h.SubscribePlayer(player2, pc2)

	h.gracefulShutdown(5 * time.Second)

	for i, c := range []*Client{rc1, rc2, pc1, pc2} {
		_, open := <-c.send
		if open {
			t.Errorf("client %d: channel should be closed after shutdown", i)
		}
	}
}
