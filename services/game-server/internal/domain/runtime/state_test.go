package runtime_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/domain/runtime"
	"github.com/recess/game-server/internal/platform/store"
)

func TestGetSessionAndState_NotFound(t *testing.T) {
	svc, _ := newRuntimeWithStore(t)
	_, _, _, err := svc.GetSessionAndState(context.Background(), uuid.New())
	if !errors.Is(err, runtime.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestGetSessionAndState_FinishedWithResult(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "state1")
	p2, _ := fs.CreatePlayer(ctx, "state2")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "STAT0001", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	_ = fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	_ = fs.AddPlayerToRoom(ctx, room.ID, p2.ID, 1)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{"board": "empty"}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)

	// Finish session and create a game result
	_ = fs.FinishSession(ctx, session.ID)
	_, _ = fs.CreateGameResult(ctx, store.CreateGameResultParams{
		SessionID: session.ID,
		GameID:    "tictactoe",
		WinnerID:  &p1.ID,
		IsDraw:    false,
		EndedBy:   store.EndedByNormal,
		Players:   []store.GameResultPlayer{{PlayerID: p1.ID, Seat: 0, Outcome: store.OutcomeWin}},
	})

	sess, gameState, result, err := svc.GetSessionAndState(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSessionAndState: %v", err)
	}
	if sess.ID != session.ID {
		t.Errorf("expected session ID %s, got %s", session.ID, sess.ID)
	}
	if gameState.Data["board"] != "empty" {
		t.Errorf("expected board=empty in state data")
	}
	if result == nil {
		t.Fatal("expected non-nil game result for finished session")
	}
	if result.WinnerID == nil || *result.WinnerID != p1.ID {
		t.Error("expected winner to be p1")
	}
}

func TestGetSessionAndState_ActiveSession_NoResult(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "state3")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "STAT0002", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	_ = fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)

	_, _, result, err := svc.GetSessionAndState(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSessionAndState: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for active session")
	}
}

func TestApplyMove_Suspended(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "susp1")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "SUSP0001", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	_ = fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	_ = fs.SuspendSession(ctx, session.ID, "pause_vote")

	_, err := svc.ApplyMove(ctx, session.ID, p1.ID, map[string]any{})
	if !errors.Is(err, runtime.ErrSuspended) {
		t.Errorf("expected ErrSuspended, got %v", err)
	}
}
