package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/domain/engine"
	"github.com/tableforge/server/internal/domain/rating"
	"github.com/tableforge/server/internal/platform/events"
	"github.com/tableforge/server/internal/platform/store"
	"github.com/tableforge/server/internal/platform/ws"
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
	store        store.Store
	registry     engine.Registry
	timer        *TurnTimer
	events       *events.Store
	ratingEngine *rating.Engine
	bots         *botRegistry
}

// New creates a new runtime Service.
func New(st store.Store, registry engine.Registry, ev *events.Store) *Service {
	return &Service{
		store:    st,
		registry: registry,
		events:   ev,
		bots:     newBotRegistry(),
	}
}

// SetTimer attaches a TurnTimer to the service.
// Call this after constructing both Service and TurnTimer to avoid a circular dep.
func (svc *Service) SetTimer(t *TurnTimer) {
	svc.timer = t
}

// SetRatingEngine attaches a rating engine to the service.
// Call this after constructing both Service and rating.Engine.
// When non-nil, ratings are updated after every ranked session.
func (svc *Service) SetRatingEngine(e *rating.Engine) {
	svc.ratingEngine = e
}

// DefaultReadyTimeout is the time players have to confirm asset loading.
// Inject a shorter duration in tests to avoid waiting 60 seconds.
const DefaultReadyTimeout = 60 * time.Second

// StartSession initiates the ready handshake for a newly created session.
// It auto-confirms any registered bots (they live on the server and need no
// asset loading), then either:
//   - calls OnAllReady immediately if all participants are already confirmed
//     (e.g. a bot-only room), or
//   - schedules a ready timer that forfeits players who don't confirm within
//     readyTimeout.
//
// The TurnTimer does NOT start here — it starts in OnAllReady once every
// participant has sent player_ready.
// Call this from the lobby after creating the session.
func (svc *Service) StartSession(ctx context.Context, session store.GameSession, hub *ws.Hub, readyTimeout time.Duration) {
	// Auto-confirm bots — they live on the server and need no asset loading.
	players, err := svc.store.ListRoomPlayers(ctx, session.RoomID)
	if err == nil {
		allReady := false
		for _, p := range players {
			if _, isBot := svc.bots.get(p.PlayerID); isBot {
				ready, _ := svc.store.VoteReady(ctx, session.ID, p.PlayerID)
				if ready {
					allReady = true
				}
			}
		}
		if allReady {
			// All participants confirmed — skip the ready timer and start immediately.
			svc.OnAllReady(ctx, session, hub)
			goto appendEvent
		}
	}

	if svc.timer != nil {
		svc.timer.ScheduleReady(session.ID, readyTimeout)
	}

appendEvent:
	if svc.events != nil {
		svc.events.Append(ctx, session.ID, events.TypeGameStarted, nil, map[string]any{
			"game_id":           session.GameID,
			"turn_timeout_secs": session.TurnTimeoutSecs,
		})
	}
}

// BroadcastMove fans out a MoveResult to all clients in the room.
// For games that implement engine.StateFilter, hub.BroadcastToRoom is used
// to deliver a per-player filtered state to each player and a spectator-filtered
// state to spectator clients. Falls back to a single hub.Broadcast for games
// without private state (e.g. TicTacToe).
//
// players must be the full list of room players (non-spectators). Obtain it
// from store.ListRoomPlayers before calling this method.
func (svc *Service) BroadcastMove(
	ctx context.Context,
	hub *ws.Hub,
	result MoveResult,
	eventType ws.EventType,
	players []store.RoomPlayer,
) {
	game, err := svc.registry.Get(result.Session.GameID)
	if err != nil {
		hub.Broadcast(result.Session.RoomID, ws.Event{Type: eventType, Payload: result})
		return
	}

	sf, ok := game.(engine.StateFilter)
	if !ok {
		hub.Broadcast(result.Session.RoomID, ws.Event{Type: eventType, Payload: result})
		return
	}

	playerIDs := make([]uuid.UUID, len(players))
	for i, p := range players {
		playerIDs[i] = p.PlayerID
	}

	spectatorResult := result
	spectatorResult.State = sf.FilterState(result.State, engine.PlayerID(""))

	hub.BroadcastToRoom(
		result.Session.RoomID,
		playerIDs,
		func(playerID uuid.UUID) ws.Event {
			filtered := result
			filtered.State = sf.FilterState(result.State, engine.PlayerID(playerID.String()))
			return ws.Event{Type: eventType, Payload: filtered}
		},
		ws.Event{Type: eventType, Payload: spectatorResult},
	)
}

