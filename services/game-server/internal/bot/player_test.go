package bot_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/bot"
	"github.com/recess/game-server/internal/domain/engine"
)

// ---------------------------------------------------------------------------
// Fake SearchFunc implementations
// ---------------------------------------------------------------------------

// fakeSearchOK always returns a fixed BotMove regardless of state.
func fakeSearchOK(_ context.Context, _ bot.BotGameState, _ engine.PlayerID, _ bot.BotAdapter, _ bot.BotConfig) (bot.BotMove, error) {
	m, _ := bot.MoveFromPayload(map[string]any{"cell": float64(4)})
	return m, nil
}

// fakeSearchNoMoves always returns ErrNoMoves.
func fakeSearchNoMoves(_ context.Context, _ bot.BotGameState, _ engine.PlayerID, _ bot.BotAdapter, _ bot.BotConfig) (bot.BotMove, error) {
	return "", bot.ErrNoMoves
}

// fakeSearchCtxCheck returns ctx.Err() if the context is already done,
// otherwise delegates to fakeSearchOK. Used to test deadline propagation.
func fakeSearchCtxCheck(ctx context.Context, root bot.BotGameState, pid engine.PlayerID, a bot.BotAdapter, cfg bot.BotConfig) (bot.BotMove, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return fakeSearchOK(ctx, root, pid, a, cfg)
}

// fakeSearchSlow blocks until ctx is done, then returns ctx.Err().
// Used to verify that DecideMove enforces MaxThinkTime.
func fakeSearchSlow(ctx context.Context, _ bot.BotGameState, _ engine.PlayerID, _ bot.BotAdapter, _ bot.BotConfig) (bot.BotMove, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

// ---------------------------------------------------------------------------
// Stub BotAdapter — minimal implementation, game logic not needed here.
// ---------------------------------------------------------------------------

type stubAdapter struct{}

func (stubAdapter) ValidMoves(_ bot.BotGameState) []bot.BotMove                        { return []bot.BotMove{`{}`} }
func (stubAdapter) ApplyMove(s bot.BotGameState, _ bot.BotMove) bot.BotGameState       { return s }
func (stubAdapter) IsTerminal(_ bot.BotGameState) bool                                 { return false }
func (stubAdapter) Result(_ bot.BotGameState, _ engine.PlayerID) float64               { return 0 }
func (stubAdapter) CurrentPlayer(_ bot.BotGameState) engine.PlayerID                   { return "player-a" }
func (stubAdapter) Determinize(s bot.BotGameState, _ engine.PlayerID) bot.BotGameState { return s }
func (stubAdapter) RolloutPolicy(_ bot.BotGameState, moves []bot.BotMove) bot.BotMove {
	return moves[0]
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestPlayer(search bot.SearchFunc) *bot.BotPlayer {
	cfg := bot.BotConfig{
		MaxThinkTime: 2 * time.Second,
		Iterations:   100,
		ExplorationC: 1.41,
	}
	return &bot.BotPlayer{
		ID:      uuid.New(),
		GameID:  "tictactoe",
		Adapter: stubAdapter{},
		Config:  cfg,
		Search:  search,
	}
}

func emptyState() engine.GameState {
	return engine.GameState{
		CurrentPlayerID: "player-a",
		Data:            map[string]any{},
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestBotPlayer_DecideMove_ReturnsCellPayload asserts that DecideMove returns
// a valid payload map when the search succeeds.
func TestBotPlayer_DecideMove_ReturnsCellPayload(t *testing.T) {
	bp := newTestPlayer(fakeSearchOK)

	payload, err := bp.DecideMove(context.Background(), emptyState())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload == nil {
		t.Fatal("expected non-nil payload")
	}
	cell, ok := payload["cell"]
	if !ok {
		t.Fatal("payload missing 'cell' key")
	}
	cellFloat, ok := cell.(float64)
	if !ok {
		t.Fatalf("cell is %T, want float64", cell)
	}
	if int(cellFloat) != 4 {
		t.Errorf("cell = %d, want 4", int(cellFloat))
	}
}

// TestBotPlayer_DecideMove_ErrNoMoves asserts that ErrNoMoves is propagated
// when the search finds no legal moves.
func TestBotPlayer_DecideMove_ErrNoMoves(t *testing.T) {
	bp := newTestPlayer(fakeSearchNoMoves)

	_, err := bp.DecideMove(context.Background(), emptyState())
	if !errors.Is(err, bot.ErrNoMoves) {
		t.Errorf("expected bot.ErrNoMoves, got %v", err)
	}
}

// TestBotPlayer_DecideMove_ContextCancelledBeforeCall asserts that a
// pre-cancelled context causes DecideMove to return an error immediately.
func TestBotPlayer_DecideMove_ContextCancelledBeforeCall(t *testing.T) {
	bp := newTestPlayer(fakeSearchCtxCheck)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := bp.DecideMove(ctx, emptyState())
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// TestBotPlayer_DecideMove_RespectsMaxThinkTime asserts that DecideMove
// applies MaxThinkTime as a deadline when the caller's ctx has no deadline.
// fakeSearchSlow blocks until ctx is done — if no deadline were applied it
// would block forever.
func TestBotPlayer_DecideMove_RespectsMaxThinkTime(t *testing.T) {
	cfg := bot.BotConfig{
		MaxThinkTime: 50 * time.Millisecond,
		Iterations:   1,
		ExplorationC: 1.41,
	}
	bp := &bot.BotPlayer{
		ID:      uuid.New(),
		GameID:  "tictactoe",
		Adapter: stubAdapter{},
		Config:  cfg,
		Search:  fakeSearchSlow,
	}

	start := time.Now()
	_, err := bp.DecideMove(context.Background(), emptyState())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from slow search with deadline, got nil")
	}
	// Allow generous headroom for slow CI.
	if elapsed > 500*time.Millisecond {
		t.Errorf("DecideMove took %v, want < 500ms with 50ms MaxThinkTime", elapsed)
	}
}

// TestBotPlayer_DecideMove_CallerDeadlineOverrides asserts that when the
// caller passes a context with its own deadline, that deadline is used
// instead of MaxThinkTime.
func TestBotPlayer_DecideMove_CallerDeadlineOverrides(t *testing.T) {
	cfg := bot.BotConfig{
		MaxThinkTime: 10 * time.Second, // large — would hang without caller deadline
		Iterations:   1,
		ExplorationC: 1.41,
	}
	bp := &bot.BotPlayer{
		ID:      uuid.New(),
		GameID:  "tictactoe",
		Adapter: stubAdapter{},
		Config:  cfg,
		Search:  fakeSearchSlow,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := bp.DecideMove(ctx, emptyState())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from slow search with caller deadline, got nil")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("DecideMove took %v, want < 500ms with 50ms caller deadline", elapsed)
	}
}
