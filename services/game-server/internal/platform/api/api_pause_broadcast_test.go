package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	sharedws "github.com/recess/shared/ws"

	"github.com/recess/game-server/internal/domain/lobby"
	"github.com/recess/game-server/internal/domain/runtime"
	"github.com/recess/game-server/internal/platform/api"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/game-server/internal/platform/ws"
	sharedmw "github.com/recess/shared/middleware"
)

// TestVotePause_BroadcastSuspendedAtMatchesSession verifies the
// EventSessionSuspended broadcast carries the session's DB-assigned
// SuspendedAt, not a fresh wall-clock reading. This protects the
// reproducibility invariant: broadcast payloads for deterministic state
// transitions must match persisted state.
func TestVotePause_BroadcastSuspendedAtMatchesSession(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	hub := ws.NewHubWithRedis(rdb)

	fs := newFakeStore()
	reg := newFakeRegistry(&stubGame{}, &stubTicTacToeGame{})
	lobbySvc := lobby.New(fs, reg)
	rt := runtime.New(fs, reg, nil, rdb, nil)
	router := api.NewRouter(lobbySvc, rt, fs, hub, nil, nil, nil, nil, nil, nil)

	// Seed: two humans in a running session.
	ownerID := uuid.New()
	guestID := uuid.New()
	roomID := uuid.New()
	sessionID := uuid.New()
	fs.Rooms[roomID] = store.Room{
		ID: roomID, GameID: "stub", OwnerID: ownerID,
		Status: store.RoomStatusInProgress, MaxPlayers: 2,
	}
	fs.RoomPlayers[roomID] = []store.RoomPlayer{
		{RoomID: roomID, PlayerID: ownerID, Seat: 0},
		{RoomID: roomID, PlayerID: guestID, Seat: 1},
	}
	fs.Sessions[sessionID] = store.GameSession{
		ID:         sessionID,
		RoomID:     roomID,
		GameID:     "stub",
		State:      []byte(`{"current_player_id":"","data":{}}`),
		LastMoveAt: time.Now(),
		StartedAt:  time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	sub := rdb.Subscribe(ctx, sharedws.RoomChannelKey(roomID))
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	cast := func(playerID uuid.UUID) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sessionID.String()+"/pause", nil)
		req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), playerID, "u", "player"))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	if w := cast(ownerID); w.Code != http.StatusOK {
		t.Fatalf("owner vote: got %d: %s", w.Code, w.Body.String())
	}
	if w := cast(guestID); w.Code != http.StatusOK {
		t.Fatalf("guest vote: got %d: %s", w.Code, w.Body.String())
	}

	// Drain WS events until we see the session_suspended one. Pause-vote
	// updates land on the same channel first.
	var suspendedAt string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := sub.ReceiveMessage(ctx)
		if err != nil {
			t.Fatalf("receive: %v", err)
		}
		var env sharedws.FilteredEnvelope
		if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
			t.Fatalf("envelope: %v", err)
		}
		var event struct {
			Type    string                 `json:"type"`
			Payload map[string]any         `json:"payload"`
		}
		raw := env.Data
		if len(raw) == 0 {
			raw = []byte(msg.Payload)
		}
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatalf("event: %v", err)
		}
		if event.Type == "session_suspended" {
			v, _ := event.Payload["suspended_at"].(string)
			suspendedAt = v
			break
		}
	}
	if suspendedAt == "" {
		t.Fatal("did not receive session_suspended event")
	}

	sess := fs.Sessions[sessionID]
	if sess.SuspendedAt == nil {
		t.Fatal("session.SuspendedAt was not set by the handler")
	}
	want := sess.SuspendedAt.Format(time.RFC3339Nano)
	if suspendedAt != want {
		t.Errorf("broadcast suspended_at=%q, want DB value %q (handler must not use time.Now())",
			suspendedAt, want)
	}
}
