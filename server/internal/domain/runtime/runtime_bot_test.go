package runtime_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tableforge/server/games/tictactoe"
	"github.com/tableforge/server/internal/bot"
	botadapter "github.com/tableforge/server/internal/bot/adapter"
	"github.com/tableforge/server/internal/bot/mcts"
	"github.com/tableforge/server/internal/domain/engine"
	"github.com/tableforge/server/internal/domain/runtime"
	"github.com/tableforge/server/internal/platform/store"
	"github.com/tableforge/server/internal/testutil"
)

// --- helpers -----------------------------------------------------------------

func newRuntimeWithStore(t *testing.T) (*runtime.Service, *testutil.FakeStore) {
	t.Helper()
	fs := testutil.NewFakeStore()
	reg := &fakeRegistry{game: &stubTicTacToe{}}
	svc := runtime.New(fs, reg, nil)
	return svc, fs
}

// stubTicTacToe is a minimal game that always returns a non-terminal state.
type stubTicTacToe struct{}

func (g *stubTicTacToe) ID() string      { return "tictactoe" }
func (g *stubTicTacToe) Name() string    { return "TicTacToe" }
func (g *stubTicTacToe) MinPlayers() int { return 2 }
func (g *stubTicTacToe) MaxPlayers() int { return 2 }
func (g *stubTicTacToe) Init(players []engine.Player) (engine.GameState, error) {
	return engine.GameState{CurrentPlayerID: players[0].ID, Data: map[string]any{}}, nil
}
func (g *stubTicTacToe) ValidateMove(_ engine.GameState, _ engine.Move) error { return nil }
func (g *stubTicTacToe) ApplyMove(s engine.GameState, _ engine.Move) (engine.GameState, error) {
	return s, nil
}
func (g *stubTicTacToe) IsOver(_ engine.GameState) (bool, engine.Result) {
	return false, engine.Result{}
}

func makeBotPlayer(t *testing.T, id uuid.UUID) *bot.BotPlayer {
	t.Helper()
	adapter, err := botadapter.New("tictactoe")
	if err != nil {
		t.Fatalf("botadapter.New: %v", err)
	}
	cfg, _ := bot.ConfigFromProfile("easy")
	return &bot.BotPlayer{
		ID:      id,
		GameID:  "tictactoe",
		Adapter: adapter,
		Config:  cfg,
		Search:  mcts.Search,
	}
}

func makeActiveSession(t *testing.T, fs *testutil.FakeStore, playerID uuid.UUID) store.GameSession {
	t.Helper()
	ctx := context.Background()

	owner, _ := fs.CreatePlayer(ctx, "alice")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code:       "TEST0001",
		GameID:     "tictactoe",
		OwnerID:    owner.ID,
		MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, owner.ID, 0)
	fs.AddPlayerToRoom(ctx, room.ID, playerID, 1)

	// Use the real tictactoe game to generate a valid initial state.
	game := &tictactoe.TicTacToe{}
	players := []engine.Player{
		{ID: engine.PlayerID(owner.ID.String())},
		{ID: engine.PlayerID(playerID.String())},
	}
	initState, err := game.Init(players)
	if err != nil {
		t.Fatalf("tictactoe Init: %v", err)
	}
	// Override current player to be the bot.
	initState.CurrentPlayerID = engine.PlayerID(playerID.String())

	stateBytes, _ := json.Marshal(initState)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	return session
}

// --- botRegistry tests -------------------------------------------------------

func TestRegisterAndUnregisterBot(t *testing.T) {
	svc, _ := newRuntimeWithStore(t)

	botID := uuid.New()
	bp := makeBotPlayer(t, botID)

	svc.RegisterBot(bp)
	// If MaybeFireBot does NOT panic, the bot is registered.
	// We verify via unregister — after unregistering, MaybeFireBot is a no-op.
	svc.UnregisterBot(botID)
	// No assertion needed beyond no panic — registry is unexported.
}