// ApplyMove validates and applies a player move to the given session.
func (svc *Service) ApplyMove(ctx context.Context, sessionID, playerID uuid.UUID, payload map[string]any) (MoveResult, error) {
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

	game, err := svc.registry.Get(session.GameID)
	if err != nil {
		return MoveResult{}, ErrGameNotFound
	}

	var state engine.GameState
	if err := json.Unmarshal(session.State, &state); err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: deserialize state: %w", err)
	}

	move := engine.Move{
		PlayerID:  engine.PlayerID(playerID.String()),
		Payload:   payload,
		Timestamp: time.Now(),
	}

	if err := game.ValidateMove(state, move); err != nil {
		return MoveResult{}, fmt.Errorf("%w: %s", ErrInvalidMove, err.Error())
	}

	newState, err := game.ApplyMove(state, move)
	if err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: apply: %w", err)
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: marshal payload: %w", err)
	}
	stateJSON, err := json.Marshal(newState)
	if err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: marshal state: %w", err)
	}

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

	if err := svc.store.UpdateSessionState(ctx, sessionID, stateJSON); err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: update state: %w", err)
	}
	if err := svc.store.TouchLastMoveAt(ctx, sessionID); err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: touch last_move_at: %w", err)
	}

	session, err = svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: reload session: %w", err)
	}

	over, result := game.IsOver(newState)
	if over {
		if svc.timer != nil {
			svc.timer.Cancel(sessionID)
		}

		if err := svc.store.FinishSession(ctx, sessionID); err != nil {
			return MoveResult{}, fmt.Errorf("ApplyMove: finish session: %w", err)
		}
		session.FinishedAt = timePtr(time.Now())

		if err := svc.store.UpdateRoomStatus(ctx, session.RoomID, store.RoomStatusFinished); err != nil {
			fmt.Printf("ApplyMove: finish room: %v\n", err)
		}

		players, _ := svc.store.ListRoomPlayers(ctx, session.RoomID)
		resultParams := buildGameResultParams(session, result, players)
		if _, err := svc.store.CreateGameResult(ctx, resultParams); err != nil {
			fmt.Printf("ApplyMove: record game result: %v\n", err)
		}

		if session.Mode == store.SessionModeRanked {
			var winnerID *uuid.UUID
			if result.WinnerID != nil {
				id, err := uuid.Parse(string(*result.WinnerID))
				if err == nil {
					winnerID = &id
				}
			}
			svc.applyRatings(ctx, session, players, winnerID, result.Status == engine.ResultDraw)
		}

		if svc.events != nil {
			svc.events.Append(ctx, sessionID, events.TypeMoveApplied, &playerID, map[string]any{
				"move_number": moveNumber,
				"payload":     payload,
			})
			svc.events.Append(ctx, sessionID, events.TypeGameOver, nil, map[string]any{
				"winner_id": result.WinnerID,
				"status":    result.Status,
				"ended_by":  "win",
			})
			svc.events.Persist(ctx, sessionID)
		}

		return MoveResult{
			Session: session,
			State:   newState,
			IsOver:  true,
			Result:  &result,
		}, nil
	}

	if svc.events != nil {
		svc.events.Append(ctx, sessionID, events.TypeMoveApplied, &playerID, map[string]any{
			"move_number": moveNumber,
			"payload":     payload,
		})
	}

	if svc.timer != nil {
		svc.timer.Schedule(session)
	}

	return MoveResult{
		Session: session,
		State:   newState,
		IsOver:  false,
	}, nil
}

