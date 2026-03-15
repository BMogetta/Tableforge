package tictactoe_test

import (
	"context"
	"testing"
	"time"

	ttt "github.com/tableforge/server/games/tictactoe"
	"github.com/tableforge/server/internal/bot"
	adapter "github.com/tableforge/server/internal/bot/adapter/tictactoe"
	botmcts "github.com/tableforge/server/internal/bot/mcts"
	"github.com/tableforge/server/internal/domain/engine"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const (
	playerX engine.PlayerID = "player-x"
	playerO engine.PlayerID = "player-o"
)

// newGame initialises a TicTacToe BotGameState with playerX going first.
func newGame() bot.BotGameState {
	game := &ttt.TicTacToe{}
	state, err := game.Init([]engine.Player{
		{ID: playerX, Username: "X"},
		{ID: playerO, Username: "O"},
	})
	if err != nil {
		panic("newGame: " + err.Error())
	}
	return bot.NewBotState(state)
}

func fastCfg() bot.BotConfig {
	return bot.BotConfig{
		Iterations:       200,
		Determinizations: 1,
		ExplorationC:     1.41,
		MaxThinkTime:     5 * time.Second,
	}
}

func strongCfg() bot.BotConfig {
	return bot.BotConfig{
		Iterations:       2000,
		Determinizations: 1,
		ExplorationC:     1.41,
		MaxThinkTime:     10 * time.Second,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestAdapter_BotVsBot plays a full game between two bots and asserts that:
//  1. The game terminates within the timeout.
//  2. Every move applied passes ValidateMove (legal play throughout).
//  3. The final result is win or draw — never an invalid state.
func TestAdapter_BotVsBot(t *testing.T) {
	a := adapter.New()
	game := &ttt.TicTacToe{}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	state := newGame()
	players := []engine.PlayerID{playerX, playerO}
	turn := 0

	for !a.IsTerminal(state) {
		if ctx.Err() != nil {
			t.Fatal("bot-vs-bot game did not terminate within 30s")
		}

		currentPlayer := players[turn%2]

		move, err := botmcts.Search(ctx, state, currentPlayer, a, fastCfg())
		if err != nil {
			t.Fatalf("turn %d: Search error: %v", turn, err)
		}

		// Assert the move is legal according to the engine.
		payload, err := move.Payload()
		if err != nil {
			t.Fatalf("turn %d: bad BotMove payload: %v", turn, err)
		}
		engineMove := engine.Move{
			PlayerID:  currentPlayer,
			Payload:   payload,
			Timestamp: time.Time{},
		}
		if err := game.ValidateMove(state.EngineState(), engineMove); err != nil {
			t.Fatalf("turn %d: illegal move %q: %v", turn, move, err)
		}

		state = a.ApplyMove(state, move)
		turn++
	}

	// Game must have ended with a valid result.
	over, result := game.IsOver(state.EngineState())
	if !over {
		t.Fatal("IsTerminal returned true but IsOver returned false")
	}
	if result.Status != engine.ResultWin && result.Status != engine.ResultDraw {
		t.Fatalf("unexpected result status %q", result.Status)
	}
}

// TestAdapter_TakesImmediateWin asserts that the bot plays the winning move
// when one is available on the next turn.
//
// Board setup (X to move, X wins by playing cell 2):
//
//	X | X | _
//	---------
//	O | O | _
//	---------
//	_ | _ | _
func TestAdapter_TakesImmediateWin(t *testing.T) {
	a := adapter.New()
	game := &ttt.TicTacToe{}

	// Build the state by applying moves manually.
	state, err := game.Init([]engine.Player{
		{ID: playerX, Username: "X"},
		{ID: playerO, Username: "O"},
	})
	if err != nil {
		t.Fatal(err)
	}

	moves := []struct {
		player engine.PlayerID
		cell   int
	}{
		{playerX, 0}, // X plays cell 0
		{playerO, 3}, // O plays cell 3
		{playerX, 1}, // X plays cell 1
		{playerO, 4}, // O plays cell 4
		// X to move — cell 2 wins the top row.
	}
	for _, m := range moves {
		state, err = game.ApplyMove(state, engine.Move{
			PlayerID:  m.player,
			Payload:   map[string]any{"cell": m.cell},
			Timestamp: time.Time{},
		})
		if err != nil {
			t.Fatalf("setup ApplyMove cell %d: %v", m.cell, err)
		}
	}

	botState := bot.NewBotState(state)

	move, err := botmcts.Search(
		context.Background(),
		botState,
		playerX,
		a,
		strongCfg(),
	)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	payload, err := move.Payload()
	if err != nil {
		t.Fatalf("bad BotMove payload: %v", err)
	}

	cell, ok := payload["cell"]
	if !ok {
		t.Fatal("move payload missing 'cell'")
	}

	// cell comes back as float64 from JSON round-trip.
	cellFloat, ok := cell.(float64)
	if !ok {
		t.Fatalf("cell is %T, want float64", cell)
	}
	if int(cellFloat) != 2 {
		t.Errorf("expected winning move cell=2, got cell=%d", int(cellFloat))
	}
}

// TestAdapter_BlocksImmediateLoss asserts that the bot blocks the opponent's
// winning move when it is the only sensible play.
//
// Board setup (X to move, must block O at cell 5):
//
//	X | _ | _
//	---------
//	O | O | _
//	---------
//	_ | _ | _
func TestAdapter_BlocksImmediateLoss(t *testing.T) {
	a := adapter.New()
	game := &ttt.TicTacToe{}

	state, err := game.Init([]engine.Player{
		{ID: playerX, Username: "X"},
		{ID: playerO, Username: "O"},
	})
	if err != nil {
		t.Fatal(err)
	}

	moves := []struct {
		player engine.PlayerID
		cell   int
	}{
		{playerX, 0}, // X plays cell 0
		{playerO, 3}, // O plays cell 3
		{playerX, 8}, // X plays cell 8 (irrelevant)
		{playerO, 4}, // O plays cell 4 — O threatens cell 5
		// X must play cell 5 to block.
	}
	for _, m := range moves {
		state, err = game.ApplyMove(state, engine.Move{
			PlayerID:  m.player,
			Payload:   map[string]any{"cell": m.cell},
			Timestamp: time.Time{},
		})
		if err != nil {
			t.Fatalf("setup ApplyMove cell %d: %v", m.cell, err)
		}
	}

	botState := bot.NewBotState(state)

	move, err := botmcts.Search(
		context.Background(),
		botState,
		playerX,
		a,
		strongCfg(),
	)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	payload, err := move.Payload()
	if err != nil {
		t.Fatalf("bad BotMove payload: %v", err)
	}

	cellFloat, ok := payload["cell"].(float64)
	if !ok {
		t.Fatalf("cell is %T, want float64", payload["cell"])
	}
	if int(cellFloat) != 5 {
		t.Errorf("expected blocking move cell=5, got cell=%d", int(cellFloat))
	}
}

// TestAdapter_ValidMovesEmptyBoard asserts that all 9 cells are returned on
// an empty board.
func TestAdapter_ValidMovesEmptyBoard(t *testing.T) {
	a := adapter.New()
	state := newGame()

	moves := a.ValidMoves(state)
	if len(moves) != 9 {
		t.Errorf("expected 9 valid moves on empty board, got %d", len(moves))
	}
}

// TestAdapter_ValidMovesTerminalState asserts that no moves are returned for
// a finished game.
func TestAdapter_ValidMovesTerminalState(t *testing.T) {
	a := adapter.New()
	game := &ttt.TicTacToe{}

	// X wins immediately: play top row.
	state, err := game.Init([]engine.Player{
		{ID: playerX, Username: "X"},
		{ID: playerO, Username: "O"},
	})
	if err != nil {
		t.Fatal(err)
	}

	sequence := []struct {
		player engine.PlayerID
		cell   int
	}{
		{playerX, 0}, {playerO, 3},
		{playerX, 1}, {playerO, 4},
		{playerX, 2}, // X wins
	}
	for _, m := range sequence {
		state, err = game.ApplyMove(state, engine.Move{
			PlayerID:  m.player,
			Payload:   map[string]any{"cell": m.cell},
			Timestamp: time.Time{},
		})
		if err != nil {
			t.Fatalf("setup ApplyMove cell %d: %v", m.cell, err)
		}
	}

	botState := bot.NewBotState(state)
	moves := a.ValidMoves(botState)
	if len(moves) != 0 {
		t.Errorf("expected 0 valid moves on terminal state, got %d", len(moves))
	}
}

// TestAdapter_ResultDraw asserts Result returns 0.5 on a drawn game for both players.
func TestAdapter_ResultDraw(t *testing.T) {
	a := adapter.New()
	game := &ttt.TicTacToe{}

	// Construct a known draw:
	// X O X
	// X X O
	// O X O  — wait, that has X winning. Use:
	// X O X
	// O X X  — X wins diag. Use a proper draw:
	// X O X
	// X O O
	// O X X
	state, err := game.Init([]engine.Player{
		{ID: playerX, Username: "X"},
		{ID: playerO, Username: "O"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Draw sequence (verified manually):
	// X O X
	// X O O
	// O X X
	sequence := []struct {
		player engine.PlayerID
		cell   int
	}{
		{playerX, 0}, {playerO, 1},
		{playerX, 2}, {playerO, 4},
		{playerX, 3}, {playerO, 5},
		{playerX, 7}, {playerO, 6},
		{playerX, 8},
	}
	for _, m := range sequence {
		state, err = game.ApplyMove(state, engine.Move{
			PlayerID:  m.player,
			Payload:   map[string]any{"cell": m.cell},
			Timestamp: time.Time{},
		})
		if err != nil {
			t.Fatalf("setup ApplyMove cell %d: %v", m.cell, err)
		}
	}

	botState := bot.NewBotState(state)

	if !a.IsTerminal(botState) {
		t.Fatal("expected terminal state after draw sequence")
	}
	if got := a.Result(botState, playerX); got != 0.5 {
		t.Errorf("Result(playerX) = %.1f, want 0.5", got)
	}
	if got := a.Result(botState, playerO); got != 0.5 {
		t.Errorf("Result(playerO) = %.1f, want 0.5", got)
	}
}