func TestRegisterBot_OverwritesExisting(t *testing.T) {
	svc, _ := newRuntimeWithStore(t)

	botID := uuid.New()
	bp1 := makeBotPlayer(t, botID)
	bp2 := makeBotPlayer(t, botID)

	svc.RegisterBot(bp1)
	svc.RegisterBot(bp2) // should not panic or deadlock
	svc.UnregisterBot(botID)
}

func TestUnregisterBot_NonExistentIsNoop(t *testing.T) {
	svc, _ := newRuntimeWithStore(t)
	// Unregistering an ID that was never registered must not panic.
	svc.UnregisterBot(uuid.New())
}

// --- MaybeFireBot tests ------------------------------------------------------

func TestMaybeFireBot_NoopWhenBotNotRegistered(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)

	botID := uuid.New()
	session := makeActiveSession(t, fs, botID)

	// Bot is NOT registered — must be a no-op (no panic, no goroutine side effects).
	svc.MaybeFireBot(context.Background(), nil, session.ID, botID)
	// Give any potential goroutine time to act (there should be none).
	time.Sleep(20 * time.Millisecond)
}

func TestMaybeFireBot_NoopWhenSessionFinished(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	botID := uuid.New()
	session := makeActiveSession(t, fs, botID)

	// Mark session as finished before firing bot.
	fs.FinishSession(ctx, session.ID)

	bp := makeBotPlayer(t, botID)
	svc.RegisterBot(bp)

	// Must not apply any move on a finished session.
	svc.MaybeFireBot(ctx, nil, session.ID, botID)
	time.Sleep(50 * time.Millisecond)

	// Move count must still be 0.
	refetched, _ := fs.GetGameSession(ctx, session.ID)
	if refetched.MoveCount != 0 {
		t.Errorf("expected move_count 0 on finished session, got %d", refetched.MoveCount)
	}
}

func TestMaybeFireBot_NoopWhenSessionSuspended(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	botID := uuid.New()
	session := makeActiveSession(t, fs, botID)

	fs.SuspendSession(ctx, session.ID, "test")

	bp := makeBotPlayer(t, botID)
	svc.RegisterBot(bp)

	svc.MaybeFireBot(ctx, nil, session.ID, botID)
	time.Sleep(50 * time.Millisecond)

	refetched, _ := fs.GetGameSession(ctx, session.ID)
	if refetched.MoveCount != 0 {
		t.Errorf("expected move_count 0 on suspended session, got %d", refetched.MoveCount)
	}
}

func TestMaybeFireBot_NoopWhenNotBotsTurn(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	botID := uuid.New()
	otherPlayerID := uuid.New()

	// Create session where the current player is NOT the bot.
	owner, _ := fs.CreatePlayer(ctx, "alice")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code:       "TEST0002",
		GameID:     "tictactoe",
		OwnerID:    owner.ID,
		MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, owner.ID, 0)
	fs.AddPlayerToRoom(ctx, room.ID, botID, 1)

	// Current player is otherPlayerID, not botID.
	state := engine.GameState{
		CurrentPlayerID: engine.PlayerID(otherPlayerID.String()),
		Data:            map[string]any{},
	}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)

	bp := makeBotPlayer(t, botID)
	svc.RegisterBot(bp)

	svc.MaybeFireBot(ctx, nil, session.ID, botID)
	time.Sleep(50 * time.Millisecond)

	refetched, _ := fs.GetGameSession(ctx, session.ID)
	if refetched.MoveCount != 0 {
		t.Errorf("expected move_count 0 when not bot's turn, got %d", refetched.MoveCount)
	}
}

func TestMaybeFireBot_FiresMove(t *testing.T) {
	svc, fs := newRuntimeWithStore(t)
	ctx := context.Background()

	botID := uuid.New()
	session := makeActiveSession(t, fs, botID)

	bp := makeBotPlayer(t, botID)
	svc.RegisterBot(bp)

	svc.MaybeFireBot(ctx, nil, session.ID, botID)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		refetched, err := fs.GetGameSession(ctx, session.ID)
		t.Logf("move_count=%d err=%v", refetched.MoveCount, err)
		if refetched.MoveCount > 0 {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}

	t.Error("expected bot to apply a move within 3s")
}
