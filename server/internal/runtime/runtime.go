package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/engine"
	"github.com/tableforge/server/internal/store"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrGameNotFound    = errors.New("game not found")
	ErrNotYourTurn     = errors.New("it is not your turn")
	ErrGameOver        = errors.New("game is already over")
	ErrInvalidMove     = errors.New("invalid move")
	ErrSuspended       = errors.New("game is suspended")
)

// MoveResult is returned after a move is successfully applied.
type MoveResult struct {
	Session store.GameSession `json:"session"`
	State   engine.GameState  `json:"state"`
	IsOver  bool              `json:"is_over"`
	Result  *engine.Result    `json:"result,omitempty"`
}

// Service processes moves for active game sessions.
type Service struct {
	store    store.Store
	registry engine.Registry
}

// New creates a new runtime Service.
func New(st store.Store, registry engine.Registry) *Service {
	return &Service{store: st, registry: registry}
}

// ApplyMove validates and applies a player move to the given session.
func (svc *Service) ApplyMove(ctx context.Context, sessionID, playerID uuid.UUID, payload map[string]any) (MoveResult, error) {
	// Load session
	session, err := svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return MoveResult{}, ErrSessionNotFound
	}
	if session.FinishedAt != nil {
		return MoveResult{}, ErrGameOver
	}
	if session.SuspendedAt != nil {
		return MoveResult{}, ErrSuspended
	}

	// Load game plugin
	game, err := svc.registry.Get(session.GameID)
	if err != nil {
		return MoveResult{}, ErrGameNotFound
	}

	// Deserialize current state
	var state engine.GameState
	if err := json.Unmarshal(session.State, &state); err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: deserialize state: %w", err)
	}

	// Build move
	move := engine.Move{
		PlayerID:  engine.PlayerID(playerID.String()),
		Payload:   payload,
		Timestamp: time.Now(),
	}

	// Validate
	if err := game.ValidateMove(state, move); err != nil {
		return MoveResult{}, fmt.Errorf("%w: %s", ErrInvalidMove, err.Error())
	}

	// Apply
	newState, err := game.ApplyMove(state, move)
	if err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: apply: %w", err)
	}

	// Marshal payload and new state
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: marshal payload: %w", err)
	}
	stateJSON, err := json.Marshal(newState)
	if err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: marshal state: %w", err)
	}

	// Persist move with state snapshot
	moveNumber := session.MoveCount + 1
	if _, err := svc.store.RecordMove(ctx, store.RecordMoveParams{
		SessionID:  sessionID,
		PlayerID:   playerID,
		Payload:    payloadJSON,
		StateAfter: stateJSON,
		MoveNumber: moveNumber,
	}); err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: record move: %w", err)
	}

	// Persist new state
	if err := svc.store.UpdateSessionState(ctx, sessionID, stateJSON); err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: update state: %w", err)
	}

	// Reload session to get updated move_count
	session, err = svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: reload session: %w", err)
	}

	// Check if game is over
	over, result := game.IsOver(newState)
	if over {
		if err := svc.store.FinishSession(ctx, sessionID); err != nil {
			return MoveResult{}, fmt.Errorf("ApplyMove: finish session: %w", err)
		}
		session.FinishedAt = timePtr(time.Now())

		// Record result and per-player outcomes
		players, _ := svc.store.ListRoomPlayers(ctx, session.RoomID)
		resultParams := buildGameResultParams(session, result, players)
		if _, err := svc.store.CreateGameResult(ctx, resultParams); err != nil {
			// Non-fatal: log but don't fail the move
			fmt.Printf("ApplyMove: record game result: %v\n", err)
		}

		return MoveResult{
			Session: session,
			State:   newState,
			IsOver:  true,
			Result:  &result,
		}, nil
	}

	return MoveResult{
		Session: session,
		State:   newState,
		IsOver:  false,
	}, nil
}

// GetState returns the current deserialized state of a session.
func (svc *Service) GetState(ctx context.Context, sessionID uuid.UUID) (engine.GameState, error) {
	session, err := svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return engine.GameState{}, ErrSessionNotFound
	}

	var state engine.GameState
	if err := json.Unmarshal(session.State, &state); err != nil {
		return engine.GameState{}, fmt.Errorf("GetState: %w", err)
	}

	return state, nil
}

// GetStateAt returns the game state after a specific move number.
func (svc *Service) GetStateAt(ctx context.Context, sessionID uuid.UUID, moveNumber int) (engine.GameState, error) {
	move, err := svc.store.GetMoveAt(ctx, sessionID, moveNumber)
	if err != nil {
		return engine.GameState{}, fmt.Errorf("GetStateAt: %w", err)
	}
	if move.StateAfter == nil {
		return engine.GameState{}, fmt.Errorf("GetStateAt: no snapshot for move %d", moveNumber)
	}

	var state engine.GameState
	if err := json.Unmarshal(move.StateAfter, &state); err != nil {
		return engine.GameState{}, fmt.Errorf("GetStateAt: %w", err)
	}
	return state, nil
}

// --- Helpers -----------------------------------------------------------------

func buildGameResultParams(session store.GameSession, result engine.Result, players []store.RoomPlayer) store.CreateGameResultParams {
	endedBy := "draw"
	var winnerID *uuid.UUID

	switch result.Status {
	case engine.ResultWin:
		endedBy = "win"
		if result.WinnerID != nil {
			id, err := uuid.Parse(string(*result.WinnerID))
			if err == nil {
				winnerID = &id
			}
		}
	case engine.ResultDraw:
		endedBy = "draw"
	}

	resultPlayers := make([]store.GameResultPlayer, len(players))
	for i, p := range players {
		outcome := "loss"
		if result.Status == engine.ResultDraw {
			outcome = "draw"
		} else if winnerID != nil && p.PlayerID == *winnerID {
			outcome = "win"
		}
		resultPlayers[i] = store.GameResultPlayer{
			PlayerID: p.PlayerID,
			Seat:     p.Seat,
			Outcome:  outcome,
		}
	}

	return store.CreateGameResultParams{
		SessionID: session.ID,
		GameID:    session.GameID,
		WinnerID:  winnerID,
		IsDraw:    result.Status == engine.ResultDraw,
		EndedBy:   endedBy,
		Players:   resultPlayers,
	}
}

func timePtr(t time.Time) *time.Time { return &t }
