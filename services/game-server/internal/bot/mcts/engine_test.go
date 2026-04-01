package mcts_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/recess/game-server/internal/bot"
	"github.com/recess/game-server/internal/bot/mcts"
	"github.com/recess/game-server/internal/domain/engine"
)

// ---------------------------------------------------------------------------
// Misère take-away game
//
// Two players alternate taking 1 or 2 stones from a pile.
// The player who takes the LAST stone loses (misère convention).
//
// Losing positions for the mover (pile ≡ 1 mod 3): 1, 4, 7, 10 …
// From any other pile, optimal play leaves the opponent in a losing position.
// ---------------------------------------------------------------------------

const (
	playerA engine.PlayerID = "player-a"
	playerB engine.PlayerID = "player-b"
)

type takeAwayState struct {
	Pile    int    `json:"pile"`
	Current string `json:"current"`
}

type takeAwayBotState struct {
	s takeAwayState
}

func (b *takeAwayBotState) EngineState() engine.GameState {
	data, _ := json.Marshal(b.s)
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	return engine.GameState{
		CurrentPlayerID: engine.PlayerID(b.s.Current),
		Data:            raw,
	}
}

func (b *takeAwayBotState) Clone() bot.BotGameState {
	return &takeAwayBotState{s: b.s}
}

func newTakeAwayState(pile int, current engine.PlayerID) bot.BotGameState {
	return &takeAwayBotState{s: takeAwayState{Pile: pile, Current: string(current)}}
}

// pileOf extracts the pile value from any BotGameState produced by the
// take-away adapter by reading back through EngineState().Data.
func pileOf(s bot.BotGameState) int {
	raw := s.EngineState().Data
	b, _ := json.Marshal(raw)
	var ts takeAwayState
	_ = json.Unmarshal(b, &ts)
	return ts.Pile
}

// ---------------------------------------------------------------------------
// take-away BotAdapter
// ---------------------------------------------------------------------------

type takeAwayAdapter struct{}

func (a takeAwayAdapter) readState(s bot.BotGameState) takeAwayState {
	raw := s.EngineState().Data
	b, _ := json.Marshal(raw)
	var ts takeAwayState
	_ = json.Unmarshal(b, &ts)
	return ts
}

func (a takeAwayAdapter) ValidMoves(s bot.BotGameState) []bot.BotMove {
	ts := a.readState(s)
	if ts.Pile <= 0 {
		return nil
	}
	moves := []bot.BotMove{`{"take":1}`}
	if ts.Pile >= 2 {
		moves = append(moves, `{"take":2}`)
	}
	return moves
}

func (a takeAwayAdapter) ApplyMove(s bot.BotGameState, m bot.BotMove) bot.BotGameState {
	ts := a.readState(s)
	var payload struct {
		Take int `json:"take"`
	}
	_ = json.Unmarshal([]byte(m), &payload)
	next := takeAwayState{Pile: ts.Pile - payload.Take}
	if ts.Current == string(playerA) {
		next.Current = string(playerB)
	} else {
		next.Current = string(playerA)
	}
	return &takeAwayBotState{s: next}
}

func (a takeAwayAdapter) IsTerminal(s bot.BotGameState) bool {
	return a.readState(s).Pile <= 0
}

// Result: at pile==0, current is the next player to move — meaning the
// opponent just took the last stone, so current player wins.
func (a takeAwayAdapter) Result(s bot.BotGameState, playerID engine.PlayerID) float64 {
	ts := a.readState(s)
	if engine.PlayerID(ts.Current) == playerID {
		return 1.0
	}
	return 0.0
}

func (a takeAwayAdapter) CurrentPlayer(s bot.BotGameState) engine.PlayerID {
	return s.EngineState().CurrentPlayerID
}

// Determinize is an identity — no hidden information in this game.
func (takeAwayAdapter) Determinize(s bot.BotGameState, _ engine.PlayerID) bot.BotGameState {
	return s
}

