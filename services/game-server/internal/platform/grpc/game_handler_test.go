package grpchandler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/game-server/internal/testutil"
	gamev1 "github.com/recess/shared/proto/game/v1"
)

func newTestHandler() (*GameHandler, *testutil.FakeStore) {
	st := testutil.NewFakeStore()
	return &GameHandler{st: st}, st
}

func TestGetMoveLog(t *testing.T) {
	h, st := newTestHandler()

	sessionID := uuid.New()
	roomID := uuid.New()
	playerA := uuid.New()
	playerB := uuid.New()

	stateJSON, _ := json.Marshal(map[string]any{"board": "empty"})
	st.Sessions[sessionID] = store.GameSession{
		ID:     sessionID,
		RoomID: roomID,
		GameID: "tictactoe",
		State:  stateJSON,
	}
	st.Moves[sessionID] = []store.Move{
		{ID: uuid.New(), SessionID: sessionID, PlayerID: playerA, Payload: []byte(`{"x":0,"y":0}`), MoveNumber: 1, AppliedAt: time.Now()},
		{ID: uuid.New(), SessionID: sessionID, PlayerID: playerB, Payload: []byte(`{"x":1,"y":1}`), MoveNumber: 2, AppliedAt: time.Now()},
	}

	resp, err := h.GetMoveLog(context.Background(), &gamev1.GetMoveLogRequest{
		SessionId: sessionID.String(),
	})
	if err != nil {
		t.Fatalf("GetMoveLog: %v", err)
	}

	if resp.GameId != "tictactoe" {
		t.Errorf("expected game_id tictactoe, got %s", resp.GameId)
	}
	if len(resp.Moves) != 2 {
		t.Fatalf("expected 2 moves, got %d", len(resp.Moves))
	}
	if resp.Moves[0].MoveNumber != 1 {
		t.Errorf("expected move_number 1, got %d", resp.Moves[0].MoveNumber)
	}
	if resp.Moves[0].PlayerId != playerA.String() {
		t.Errorf("expected player_id %s, got %s", playerA, resp.Moves[0].PlayerId)
	}
	if resp.Moves[1].Payload != `{"x":1,"y":1}` {
		t.Errorf("unexpected payload: %s", resp.Moves[1].Payload)
	}
}

func TestGetMoveLog_EmptySession(t *testing.T) {
	h, st := newTestHandler()

	sessionID := uuid.New()
	stateJSON, _ := json.Marshal(map[string]any{})
	st.Sessions[sessionID] = store.GameSession{
		ID:     sessionID,
		GameID: "tictactoe",
		State:  stateJSON,
	}

	resp, err := h.GetMoveLog(context.Background(), &gamev1.GetMoveLogRequest{
		SessionId: sessionID.String(),
	})
	if err != nil {
		t.Fatalf("GetMoveLog: %v", err)
	}
	if len(resp.Moves) != 0 {
		t.Errorf("expected 0 moves, got %d", len(resp.Moves))
	}
}

func TestGetMoveLog_SessionNotFound(t *testing.T) {
	h, _ := newTestHandler()

	_, err := h.GetMoveLog(context.Background(), &gamev1.GetMoveLogRequest{
		SessionId: uuid.NewString(),
	})
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}

func TestGetMoveLog_MissingSessionID(t *testing.T) {
	h, _ := newTestHandler()

	_, err := h.GetMoveLog(context.Background(), &gamev1.GetMoveLogRequest{})
	if err == nil {
		t.Fatal("expected error for empty session_id")
	}
}

func TestGetMoveLog_InvalidSessionID(t *testing.T) {
	h, _ := newTestHandler()

	_, err := h.GetMoveLog(context.Background(), &gamev1.GetMoveLogRequest{
		SessionId: "not-a-uuid",
	})
	if err == nil {
		t.Fatal("expected error for invalid session_id")
	}
}

func TestGetRoomPlayers(t *testing.T) {
	h, st := newTestHandler()

	roomID := uuid.New()
	p1 := uuid.New()
	p2 := uuid.New()
	st.RoomPlayers[roomID] = []store.RoomPlayer{
		{PlayerID: p1, Seat: 0},
		{PlayerID: p2, Seat: 1},
	}

	resp, err := h.GetRoomPlayers(context.Background(), &gamev1.GetRoomPlayersRequest{
		RoomId: roomID.String(),
	})
	if err != nil {
		t.Fatalf("GetRoomPlayers: %v", err)
	}
	if len(resp.PlayerIds) != 2 {
		t.Fatalf("expected 2 player IDs, got %d", len(resp.PlayerIds))
	}
	if resp.PlayerIds[0] != p1.String() {
		t.Errorf("expected first player %s, got %s", p1, resp.PlayerIds[0])
	}
}

func TestGetRoomPlayers_MissingRoomID(t *testing.T) {
	h, _ := newTestHandler()

	_, err := h.GetRoomPlayers(context.Background(), &gamev1.GetRoomPlayersRequest{})
	if err == nil {
		t.Fatal("expected error for empty room_id")
	}
}
