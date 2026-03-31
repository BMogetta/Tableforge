// Package tictactoe provides a bot.BotAdapter implementation for TicTacToe.
// TicTacToe has no hidden information, so Determinize is an identity function
// and RolloutPolicy uses uniform random selection.
package tictactoe

import (
	"fmt"
	"time"

	ttt "github.com/tableforge/game-server/games/tictactoe"
	"github.com/tableforge/game-server/internal/bot"
	"github.com/tableforge/game-server/internal/bot/mcts"
	"github.com/tableforge/game-server/internal/domain/engine"
)

// Adapter implements bot.BotAdapter for TicTacToe.
// ApplyMove delegates directly to the game engine without calling ValidateMove —
// it is the caller's responsibility to only pass moves returned by ValidMoves.
type Adapter struct {
	game *ttt.TicTacToe
}

// New returns a new TicTacToe Adapter.
func New() *Adapter {
	return &Adapter{game: &ttt.TicTacToe{}}
}

// ValidMoves returns one move per empty cell as {"cell":N} JSON payloads.
// Returns nil if the state is terminal.
func (a *Adapter) ValidMoves(s bot.BotGameState) []bot.BotMove {
	state := s.EngineState()

	// Guard: do not enumerate moves on a finished game.
	if over, _ := a.game.IsOver(state); over {
		return nil
	}

	board, err := getBoard(state)
	if err != nil {
		return nil
	}

	moves := make([]bot.BotMove, 0, 9)
	for i, cell := range board {
		if cell == "" {
			m, err := bot.MoveFromPayload(map[string]any{"cell": i})
			if err != nil {
				continue
			}
			moves = append(moves, m)
		}
	}
	return moves
}

// ApplyMove applies the given BotMove and returns the resulting BotGameState.
// Panics if the move payload is malformed — callers must use moves from ValidMoves.
func (a *Adapter) ApplyMove(s bot.BotGameState, m bot.BotMove) bot.BotGameState {
	state := s.EngineState()

	payload, err := m.Payload()
	if err != nil {
		panic("tictactoe adapter: malformed BotMove: " + err.Error())
	}

	move := engine.Move{
		PlayerID:  state.CurrentPlayerID,
		Payload:   payload,
		Timestamp: time.Time{},
	}

	next, err := a.game.ApplyMove(state, move)
	if err != nil {
		panic("tictactoe adapter: ApplyMove failed: " + err.Error())
	}

	return bot.NewBotState(next)
}

// IsTerminal returns true if the game is over.
func (a *Adapter) IsTerminal(s bot.BotGameState) bool {
	over, _ := a.game.IsOver(s.EngineState())
	return over
}

// Result returns the score for playerID at a terminal state.
// 1.0 = win, 0.0 = loss, 0.5 = draw.
// Must only be called when IsTerminal returns true.
func (a *Adapter) Result(s bot.BotGameState, playerID engine.PlayerID) float64 {
	_, result := a.game.IsOver(s.EngineState())
	if result.WinnerID == nil {
		return 0.5 // draw
	}
	if *result.WinnerID == playerID {
		return 1.0
	}
	return 0.0
}

// CurrentPlayer returns the ID of the player whose turn it is.
func (a *Adapter) CurrentPlayer(s bot.BotGameState) engine.PlayerID {
	return s.EngineState().CurrentPlayerID
}

// Determinize is an identity function — TicTacToe has no hidden information.
func (a *Adapter) Determinize(s bot.BotGameState, _ engine.PlayerID) bot.BotGameState {
	return s
}

// RolloutPolicy delegates to uniform random selection.
// TicTacToe has no meaningful heuristic that would justify a guided rollout.
func (a *Adapter) RolloutPolicy(s bot.BotGameState, moves []bot.BotMove) bot.BotMove {
	return mcts.RandomRolloutPolicy(s, moves)
}

// getBoard reads the board from state.Data, handling both the native [9]string
// form and the []any form that results from a JSON round-trip in bot.Clone().
func getBoard(state engine.GameState) ([9]string, error) {
	raw, ok := state.Data["board"]
	if !ok {
		return [9]string{}, fmt.Errorf("tictactoe adapter: state missing 'board'")
	}
	switch v := raw.(type) {
	case [9]string:
		return v, nil
	case []any:
		if len(v) != 9 {
			return [9]string{}, fmt.Errorf("tictactoe adapter: board has %d cells, want 9", len(v))
		}
		var board [9]string
		for i, cell := range v {
			if cell == nil {
				board[i] = ""
			} else if s, ok := cell.(string); ok {
				board[i] = s
			} else {
				return [9]string{}, fmt.Errorf("tictactoe adapter: board cell %d is not a string", i)
			}
		}
		return board, nil
	default:
		return [9]string{}, fmt.Errorf("tictactoe adapter: unexpected board type %T", raw)
	}
}
