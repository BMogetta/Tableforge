package runtime_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/domain/runtime"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/game-server/internal/platform/ws"
	"github.com/recess/game-server/internal/testutil"
	sharedws "github.com/recess/shared/ws"
)

// filterGame implements engine.Game + engine.StateFilter. State shape is
// `{"hands": {playerID: "secret-<id>"}}`. Filtering strips every hand except
// the requested player's; spectator view (playerID=="") gets no hands.
type filterGame struct{}

func (g *filterGame) ID() string      { return "filter-game" }
func (g *filterGame) Name() string    { return "FilterGame" }
func (g *filterGame) MinPlayers() int { return 2 }
func (g *filterGame) MaxPlayers() int { return 4 }
func (g *filterGame) Init(players []engine.Player) (engine.GameState, error) {
	hands := make(map[string]any, len(players))
	for _, p := range players {
		hands[string(p.ID)] = "secret-" + string(p.ID)
	}
	return engine.GameState{CurrentPlayerID: players[0].ID, Data: map[string]any{"hands": hands}}, nil
}
func (g *filterGame) ValidateMove(_ engine.GameState, _ engine.Move) error { return nil }
func (g *filterGame) ApplyMove(s engine.GameState, _ engine.Move) (engine.GameState, error) {
	return s, nil
}
func (g *filterGame) IsOver(_ engine.GameState) (bool, engine.Result) { return false, engine.Result{} }
func (g *filterGame) FilterState(state engine.GameState, playerID engine.PlayerID) engine.GameState {
	out := engine.GameState{CurrentPlayerID: state.CurrentPlayerID, Data: map[string]any{}}
	hands, _ := state.Data["hands"].(map[string]any)
	filtered := map[string]any{}
	if playerID != "" {
		if h, ok := hands[string(playerID)]; ok {
			filtered[string(playerID)] = h
		}
	}
	out.Data["hands"] = filtered
	return out
}

// TestBroadcastSessionStarted_FiltersStatePerPlayer verifies each player
// receives only their own hand and the spectator channel receives no hands.
func TestBroadcastSessionStarted_FiltersStatePerPlayer(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	hub := ws.NewHubWithRedis(rdb)
	fs := testutil.NewFakeStore()
	reg := &fakeRegistry{game: &filterGame{}}
	svc := runtime.New(fs, reg, nil, rdb, nil)

	roomID := uuid.New()
	p1 := uuid.New()
	p2 := uuid.New()
	fs.RoomPlayers[roomID] = []store.RoomPlayer{
		{RoomID: roomID, PlayerID: p1, Seat: 0},
		{RoomID: roomID, PlayerID: p2, Seat: 1},
	}

	initial, _ := (&filterGame{}).Init([]engine.Player{
		{ID: engine.PlayerID(p1.String())},
		{ID: engine.PlayerID(p2.String())},
	})
	stateJSON, _ := json.Marshal(initial)
	session := store.GameSession{
		ID:     uuid.New(),
		RoomID: roomID,
		GameID: "filter-game",
		State:  stateJSON,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	p1Sub := rdb.Subscribe(ctx, sharedws.PlayerChannelKey(p1))
	defer p1Sub.Close()
	p2Sub := rdb.Subscribe(ctx, sharedws.PlayerChannelKey(p2))
	defer p2Sub.Close()
	roomSub := rdb.Subscribe(ctx, sharedws.RoomChannelKey(roomID))
	defer roomSub.Close()

	// Wait for subscriptions to register before publishing, otherwise the
	// messages are dropped on the floor by miniredis.
	for _, s := range []*redis.PubSub{p1Sub, p2Sub, roomSub} {
		if _, err := s.Receive(ctx); err != nil {
			t.Fatalf("subscribe: %v", err)
		}
	}

	svc.BroadcastSessionStarted(ctx, hub, session)

	readEventSession := func(sub *redis.PubSub, name string, envelope bool) store.GameSession {
		msg, err := sub.ReceiveMessage(ctx)
		if err != nil {
			t.Fatalf("%s: receive: %v", name, err)
		}
		raw := []byte(msg.Payload)
		if envelope {
			var env sharedws.FilteredEnvelope
			if err := json.Unmarshal(raw, &env); err != nil {
				t.Fatalf("%s: envelope unmarshal: %v", name, err)
			}
			raw = env.Data
		}
		var event struct {
			Type    string `json:"type"`
			Payload struct {
				Session store.GameSession `json:"session"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatalf("%s: event unmarshal: %v", name, err)
		}
		return event.Payload.Session
	}

	readHands := func(sess store.GameSession, name string) map[string]any {
		var state engine.GameState
		if err := json.Unmarshal(sess.State, &state); err != nil {
			t.Fatalf("%s: state unmarshal: %v", name, err)
		}
		hands, _ := state.Data["hands"].(map[string]any)
		return hands
	}

	s1 := readEventSession(p1Sub, "p1", false)
	s2 := readEventSession(p2Sub, "p2", false)
	sRoom := readEventSession(roomSub, "room", true)

	h1 := readHands(s1, "p1")
	if _, ok := h1[p1.String()]; !ok {
		t.Errorf("p1 should see their own hand, got %v", h1)
	}
	if _, ok := h1[p2.String()]; ok {
		t.Errorf("p1 leaked p2's hand: %v", h1)
	}

	h2 := readHands(s2, "p2")
	if _, ok := h2[p2.String()]; !ok {
		t.Errorf("p2 should see their own hand, got %v", h2)
	}
	if _, ok := h2[p1.String()]; ok {
		t.Errorf("p2 leaked p1's hand: %v", h2)
	}

	hR := readHands(sRoom, "room")
	if len(hR) != 0 {
		t.Errorf("room/spectator channel should receive no hands, got %v", hR)
	}
}