// GetSessionAndState returns the session, its deserialized state, and optionally
// the game result if the session is finished.
func (svc *Service) GetSessionAndState(ctx context.Context, sessionID uuid.UUID) (store.GameSession, engine.GameState, *store.GameResult, error) {
	session, err := svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return store.GameSession{}, engine.GameState{}, nil, ErrSessionNotFound
	}

	var state engine.GameState
	if err := json.Unmarshal(session.State, &state); err != nil {
		return store.GameSession{}, engine.GameState{}, nil, fmt.Errorf("GetSessionAndState: %w", err)
	}

	var result *store.GameResult
	if session.FinishedAt != nil {
		r, err := svc.store.GetGameResult(ctx, sessionID)
		if err == nil {
			result = &r
		}
	}

	return session, state, result, nil
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

// ErrNotParticipant is returned when the player is not part of the session.
var ErrNotParticipant = errors.New("player is not a participant in this session")

// Surrender forfeits the session on behalf of playerID.
// The opponent is recorded as the winner with ended_by = "forfeit".
func (svc *Service) Surrender(ctx context.Context, sessionID, playerID uuid.UUID) (MoveResult, error) {
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

	players, err := svc.store.ListRoomPlayers(ctx, session.RoomID)
	if err != nil {
		return MoveResult{}, fmt.Errorf("Surrender: list players: %w", err)
	}

	var opponentID *uuid.UUID
	callerFound := false
	for _, p := range players {
		if p.PlayerID == playerID {
			callerFound = true
		} else {
			id := p.PlayerID
			opponentID = &id
		}
	}
	if !callerFound {
		return MoveResult{}, ErrNotParticipant
	}

	if svc.timer != nil {
		svc.timer.Cancel(sessionID)
	}

	if err := svc.store.FinishSession(ctx, sessionID); err != nil {
		return MoveResult{}, fmt.Errorf("Surrender: finish session: %w", err)
	}
	session.FinishedAt = timePtr(time.Now())

	if err := svc.store.UpdateRoomStatus(ctx, session.RoomID, store.RoomStatusFinished); err != nil {
		fmt.Printf("Surrender: finish room: %v\n", err)
	}

	resultPlayers := make([]store.GameResultPlayer, len(players))
	for i, p := range players {
		outcome := "loss"
		if opponentID != nil && p.PlayerID == *opponentID {
			outcome = "win"
		}
		resultPlayers[i] = store.GameResultPlayer{
			PlayerID: p.PlayerID,
			Seat:     p.Seat,
			Outcome:  outcome,
		}
	}

	if _, err := svc.store.CreateGameResult(ctx, store.CreateGameResultParams{
		SessionID: session.ID,
		GameID:    session.GameID,
		WinnerID:  opponentID,
		IsDraw:    false,
		EndedBy:   "forfeit",
		Players:   resultPlayers,
	}); err != nil {
		fmt.Printf("Surrender: record game result: %v\n", err)
	}

	if session.Mode == store.SessionModeRanked {
		svc.applyRatings(ctx, session, players, opponentID, false)
	}

	if svc.events != nil {
		svc.events.Append(ctx, sessionID, events.TypePlayerSurrendered, &playerID, map[string]any{
			"player_id":   playerID.String(),
			"opponent_id": opponentID,
		})
		svc.events.Append(ctx, sessionID, events.TypeGameOver, nil, map[string]any{
			"winner_id": opponentID,
			"status":    "win",
			"ended_by":  "forfeit",
		})
		svc.events.Persist(ctx, sessionID)
	}

	var state engine.GameState
	if err := json.Unmarshal(session.State, &state); err != nil {
		return MoveResult{}, fmt.Errorf("Surrender: deserialize state: %w", err)
	}

	var winnerEngineID *engine.PlayerID
	if opponentID != nil {
		id := engine.PlayerID(opponentID.String())
		winnerEngineID = &id
	}

	return MoveResult{
		Session: session,
		State:   state,
		IsOver:  true,
		Result: &engine.Result{
			Status:   engine.ResultWin,
			WinnerID: winnerEngineID,
		},
	}, nil
}

