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

func TestSurrender_Success(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "surr1")
	p2, _ := fs.CreatePlayer(ctx, "surr2")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "SURR0001", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	fs.AddPlayerToRoom(ctx, room.ID, p2.ID, 1)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)

	result, err := svc.Surrender(ctx, session.ID, p1.ID)
	if err != nil {
		t.Fatalf("Surrender: %v", err)
	}

	if !result.IsOver {
		t.Error("expected game to be over after surrender")
	}
	if result.Result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Result.Status != engine.ResultWin {
		t.Errorf("expected win status, got %s", result.Result.Status)
	}
	// Winner should be p2 (the opponent)
	if result.Result.WinnerID == nil {
		t.Fatal("expected non-nil winner")
	}
	if string(*result.Result.WinnerID) != p2.ID.String() {
		t.Errorf("expected winner %s, got %s", p2.ID, *result.Result.WinnerID)
	}

	// Session should be finished
	refetched, _ := fs.GetGameSession(ctx, session.ID)
	if refetched.FinishedAt == nil {
		t.Error("expected session to be finished")
	}
}

func TestSurrender_SessionNotFound(t *testing.T) {
	svc, _ := newRuntimeWithStore(t)
	_, err := svc.Surrender(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, runtime.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestSurrender_GameOver(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "surr3")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "SURR0003", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	fs.FinishSession(ctx, session.ID)

	_, err := svc.Surrender(ctx, session.ID, p1.ID)
	if !errors.Is(err, runtime.ErrGameOver) {
		t.Errorf("expected ErrGameOver, got %v", err)
	}
}

func TestSurrender_NotParticipant(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "surr4")
	outsider, _ := fs.CreatePlayer(ctx, "outsider4")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "SURR0004", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)

	_, err := svc.Surrender(ctx, session.ID, outsider.ID)
	if !errors.Is(err, runtime.ErrNotParticipant) {
		t.Errorf("expected ErrNotParticipant, got %v", err)
	}
}

func TestSurrender_AllowedWhenSuspended(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "surr5")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "SURR0005", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	fs.SuspendSession(ctx, session.ID, "test")

	result, err := svc.Surrender(ctx, session.ID, p1.ID)
	if err != nil {
		t.Fatalf("expected surrender to succeed on suspended session, got %v", err)
	}
	if !result.IsOver {
		t.Errorf("expected IsOver=true after surrender, got false")
	}
}

// --- VoteRematch tests -------------------------------------------------------

func TestVoteRematch_SessionNotFound(t *testing.T) {
	svc, _ := newRuntimeWithStore(t)
	_, _, _, _, _, err := svc.VoteRematch(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, runtime.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestVoteRematch_NotFinished(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "rematch1")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "REMA0001", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)

	_, _, _, _, _, err := svc.VoteRematch(ctx, session.ID, p1.ID)
	if !errors.Is(err, runtime.ErrNotFinished) {
		t.Errorf("expected ErrNotFinished, got %v", err)
	}
}

func TestVoteRematch_NotParticipant(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "rematch2")
	outsider, _ := fs.CreatePlayer(ctx, "outsider5")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "REMA0002", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	fs.FinishSession(ctx, session.ID)

	_, _, _, _, _, err := svc.VoteRematch(ctx, session.ID, outsider.ID)
	if !errors.Is(err, runtime.ErrNotParticipant) {
		t.Errorf("expected ErrNotParticipant, got %v", err)
	}
}

func TestVoteRematch_SinglePlayer_Consensus(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "rematch3")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "REMA0003", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	fs.FinishSession(ctx, session.ID)

	_, total, roomID, unanimous, mode, err := svc.VoteRematch(ctx, session.ID, p1.ID)
	if err != nil {
		t.Fatalf("VoteRematch: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if roomID != room.ID {
		t.Errorf("expected room %s, got %s", room.ID, roomID)
	}
	if !unanimous {
		t.Error("expected unanimous true with single player")
	}
	if mode != store.SessionModeCasual {
		t.Errorf("expected mode casual, got %s", mode)
	}
}

// TestVoteRematch_RejectsRanked guards the silent ranked→casual downgrade bug.
// A ranked session must always short-circuit with ErrRematchNotAllowedRanked
// so callers never reach the "start new session" path with a stale mode.
func TestVoteRematch_RejectsRanked(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "rankedRematch1")
	p2, _ := fs.CreatePlayer(ctx, "rankedRematch2")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "REMA0099", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	fs.AddPlayerToRoom(ctx, room.ID, p2.ID, 1)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeRanked)
	fs.FinishSession(ctx, session.ID)

	_, _, _, unanimous, mode, err := svc.VoteRematch(ctx, session.ID, p1.ID)
	if !errors.Is(err, runtime.ErrRematchNotAllowedRanked) {
		t.Fatalf("expected ErrRematchNotAllowedRanked, got %v", err)
	}
	if unanimous {
		t.Error("unanimous must be false when rematch is rejected")
	}
	// Mode is still surfaced on rejection so the caller can log/audit the attempt.
	if mode != store.SessionModeRanked {
		t.Errorf("expected mode ranked, got %s", mode)
	}
}

func TestVoteRematch_TwoPlayers_PartialThenConsensus(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "rematch4a")
	p2, _ := fs.CreatePlayer(ctx, "rematch4b")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "REMA0004", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	fs.AddPlayerToRoom(ctx, room.ID, p2.ID, 1)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	fs.FinishSession(ctx, session.ID)

	// First vote — not unanimous
	_, total, _, unanimous, _, err := svc.VoteRematch(ctx, session.ID, p1.ID)
	if err != nil {
		t.Fatalf("VoteRematch p1: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if unanimous {
		t.Error("expected unanimous false after first vote")
	}

	// Second vote — unanimous
	_, _, _, unanimous, _, err = svc.VoteRematch(ctx, session.ID, p2.ID)
	if err != nil {
		t.Fatalf("VoteRematch p2: %v", err)
	}
	if !unanimous {
		t.Error("expected unanimous true after both votes")
	}
}
