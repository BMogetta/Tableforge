package tictactoe

import (
	"errors"
	"fmt"

	"github.com/recess/game-server/games"
	"github.com/recess/game-server/internal/domain/engine"
)

func init() {
	games.Register(&TicTacToe{})
}

const gameID = "tictactoe"

// board positions are indexed 0-8, row by row:
//
//	0 | 1 | 2
//	---------
//	3 | 4 | 5
//	---------
//	6 | 7 | 8
var winLines = [8][3]int{
	{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, // rows
	{0, 3, 6}, {1, 4, 7}, {2, 5, 8}, // cols
	{0, 4, 8}, {2, 4, 6}, // diagonals
}

// TicTacToe implements engine.Game.
type TicTacToe struct{}

func (g *TicTacToe) ID() string      { return gameID }
func (g *TicTacToe) Name() string    { return "Tic-tac-toe" }
func (g *TicTacToe) MinPlayers() int { return 2 }
func (g *TicTacToe) MaxPlayers() int { return 2 }

// Init sets up an empty board. Player at seat 0 goes first and plays as "X".
func (g *TicTacToe) Init(players []engine.Player) (engine.GameState, error) {
	if len(players) != 2 {
		return engine.GameState{}, fmt.Errorf("tictactoe requires exactly 2 players, got %d", len(players))
	}

	return engine.GameState{
		CurrentPlayerID: players[0].ID,
		Data: map[string]any{
			"board": [9]string{}, // "" = empty, "X" or "O"
			"marks": map[string]string{ // player ID -> mark
				string(players[0].ID): "X",
				string(players[1].ID): "O",
			},
			"players": []string{string(players[0].ID), string(players[1].ID)},
		},
	}, nil
}

// ValidateMove checks that the move is a valid cell selection.
// The move payload must contain: {"cell": <0-8>}
// Timeout moves ({"timeout_action": "..."}) are always valid for the current player.
func (g *TicTacToe) ValidateMove(state engine.GameState, move engine.Move) error {
	if move.PlayerID != state.CurrentPlayerID {
		return errors.New("it is not your turn")
	}

	if _, ok := move.Payload["timeout_action"]; ok {
		return nil
	}

	cell, err := getCell(move)
	if err != nil {
		return err
	}

	board, err := getBoard(state)
	if err != nil {
		return err
	}

	if board[cell] != "" {
		return fmt.Errorf("cell %d is already taken", cell)
	}

	return nil
}

// ApplyMove places the player's mark on the board and advances the turn.
// Timeout moves skip the mark placement:
//   - lose_turn: advances to the next player without placing a mark
//   - lose_game: marks the game as forfeited (IsOver detects the forfeit)
func (g *TicTacToe) ApplyMove(state engine.GameState, move engine.Move) (engine.GameState, error) {
	if action, ok := move.Payload["timeout_action"].(string); ok {
		return g.applyTimeout(state, move.PlayerID, action)
	}

	cell, err := getCell(move)
	if err != nil {
		return state, err
	}

	board, err := getBoard(state)
	if err != nil {
		return state, err
	}

	marks, err := getMarks(state)
	if err != nil {
		return state, err
	}

	players, err := getPlayers(state)
	if err != nil {
		return state, err
	}

	// Place mark
	board[cell] = marks[string(move.PlayerID)]

	// Advance turn to the other player
	nextPlayerID := players[0]
	if engine.PlayerID(players[0]) == move.PlayerID {
		nextPlayerID = players[1]
	}

	return engine.GameState{
		CurrentPlayerID: engine.PlayerID(nextPlayerID),
		Data: map[string]any{
			"board":   board,
			"marks":   marks,
			"players": players,
		},
	}, nil
}

func (g *TicTacToe) applyTimeout(state engine.GameState, playerID engine.PlayerID, action string) (engine.GameState, error) {
	players, err := getPlayers(state)
	if err != nil {
		return state, err
	}

	nextPlayerID := players[0]
	if engine.PlayerID(players[0]) == playerID {
		nextPlayerID = players[1]
	}

	if action == "lose_game" {
		state.Data["forfeit"] = string(playerID)
		return state, nil
	}

	// lose_turn (default): advance turn without placing a mark
	return engine.GameState{
		CurrentPlayerID: engine.PlayerID(nextPlayerID),
		Data:            state.Data,
	}, nil
}

// IsOver checks for a win, a draw, or a forfeit (timeout lose_game).
func (g *TicTacToe) IsOver(state engine.GameState) (bool, engine.Result) {
	// Forfeit: timed-out player loses, opponent wins.
	if forfeit, ok := state.Data["forfeit"].(string); ok {
		players, err := getPlayers(state)
		if err != nil {
			return false, engine.Result{}
		}
		for _, p := range players {
			if p != forfeit {
				winnerID := engine.PlayerID(p)
				return true, engine.Result{
					Status:   engine.ResultWin,
					WinnerID: &winnerID,
				}
			}
		}
	}

	board, err := getBoard(state)
	if err != nil {
		return false, engine.Result{}
	}

	marks, err := getMarks(state)
	if err != nil {
		return false, engine.Result{}
	}

	players, err := getPlayers(state)
	if err != nil {
		return false, engine.Result{}
	}

	// Check win lines
	for _, line := range winLines {
		a, b, c := board[line[0]], board[line[1]], board[line[2]]
		if a != "" && a == b && b == c {
			// Find which player has this mark
			for _, pid := range players {
				if marks[pid] == a {
					winnerID := engine.PlayerID(pid)
					return true, engine.Result{
						Status:   engine.ResultWin,
						WinnerID: &winnerID,
					}
				}
			}
		}
	}

	// Check draw: all cells filled, no winner
	for _, cell := range board {
		if cell == "" {
			return false, engine.Result{}
		}
	}

	return true, engine.Result{Status: engine.ResultDraw}
}

// TimeoutMove implements engine.TurnTimeoutHandler.
// For lose_turn, the move skips the turn without placing a mark.
// For lose_game, the move forfeits the game — IsOver detects the forfeit.
func (g *TicTacToe) TimeoutMove(penalty string) map[string]any {
	return map[string]any{"timeout_action": penalty}
}

// --- Helpers -----------------------------------------------------------------

func getCell(move engine.Move) (int, error) {
	raw, ok := move.Payload["cell"]
	if !ok {
		return 0, errors.New("move payload must contain 'cell'")
	}

	// JSON numbers decode as float64
	switch v := raw.(type) {
	case float64:
		cell := int(v)
		if cell < 0 || cell > 8 {
			return 0, fmt.Errorf("cell must be between 0 and 8, got %d", cell)
		}
		return cell, nil
	case int:
		if v < 0 || v > 8 {
			return 0, fmt.Errorf("cell must be between 0 and 8, got %d", v)
		}
		return v, nil
	default:
		return 0, fmt.Errorf("cell must be a number, got %T", raw)
	}
}

func getBoard(state engine.GameState) ([9]string, error) {
	raw, ok := state.Data["board"]
	if !ok {
		return [9]string{}, errors.New("state missing 'board'")
	}

	switch v := raw.(type) {
	case [9]string:
		return v, nil
	case []any:
		// After JSON round-trip the array comes back as []any
		if len(v) != 9 {
			return [9]string{}, fmt.Errorf("board must have 9 cells, got %d", len(v))
		}
		var board [9]string
		for i, cell := range v {
			if cell == nil {
				board[i] = ""
			} else if s, ok := cell.(string); ok {
				board[i] = s
			} else {
				return [9]string{}, fmt.Errorf("board cell %d must be a string", i)
			}
		}
		return board, nil
	default:
		return [9]string{}, fmt.Errorf("unexpected board type %T", raw)
	}
}

func getMarks(state engine.GameState) (map[string]string, error) {
	raw, ok := state.Data["marks"]
	if !ok {
		return nil, errors.New("state missing 'marks'")
	}
	switch v := raw.(type) {
	case map[string]string:
		return v, nil
	case map[string]any:
		marks := make(map[string]string, len(v))
		for k, val := range v {
			s, ok := val.(string)
			if !ok {
				return nil, fmt.Errorf("mark for player %s must be a string", k)
			}
			marks[k] = s
		}
		return marks, nil
	default:
		return nil, fmt.Errorf("unexpected marks type %T", raw)
	}
}

func getPlayers(state engine.GameState) ([]string, error) {
	raw, ok := state.Data["players"]
	if !ok {
		return nil, errors.New("state missing 'players'")
	}
	switch v := raw.(type) {
	case []string:
		return v, nil
	case []any:
		players := make([]string, len(v))
		for i, p := range v {
			s, ok := p.(string)
			if !ok {
				return nil, fmt.Errorf("player %d must be a string", i)
			}
			players[i] = s
		}
		return players, nil
	default:
		return nil, fmt.Errorf("unexpected players type %T", raw)
	}
}
