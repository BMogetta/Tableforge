// BotAdapter implementation for TicTacToe — co-located with the game plugin so
// the package is a single self-contained unit (engine + rules + bot adapter).
// Registration happens in init() via adapter.Register.
//
// TicTacToe has no hidden information, so Determinize is an identity function
// and RolloutPolicy uses uniform random selection.

package tictactoe

import (
	"time"

	"github.com/recess/game-server/internal/bot"
	botadapter "github.com/recess/game-server/internal/bot/adapter"
	"github.com/recess/game-server/internal/bot/mcts"
	"github.com/recess/game-server/internal/domain/engine"
)

func init() {
	botadapter.Register(gameID, botadapter.Factory{
		New: func() bot.BotAdapter { return NewAdapter() },
	})
}

// Adapter implements bot.BotAdapter for TicTacToe. ApplyMove delegates
// directly to the game engine without calling ValidateMove — callers must
// only pass moves returned by ValidMoves.
type Adapter struct {
	game *TicTacToe
}

// NewAdapter returns a fresh TicTacToe bot adapter.
func NewAdapter() *Adapter {
	return &Adapter{game: &TicTacToe{}}
}

// ValidMoves returns one move per empty cell as {"cell":N} JSON payloads.
// Returns nil if the state is terminal.
func (a *Adapter) ValidMoves(s bot.BotGameState) []bot.BotMove {
	state := s.EngineState()

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
// 1.0 = win, 0.0 = loss, 0.5 = draw. Must only be called when IsTerminal is true.
func (a *Adapter) Result(s bot.BotGameState, playerID engine.PlayerID) float64 {
	_, result := a.game.IsOver(s.EngineState())
	if result.WinnerID == nil {
		return 0.5
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

// RolloutPolicy delegates to uniform random selection. TicTacToe has no
// meaningful heuristic that would justify a guided rollout.
func (a *Adapter) RolloutPolicy(s bot.BotGameState, moves []bot.BotMove) bot.BotMove {
	return mcts.RandomRolloutPolicy(s, moves)
}