func (svc *Service) OnAllReady(ctx context.Context, session store.GameSession, hub *ws.Hub) {
	if svc.timer != nil {
		svc.timer.CancelReady(session.ID)
		svc.timer.Schedule(session)
	}
	if err := svc.store.ClearReadyVotes(ctx, session.ID); err != nil {
		log.Printf("OnAllReady: clear ready votes %s: %v", session.ID, err)
	}
	if hub != nil {
		hub.Broadcast(session.RoomID, ws.Event{
			Type:    ws.EventGameReady,
			Payload: map[string]any{"session_id": session.ID},
		})
	}
}

// ErrNotFinished is returned when a rematch is requested on an active session.
var ErrNotFinished = errors.New("game is not finished yet")

// VoteRematch registers a rematch vote for playerID on the given session.
func (svc *Service) VoteRematch(ctx context.Context, sessionID, playerID uuid.UUID) ([]store.RematchVote, int, uuid.UUID, bool, error) {
	session, err := svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return nil, 0, uuid.Nil, false, ErrSessionNotFound
	}
	if session.FinishedAt == nil {
		return nil, 0, uuid.Nil, false, ErrNotFinished
	}

	players, err := svc.store.ListRoomPlayers(ctx, session.RoomID)
	if err != nil {
		return nil, 0, uuid.Nil, false, fmt.Errorf("VoteRematch: list players: %w", err)
	}

	callerFound := false
	for _, p := range players {
		if p.PlayerID == playerID {
			callerFound = true
			break
		}
	}
	if !callerFound {
		return nil, 0, uuid.Nil, false, ErrNotParticipant
	}

	if err := svc.store.UpsertRematchVote(ctx, sessionID, playerID); err != nil {
		return nil, 0, uuid.Nil, false, fmt.Errorf("VoteRematch: upsert vote: %w", err)
	}

	// Auto-vote for any registered bots in the room so they never block rematch consensus.
	for _, p := range players {
		if _, isBot := svc.bots.get(p.PlayerID); isBot {
			_ = svc.store.UpsertRematchVote(ctx, sessionID, p.PlayerID)
		}
	}

	if svc.events != nil {
		svc.events.Append(ctx, sessionID, events.TypeRematchVoted, &playerID, map[string]any{
			"player_id": playerID.String(),
		})
	}

	votes, err := svc.store.ListRematchVotes(ctx, sessionID)
	if err != nil {
		return nil, 0, uuid.Nil, false, fmt.Errorf("VoteRematch: list votes: %w", err)
	}

	totalPlayers := len(players)

	if len(votes) < totalPlayers {
		return votes, totalPlayers, session.RoomID, false, nil
	}

	if err := svc.store.DeleteRematchVotes(ctx, sessionID); err != nil {
		return nil, 0, uuid.Nil, false, fmt.Errorf("VoteRematch: delete votes: %w", err)
	}

	if svc.timer != nil {
		svc.timer.Cancel(sessionID)
	}

	if err := svc.store.UpdateRoomStatus(ctx, session.RoomID, store.RoomStatusWaiting); err != nil {
		return nil, 0, uuid.Nil, false, fmt.Errorf("VoteRematch: reset room status: %w", err)
	}

	return votes, totalPlayers, session.RoomID, true, nil
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

// Ensure ws import is used — hub is accessed via TurnTimer only.
var _ = ws.EventGameOver
