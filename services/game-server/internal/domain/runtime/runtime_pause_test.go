package runtime_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/domain/runtime"
	"github.com/recess/game-server/internal/platform/store"
)

func TestVotePause_SinglePlayerReachesConsensus(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1 := uuid.New()
	_ = makeActiveSession(t, fs, p1)

	// makeActiveSession adds alice (owner) + p1. We need a 1-player room
	// for immediate consensus, so create a dedicated setup.
	owner, _ := fs.CreatePlayer(ctx, "pause1")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "PAUS0001", GameID: "tictactoe", OwnerID: owner.ID, MaxPlayers: 2,
	})
	_ = fs.AddPlayerToRoom(ctx, room.ID, owner.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(owner.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)

	result, err := svc.VotePause(ctx, session.ID, owner.ID)
	if err != nil {
		t.Fatalf("VotePause: %v", err)
	}
	if !result.AllVoted {
		t.Error("expected AllVoted true with single participant")
	}
	if result.Required != 1 {
		t.Errorf("expected required 1, got %d", result.Required)
	}

	// Session should now be suspended.
	refetched, _ := fs.GetGameSession(ctx, session.ID)
	if refetched.SuspendedAt == nil {
		t.Error("expected session to be suspended after all voted")
	}
}

func TestVotePause_TwoPlayersRequireBothVotes(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "pause2a")
	p2, _ := fs.CreatePlayer(ctx, "pause2b")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "PAUS0002", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	_ = fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	_ = fs.AddPlayerToRoom(ctx, room.ID, p2.ID, 1)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)

	// First vote — not yet consensus.
	result, err := svc.VotePause(ctx, session.ID, p1.ID)
	if err != nil {
		t.Fatalf("VotePause p1: %v", err)
	}
	if result.AllVoted {
		t.Error("expected AllVoted false after first of two votes")
	}
	if result.Required != 2 {
		t.Errorf("expected required 2, got %d", result.Required)
	}

	// Second vote — consensus reached.
	result, err = svc.VotePause(ctx, session.ID, p2.ID)
	if err != nil {
		t.Fatalf("VotePause p2: %v", err)
	}
	if !result.AllVoted {
		t.Error("expected AllVoted true after both votes")
	}

	refetched, _ := fs.GetGameSession(ctx, session.ID)
	if refetched.SuspendedAt == nil {
		t.Error("expected session suspended after consensus")
	}
}

func TestVotePause_ErrAlreadyPaused(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "pause3")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "PAUS0003", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	_ = fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	_ = fs.SuspendSession(ctx, session.ID, "already_paused")

	_, err := svc.VotePause(ctx, session.ID, p1.ID)
	if err != runtime.ErrAlreadyPaused {
		t.Errorf("expected ErrAlreadyPaused, got %v", err)
	}
}

func TestVotePause_ErrNotParticipant(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "pause4")
	outsider, _ := fs.CreatePlayer(ctx, "outsider")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "PAUS0004", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	_ = fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)

	_, err := svc.VotePause(ctx, session.ID, outsider.ID)
	if err != runtime.ErrNotParticipant {
		t.Errorf("expected ErrNotParticipant, got %v", err)
	}
}

func TestVotePause_ErrGameOver(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "pause5")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "PAUS0005", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	_ = fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	_ = fs.FinishSession(ctx, session.ID)

	_, err := svc.VotePause(ctx, session.ID, p1.ID)
	if err != runtime.ErrGameOver {
		t.Errorf("expected ErrGameOver, got %v", err)
	}
}

// --- VoteResume tests --------------------------------------------------------

func TestVoteResume_ResumesAfterConsensus(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "resume1")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "RESU0001", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	_ = fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	_ = fs.SuspendSession(ctx, session.ID, "pause_vote")

	result, err := svc.VoteResume(ctx, session.ID, p1.ID)
	if err != nil {
		t.Fatalf("VoteResume: %v", err)
	}
	if !result.AllVoted {
		t.Error("expected AllVoted true with single participant")
	}

	refetched, _ := fs.GetGameSession(ctx, session.ID)
	if refetched.SuspendedAt != nil {
		t.Error("expected session to be resumed (SuspendedAt nil)")
	}
}

func TestVoteResume_ErrNotSuspended(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "resume2")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "RESU0002", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	_ = fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	// Not suspended — VoteResume should fail.

	_, err := svc.VoteResume(ctx, session.ID, p1.ID)
	if err != runtime.ErrNotSuspended {
		t.Errorf("expected ErrNotSuspended, got %v", err)
	}
}

func TestVoteResume_ErrNotParticipant(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "resume3")
	outsider, _ := fs.CreatePlayer(ctx, "outsider2")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "RESU0003", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	_ = fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	_ = fs.SuspendSession(ctx, session.ID, "pause_vote")

	_, err := svc.VoteResume(ctx, session.ID, outsider.ID)
	if err != runtime.ErrNotParticipant {
		t.Errorf("expected ErrNotParticipant, got %v", err)
	}
}

func TestVoteResume_ErrGameOver(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "resume4")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "RESU0004", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	_ = fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	_ = fs.FinishSession(ctx, session.ID)

	_, err := svc.VoteResume(ctx, session.ID, p1.ID)
	if err != runtime.ErrGameOver {
		t.Errorf("expected ErrGameOver, got %v", err)
	}
}
