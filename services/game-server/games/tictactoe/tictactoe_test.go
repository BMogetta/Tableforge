package tictactoe_test

import (
	"testing"
	"time"

	"github.com/tableforge/game-server/games/tictactoe"
	"github.com/tableforge/game-server/internal/domain/engine"
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
