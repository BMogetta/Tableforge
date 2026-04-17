package tictactoe_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/recess/game-server/games/tictactoe"
	"github.com/recess/game-server/internal/domain/engine"
)

var (
	p1 = engine.Player{ID: "player-1", Username: "alice"}
	p2 = engine.Player{ID: "player-2", Username: "bob"}
)

func newGame(t *testing.T) engine.GameState {
	t.Helper()
	g := &tictactoe.TicTacToe{}
	state, err := g.Init([]engine.Player{p1, p2})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	return state
}

func move(playerID engine.PlayerID, cell int) engine.Move {
	return engine.Move{
		PlayerID:  playerID,
		Payload:   map[string]any{"cell": float64(cell)},
		Timestamp: time.Now(),
	}
}

func applyMove(t *testing.T, g *tictactoe.TicTacToe, state engine.GameState, m engine.Move) engine.GameState {
	t.Helper()
	if err := g.ValidateMove(state, m); err != nil {
		t.Fatalf("ValidateMove: %v", err)
	}
	next, err := g.ApplyMove(state, m)
	if err != nil {
		t.Fatalf("ApplyMove: %v", err)
	}
	return next
}

// --- Tests -------------------------------------------------------------------

func TestInit(t *testing.T) {
	state := newGame(t)

	if state.CurrentPlayerID != p1.ID {
		t.Errorf("expected p1 to go first, got %s", state.CurrentPlayerID)
	}
}

func TestTurnAdvances(t *testing.T) {
	g := &tictactoe.TicTacToe{}
	state := newGame(t)

	state = applyMove(t, g, state, move(p1.ID, 0))
	if state.CurrentPlayerID != p2.ID {
		t.Errorf("expected p2's turn after p1 moves, got %s", state.CurrentPlayerID)
	}

	state = applyMove(t, g, state, move(p2.ID, 1))
	if state.CurrentPlayerID != p1.ID {
		t.Errorf("expected p1's turn after p2 moves, got %s", state.CurrentPlayerID)
	}
}

func TestWrongPlayerMove(t *testing.T) {
	g := &tictactoe.TicTacToe{}
	state := newGame(t)

	err := g.ValidateMove(state, move(p2.ID, 0))
	if err == nil {
		t.Error("expected error when wrong player moves")
	}
}

func TestCellAlreadyTaken(t *testing.T) {
	g := &tictactoe.TicTacToe{}
	state := newGame(t)

	state = applyMove(t, g, state, move(p1.ID, 4))

	err := g.ValidateMove(state, move(p2.ID, 4))
	if err == nil {
		t.Error("expected error when cell is already taken")
	}
}

func TestInvalidCell(t *testing.T) {
	g := &tictactoe.TicTacToe{}
	state := newGame(t)

	err := g.ValidateMove(state, move(p1.ID, 9))
	if err == nil {
		t.Error("expected error for cell out of range")
	}
}

func TestWinRow(t *testing.T) {
	g := &tictactoe.TicTacToe{}
	state := newGame(t)

	// p1: 0,1,2 (top row) — p2: 3,4
	moves := []engine.Move{
		move(p1.ID, 0), move(p2.ID, 3),
		move(p1.ID, 1), move(p2.ID, 4),
		move(p1.ID, 2),
	}
	for _, m := range moves {
		state = applyMove(t, g, state, m)
	}

	over, result := g.IsOver(state)
	if !over {
		t.Fatal("expected game to be over")
	}
	if result.Status != engine.ResultWin {
		t.Errorf("expected win, got %s", result.Status)
	}
	if result.WinnerID == nil || *result.WinnerID != p1.ID {
		t.Errorf("expected p1 to win")
	}
}

func TestWinColumn(t *testing.T) {
	g := &tictactoe.TicTacToe{}
	state := newGame(t)

	// p1: 0,3,6 (left col) — p2: 1,2
	moves := []engine.Move{
		move(p1.ID, 0), move(p2.ID, 1),
		move(p1.ID, 3), move(p2.ID, 2),
		move(p1.ID, 6),
	}
	for _, m := range moves {
		state = applyMove(t, g, state, m)
	}

	over, result := g.IsOver(state)
	if !over {
		t.Fatal("expected game to be over")
	}
	if result.WinnerID == nil || *result.WinnerID != p1.ID {
		t.Errorf("expected p1 to win")
	}
}

