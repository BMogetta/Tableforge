package tictactoe_test

import (
	"context"
	"testing"
	"time"

	"github.com/recess/game-server/games/tictactoe"
	"github.com/recess/game-server/internal/bot"
	botmcts "github.com/recess/game-server/internal/bot/mcts"
	"github.com/recess/game-server/internal/domain/engine"
)

const (
	playerX engine.PlayerID = "player-x"
	playerO engine.PlayerID = "player-o"
)

// newBotState initialises a TicTacToe BotGameState with playerX going first.
// Named distinctly from the engine-level newGame helper in tictactoe_test.go.
func newBotState() bot.BotGameState {
	game := &tictactoe.TicTacToe{}
	state, err := game.Init([]engine.Player{
		{ID: playerX, Username: "X"},
		{ID: playerO, Username: "O"},
	})
	if err != nil {
		panic("newBotState: " + err.Error())
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

// TestAdapter_BotVsBot plays a full game between two bots and asserts that:
//  1. The game terminates within the timeout.
//  2. Every move applied passes ValidateMove (legal play throughout).
//  3. The final result is win or draw — never an invalid state.
func TestAdapter_BotVsBot(t *testing.T) {
	a := tictactoe.NewAdapter()
	game := &tictactoe.TicTacToe{}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	state := newBotState()
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

	over, result := game.IsOver(state.EngineState())
	if !over {
		t.Fatal("IsTerminal returned true but IsOver returned false")
	}
	if result.Status != engine.ResultWin && result.Status != engine.ResultDraw {
		t.Fatalf("unexpected result status %q", result.Status)
	}
}

// TestAdapter_TakesImmediateWin asserts the bot plays the winning move when
// one is available on the next turn. Board (X to move, X wins by playing cell 2):
//
//	X | X | _
//	---------
//	O | O | _
//	---------
//	_ | _ | _
func TestAdapter_TakesImmediateWin(t *testing.T) {
	a := tictactoe.NewAdapter()
	game := &tictactoe.TicTacToe{}

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
		{playerX, 0},
		{playerO, 3},
		{playerX, 1},
		{playerO, 4},
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

	move, err := botmcts.Search(context.Background(), botState, playerX, a, strongCfg())
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
	cellFloat, ok := cell.(float64)
	if !ok {
		t.Fatalf("cell is %T, want float64", cell)
	}
	if int(cellFloat) != 2 {
		t.Errorf("expected winning move cell=2, got cell=%d", int(cellFloat))
	}
}

// TestAdapter_BlocksImmediateLoss asserts the bot blocks the opponent's
// winning move when it is the only sensible play. Board (X to move, must
// block O at cell 5):
//
//	X | _ | _
//	---------
//	O | O | _
//	---------
//	_ | _ | _
func TestAdapter_BlocksImmediateLoss(t *testing.T) {
	a := tictactoe.NewAdapter()
	game := &tictactoe.TicTacToe{}

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
		{playerX, 0},
		{playerO, 3},
		{playerX, 8},
		{playerO, 4},
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

	move, err := botmcts.Search(context.Background(), botState, playerX, a, strongCfg())
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

func TestAdapter_ValidMovesEmptyBoard(t *testing.T) {
	a := tictactoe.NewAdapter()
	state := newBotState()

	moves := a.ValidMoves(state)
	if len(moves) != 9 {
		t.Errorf("expected 9 valid moves on empty board, got %d", len(moves))
	}
}

func TestAdapter_ValidMovesTerminalState(t *testing.T) {
	a := tictactoe.NewAdapter()
	game := &tictactoe.TicTacToe{}

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
		{playerX, 2},
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
// Drawn position:
//
//	X | O | X
//	X | O | O
//	O | X | X
func TestAdapter_ResultDraw(t *testing.T) {
	a := tictactoe.NewAdapter()
	game := &tictactoe.TicTacToe{}

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