func (takeAwayAdapter) RolloutPolicy(s bot.BotGameState, moves []bot.BotMove) bot.BotMove {
	return mcts.RandomRolloutPolicy(s, moves)
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

func mediumCfg() bot.BotConfig {
	return bot.BotConfig{
		Iterations:       600,
		Determinizations: 1,
		ExplorationC:     1.41,
		MaxThinkTime:     2 * time.Second,
	}
}

func strongCfg() bot.BotConfig {
	return bot.BotConfig{
		Iterations:       3000,
		Determinizations: 1,
		ExplorationC:     1.41,
		MaxThinkTime:     5 * time.Second,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestSearch_TakesWinningMove asserts that with pile=2 and playerA to move,
// the bot picks take-1.
// take-1 → leaves pile=1 for playerB → playerB is forced to take it → playerB loses.
// take-2 → playerA takes last stone → playerA loses.
func TestSearch_TakesWinningMove(t *testing.T) {
	move, err := mcts.Search(
		context.Background(),
		newTakeAwayState(2, playerA),
		playerA,
		takeAwayAdapter{},
		mediumCfg(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if move != `{"take":1}` {
		t.Errorf("expected take-1 (winning move), got %q", move)
	}
}

// TestSearch_NoMoves asserts ErrNoMoves when the pile is already empty.
func TestSearch_NoMoves(t *testing.T) {
	_, err := mcts.Search(
		context.Background(),
		newTakeAwayState(0, playerA),
		playerA,
		takeAwayAdapter{},
		mediumCfg(),
	)
	if err != mcts.ErrNoMoves {
		t.Errorf("expected mcts.ErrNoMoves, got %v", err)
	}
}

// TestSearch_ContextCancelledBeforeStart asserts that a pre-cancelled context
// causes Search to return a fallback move without error.
func TestSearch_ContextCancelledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	move, err := mcts.Search(
		ctx,
		newTakeAwayState(5, playerA),
		playerA,
		takeAwayAdapter{},
		mediumCfg(),
	)
	if err != nil {
		t.Fatalf("expected fallback move, got error: %v", err)
	}
	if move == "" {
		t.Error("expected non-empty fallback move")
	}
}

// TestSearch_RespectsDeadline asserts that Search returns within a tight
// deadline even when given an enormous iteration count.
func TestSearch_RespectsDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	cfg := mediumCfg()
	cfg.Iterations = 1_000_000 // intentionally huge — deadline must cut it short

	start := time.Now()
	_, err := mcts.Search(ctx, newTakeAwayState(10, playerA), playerA, takeAwayAdapter{}, cfg)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Generous headroom for slow CI.
	if elapsed > 500*time.Millisecond {
		t.Errorf("Search took %v, want < 500ms with 50ms deadline", elapsed)
	}
}

// TestSearch_SmokeTest runs Search on a non-trivial pile and asserts only
// that no error is returned and the move is legal.
func TestSearch_SmokeTest(t *testing.T) {
	move, err := mcts.Search(
		context.Background(),
		newTakeAwayState(6, playerA),
		playerA,
		takeAwayAdapter{},
		mediumCfg(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if move != `{"take":1}` && move != `{"take":2}` {
		t.Errorf("unexpected move %q", move)
	}
}

// TestSearch_WinningMoves_Table exercises known winning positions to validate
// overall MCTS strength. We assert on the resulting pile rather than the
// exact move string so the test is robust to tie-breaking order.
// Uses strongCfg (3000 iterations) so the search reliably finds the optimum.
//
// Losing positions for the mover (pile ≡ 1 mod 3): 1, 4, 7, 10 …
// Optimal play from a winning position must leave the opponent in one of these.
func TestSearch_WinningMoves_Table(t *testing.T) {
	cases := []struct {
		pile         int
		wantPileLeft int // pile after optimal move; must be ≡ 1 (mod 3)
	}{
		{pile: 2, wantPileLeft: 1}, // take-1 → pile=1
		{pile: 3, wantPileLeft: 1}, // take-2 → pile=1
		{pile: 5, wantPileLeft: 4}, // take-1 → pile=4
		{pile: 6, wantPileLeft: 4}, // take-2 → pile=4
	}

	adapter := takeAwayAdapter{}

	for _, tc := range cases {
		tc := tc
		t.Run(string(rune('0'+tc.pile)), func(t *testing.T) {
			state := newTakeAwayState(tc.pile, playerA)
			move, err := mcts.Search(
				context.Background(),
				state,
				playerA,
				adapter,
				strongCfg(),
			)
			if err != nil {
				t.Fatalf("pile=%d: unexpected error: %v", tc.pile, err)
			}
			next := adapter.ApplyMove(state, move)
			gotPile := pileOf(next)
			if gotPile != tc.wantPileLeft {
				t.Errorf("pile=%d: move %q left pile=%d, want pile=%d",
					tc.pile, move, gotPile, tc.wantPileLeft)
			}
		})
	}
}