func TestWinDiagonal(t *testing.T) {
	g := &tictactoe.TicTacToe{}
	state := newGame(t)

	// p1: 0,4,8 (main diagonal) — p2: 1,2
	moves := []engine.Move{
		move(p1.ID, 0), move(p2.ID, 1),
		move(p1.ID, 4), move(p2.ID, 2),
		move(p1.ID, 8),
	}
	for _, m := range moves {
		state = applyMove(t, g, state, m)
	}

	over, result := g.IsOver(state)
	if !over {
		t.Fatal("expected game to be over")
	}
	if result.WinnerID == nil || *result.WinnerID != p1.ID {
		t.Errorf("expected p1 to win")
	}
}

func TestDraw(t *testing.T) {
	g := &tictactoe.TicTacToe{}
	state := newGame(t)

	// Fill the board with no winner:
	// X O X
	// X O O
	// O X X
	moves := []engine.Move{
		move(p1.ID, 0), move(p2.ID, 1),
		move(p1.ID, 2), move(p2.ID, 4),
		move(p1.ID, 3), move(p2.ID, 5),
		move(p1.ID, 7), move(p2.ID, 6),
		move(p1.ID, 8),
	}
	for _, m := range moves {
		state = applyMove(t, g, state, m)
	}

	over, result := g.IsOver(state)
	if !over {
		t.Fatal("expected game to be over")
	}
	if result.Status != engine.ResultDraw {
		t.Errorf("expected draw, got %s", result.Status)
	}
}

func TestNotOverMidGame(t *testing.T) {
	g := &tictactoe.TicTacToe{}
	state := newGame(t)

	state = applyMove(t, g, state, move(p1.ID, 0))
	state = applyMove(t, g, state, move(p2.ID, 1))

	over, _ := g.IsOver(state)
	if over {
		t.Error("game should not be over mid-game")
	}
}

// TestApplyMoveIsPure verifies ApplyMove does not mutate the input state.
// The same state + same move applied twice must produce identical output,
// and the caller's state.Data must remain unchanged between calls.
func TestApplyMoveIsPure(t *testing.T) {
	g := &tictactoe.TicTacToe{}
	state := newGame(t)
	m := move(p1.ID, 4)

	snapshotBoard := state.Data["board"]
	snapshotMarks := state.Data["marks"]
	_, hadForfeit := state.Data["forfeit"]

	s1, err := g.ApplyMove(state, m)
	if err != nil {
		t.Fatalf("first ApplyMove: %v", err)
	}
	s2, err := g.ApplyMove(state, m)
	if err != nil {
		t.Fatalf("second ApplyMove: %v", err)
	}

	if !reflect.DeepEqual(s1.Data["board"], s2.Data["board"]) {
		t.Errorf("ApplyMove not deterministic: board diverged\nfirst=%v\nsecond=%v", s1.Data["board"], s2.Data["board"])
	}
	if s1.CurrentPlayerID != s2.CurrentPlayerID {
		t.Errorf("ApplyMove not deterministic: CurrentPlayerID diverged (%s vs %s)", s1.CurrentPlayerID, s2.CurrentPlayerID)
	}
	if !reflect.DeepEqual(state.Data["board"], snapshotBoard) {
		t.Error("ApplyMove mutated caller's board")
	}
	if !reflect.DeepEqual(state.Data["marks"], snapshotMarks) {
		t.Error("ApplyMove mutated caller's marks")
	}
	if _, now := state.Data["forfeit"]; now != hadForfeit {
		t.Error("ApplyMove leaked a forfeit marker into caller's state")
	}
}

// TestTimeoutLoseGameDoesNotMutateCaller covers the historical bug where
// applyTimeout wrote "forfeit" directly into state.Data, which in Go is a
// reference type — the caller's map saw the mutation.
func TestTimeoutLoseGameDoesNotMutateCaller(t *testing.T) {
	g := &tictactoe.TicTacToe{}
	state := newGame(t)

	m := engine.Move{
		PlayerID:  p1.ID,
		Payload:   map[string]any{"timeout_action": "lose_game"},
		Timestamp: time.Now(),
	}

	next, err := g.ApplyMove(state, m)
	if err != nil {
		t.Fatalf("ApplyMove: %v", err)
	}

	if _, ok := state.Data["forfeit"]; ok {
		t.Error("lose_game leaked a forfeit marker into caller's state.Data")
	}
	if forfeit, ok := next.Data["forfeit"].(string); !ok || forfeit != string(p1.ID) {
		t.Errorf("returned state missing forfeit=%s, got %v", p1.ID, next.Data["forfeit"])
	}
}
