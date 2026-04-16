// Package engine defines the core interfaces and types for the Recess game engine.
// Any board game can be added as a plugin by implementing the Game interface.
package engine

import "time"

// PlayerID is a unique identifier for a player.
type PlayerID string

// Player represents a participant in a game session.
type Player struct {
	ID       PlayerID `json:"id"`
	Username string   `json:"username"`
}

// Move represents an action taken by a player during their turn.
// The Payload field is game-specific and interpreted by each Game implementation.
type Move struct {
	PlayerID  PlayerID       `json:"player_id"`
	Payload   map[string]any `json:"payload"`
	Timestamp time.Time      `json:"timestamp"`
}

// GameState holds the full state of an in-progress game.
// The Data field is game-specific and opaque to the engine.
type GameState struct {
	CurrentPlayerID PlayerID       `json:"current_player_id"`
	Data            map[string]any `json:"data"`
}

// ResultStatus represents the outcome of a completed game.
type ResultStatus string

const (
	ResultWin  ResultStatus = "win"
	ResultDraw ResultStatus = "draw"
)

// Result is returned when a game is over.
type Result struct {
	Status   ResultStatus `json:"status"`
	WinnerID *PlayerID    `json:"winner_id,omitempty"`
	// EndedBy is set by the runtime (not by game plugins) to communicate the
	// game-over reason to WS clients. Plugins leave it empty.
	EndedBy string `json:"ended_by,omitempty"`
}

// Game is the interface every board game plugin must implement.
// The engine calls these methods to manage the game lifecycle without
// knowing anything about the specific rules.
type Game interface {
	// ID returns a unique string identifier for the game type (e.g. "chess", "checkers").
	ID() string

	// Name returns a human-readable name for the game.
	Name() string

	// MinPlayers returns the minimum number of players required.
	MinPlayers() int

	// MaxPlayers returns the maximum number of players allowed.
	MaxPlayers() int

	// Init sets up the initial game state for the given players.
	Init(players []Player) (GameState, error)

	// ValidateMove checks whether a move is legal given the current state.
	// Returns an error describing why the move is invalid, or nil if valid.
	ValidateMove(state GameState, move Move) error

	// ApplyMove applies a validated move and returns the new game state.
	ApplyMove(state GameState, move Move) (GameState, error)

	// IsOver checks whether the game has ended.
	// Returns (true, result) if the game is over, (false, {}) otherwise.
	IsOver(state GameState) (bool, Result)
}

// Registry is a read-only store of available games.
// Implement this interface to provide game lookup to the lobby.
type Registry interface {
	Get(id string) (Game, error)
}

// StateFilter is an optional interface a Game can implement when different
// players must receive different views of the same game state.
// If a game does not implement this interface, the full state is broadcast
// to all players unchanged.
//
// FilterState must return a state safe to send to playerID:
//   - strip other players' hands from the returned state
//   - strip chancellor_choices when playerID is not the active player
//   - spectators pass playerID == "" and receive a view with all hands empty
type StateFilter interface {
	FilterState(state GameState, playerID PlayerID) GameState
}

// TurnTimeoutHandler is an optional interface a Game can implement to provide
// a custom move payload when a player's turn times out.
// The timer calls ApplyMove with the returned payload instead of applying
// the platform-level penalty directly.
// The move is applied on behalf of the timed-out player (state.CurrentPlayerID).
//
// penalty is the platform-level timeout penalty from the game config
// (e.g. "lose_turn", "lose_game"). Games may use it to vary behavior or
// ignore it and always apply their own semantics.
type TurnTimeoutHandler interface {
	TimeoutMove(penalty string) map[string]any
}
