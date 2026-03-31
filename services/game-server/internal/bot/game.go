// Package bot provides an IS-MCTS (Information Set Monte Carlo Tree Search)
// engine for game-playing bots. The engine is game-agnostic — each game
// implements BotAdapter to plug into the search.
package bot

import (
	"github.com/tableforge/game-server/internal/domain/engine"
)

// BotGameState is a snapshot of a game at a point in time.
// It wraps engine.GameState and adds Clone(), which is required by the MCTS
// engine to branch the state thousands of times without aliasing.
type BotGameState interface {
	// Clone returns a fully independent deep copy of this state.
	// No references (maps, slices, pointers) may be shared between the
	// original and the clone — aliasing causes silent race conditions in
	// parallel MCTS.
	Clone() BotGameState

	// EngineState returns the underlying engine.GameState.
	// Adapters use this to read game-specific fields from Data.
	EngineState() engine.GameState
}

// BotMove is an opaque move identifier.
// It is the JSON-serialized form of the move payload map[string]any,
// making it comparable and usable as a map key.
// Example: `{"card":"guard","target_player_id":"...","guess":"priest"}`
type BotMove string

// BotAdapter is the bridge between the MCTS engine and a specific game.
// Each game must implement this interface to enable bot support.
//
// Contract for Search callers: always check IsTerminal before calling
// Search. Search returns ErrNoMoves if ValidMoves returns empty, but
// the canonical guard is IsTerminal.
type BotAdapter interface {
	// ValidMoves returns all legal moves for the current player in state s.
	// The returned slice must be non-nil and non-empty for non-terminal states.
	ValidMoves(s BotGameState) []BotMove

	// ApplyMove applies move m to state s and returns the resulting new state.
	// Must NOT mutate s — always return a new BotGameState.
	ApplyMove(s BotGameState, m BotMove) BotGameState

	// IsTerminal returns true if the game is over (win, loss, draw, or no
	// legal moves remain).
	IsTerminal(s BotGameState) bool

	// Result returns a score in [0.0, 1.0] for playerID at a terminal state.
	// Canonical values: 1.0 = win, 0.0 = loss, 0.5 = draw.
	// Called only when IsTerminal returns true.
	Result(s BotGameState, playerID engine.PlayerID) float64

	// CurrentPlayer returns the ID of the player whose turn it is.
	CurrentPlayer(s BotGameState) engine.PlayerID

	// Determinize fills in hidden information (e.g. opponent's hand in Love
	// Letter) with a random sample consistent with what observer can legally
	// know. For games with no hidden information (TicTacToe), this is an
	// identity function that returns s unchanged.
	Determinize(s BotGameState, observer engine.PlayerID) BotGameState

	// RolloutPolicy selects a move during a simulation rollout.
	// moves is guaranteed non-empty when called.
	// The default (random) implementation is sufficient for most games;
	// adapters may override this for guided (heuristic) playouts.
	// The Personality field in BotConfig can inform heuristic weight.
	RolloutPolicy(s BotGameState, moves []BotMove) BotMove
}
