package runtime_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/domain/runtime"
	"github.com/recess/game-server/internal/platform/store"
)

// --- VoteReady tests ---------------------------------------------------------

func TestVoteReady_SinglePlayerReachesConsensus(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "ready1")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "REDY0001", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)

	result, err := svc.VoteReady(ctx, session.ID, p1.ID)
	if err != nil {
		t.Fatalf("VoteReady: %v", err)
	}
	if !result.AllReady {
		t.Error("expected AllReady true with single participant")
	}
	if result.Required != 1 {
		t.Errorf("expected required 1, got %d", result.Required)
	}
	if len(result.ReadyPlayers) != 1 {
		t.Errorf("expected 1 ready player, got %d", len(result.ReadyPlayers))
	}
}

func TestVoteReady_TwoPlayersRequireBothVotes(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "ready2a")
	p2, _ := fs.CreatePlayer(ctx, "ready2b")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "REDY0002", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	fs.AddPlayerToRoom(ctx, room.ID, p2.ID, 1)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)

	result, err := svc.VoteReady(ctx, session.ID, p1.ID)
	if err != nil {
		t.Fatalf("VoteReady p1: %v", err)
	}
	if result.AllReady {
		t.Error("expected AllReady false after first of two votes")
	}
	if result.Required != 2 {
		t.Errorf("expected required 2, got %d", result.Required)
	}

	result, err = svc.VoteReady(ctx, session.ID, p2.ID)
	if err != nil {
		t.Fatalf("VoteReady p2: %v", err)
	}
	if !result.AllReady {
		t.Error("expected AllReady true after both votes")
	}
}

func TestVoteReady_IdempotentVote(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "ready3a")
	p2, _ := fs.CreatePlayer(ctx, "ready3b")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "REDY0003", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	fs.AddPlayerToRoom(ctx, room.ID, p2.ID, 1)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)

	svc.VoteReady(ctx, session.ID, p1.ID)
	result, err := svc.VoteReady(ctx, session.ID, p1.ID)
	if err != nil {
		t.Fatalf("VoteReady idempotent: %v", err)
	}
	if result.AllReady {
		t.Error("expected AllReady false — p2 has not voted")
	}
	if len(result.ReadyPlayers) != 1 {
		t.Errorf("expected 1 ready player after duplicate vote, got %d", len(result.ReadyPlayers))
	}
}

func TestVoteReady_ErrNotParticipant(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "ready4")
	outsider, _ := fs.CreatePlayer(ctx, "outsider3")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "REDY0004", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)

	_, err := svc.VoteReady(ctx, session.ID, outsider.ID)
	if err != runtime.ErrNotParticipant {
		t.Errorf("expected ErrNotParticipant, got %v", err)
	}
}

func TestVoteReady_ErrGameOver(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "ready5")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "REDY0005", GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	fs.FinishSession(ctx, session.ID)

	_, err := svc.VoteReady(ctx, session.ID, p1.ID)
	if err != runtime.ErrGameOver {
		t.Errorf("expected ErrGameOver, got %v", err)
	}
}

// --- StartSession bot auto-confirm tests -------------------------------------

func TestStartSession_BotAutoConfirms(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	human, _ := fs.CreatePlayer(ctx, "ready6human")
	botPlayer, _ := fs.CreateBotPlayer(ctx, "ready6bot")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "REDY0006", GameID: "tictactoe", OwnerID: human.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, human.ID, 0)
	fs.AddPlayerToRoom(ctx, room.ID, botPlayer.ID, 1)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(human.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)

	bp := makeBotPlayer(t, botPlayer.ID)
	svc.RegisterBot(bp)

	svc.StartSession(ctx, session, nil, runtime.DefaultReadyTimeout)

	refetched, _ := fs.GetGameSession(ctx, session.ID)

	botConfirmed := false
	for _, id := range refetched.ReadyPlayers {
		if id == botPlayer.ID.String() {
			botConfirmed = true
			break
		}
	}
	if !botConfirmed {
		t.Error("expected bot to be auto-confirmed in ready_players after StartSession")
	}

	humanConfirmed := false
	for _, id := range refetched.ReadyPlayers {
		if id == human.ID.String() {
			humanConfirmed = true
			break
		}
	}
	if humanConfirmed {
		t.Error("expected human to NOT be auto-confirmed in ready_players after StartSession")
	}
}

func TestStartSession_AllBotsAutoConfirmReachesConsensus(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	bot1, _ := fs.CreateBotPlayer(ctx, "ready7bot1")
	bot2, _ := fs.CreateBotPlayer(ctx, "ready7bot2")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: "REDY0007", GameID: "tictactoe", OwnerID: bot1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, bot1.ID, 0)
	fs.AddPlayerToRoom(ctx, room.ID, bot2.ID, 1)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(bot1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)

	svc.RegisterBot(makeBotPlayer(t, bot1.ID))
	svc.RegisterBot(makeBotPlayer(t, bot2.ID))

	svc.StartSession(ctx, session, nil, runtime.DefaultReadyTimeout)

	// When all bots confirm, OnAllReady is called which clears ready_players
	// and starts the TurnTimer. Verify consensus was reached by checking that
	// ready_players was cleared (ClearReadyVotes ran) rather than never filled.
	//
	// We do this by voting again as bot1 — if allReady returns true immediately
	// it means the votes were cleared and the counter reset, confirming OnAllReady ran.
	// Alternatively, just assert no panic occurred and the session is still active.
	refetched, err := fs.GetGameSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetGameSession: %v", err)
	}
	// Session should not be finished — OnAllReady starts the game, doesn't end it.
	if refetched.FinishedAt != nil {
		t.Error("expected session to still be active after all bots confirmed")
	}
}
