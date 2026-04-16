package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/platform/events"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/game-server/internal/platform/ws"
	sharedEvents "github.com/recess/shared/events"
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
	timer    Timer
	events   *events.Store
	rdb      *redis.Client
	bots     *botRegistry
	log      *slog.Logger
}

// New creates a new runtime Service.
func New(st store.Store, registry engine.Registry, ev *events.Store, rdb *redis.Client, log *slog.Logger) *Service {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Service{
		store:    st,
		registry: registry,
		events:   ev,
		rdb:      rdb,
		bots:     newBotRegistry(),
		log:      log,
	}
}

// SetTimer attaches a Timer to the service.
// Call this after constructing both Service and Timer to avoid a circular dep.
func (svc *Service) SetTimer(t Timer) {
	svc.timer = t
}

// DefaultReadyTimeout is the time players have to confirm asset loading.
// Inject a shorter duration in tests to avoid waiting 60 seconds.
const DefaultReadyTimeout = 60 * time.Second

// StartSession initiates the ready handshake for a newly created session.
func (svc *Service) StartSession(ctx context.Context, session store.GameSession, hub *ws.Hub, readyTimeout time.Duration) {
	players, err := svc.store.ListRoomPlayers(ctx, session.RoomID)
	if err == nil {
		hasHuman := false
		for _, p := range players {
			if svc.isBot(ctx, p.PlayerID) {
				svc.store.VoteReady(ctx, session.ID, p.PlayerID)
			} else {
				hasHuman = true
			}
		}
		// If no humans, all bots are ready — start immediately.
		// If there are humans, let their VoteReady call trigger consensus.
		// Evaluating here would let bot auto-confirms satisfy the threshold
		// before any human has confirmed.
		if !hasHuman {
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
	over, result := game.IsOver(newState)

	// Persist all DB writes atomically: move recording, state update,
	// and (if game over) session finish + room status + game result.
	var gameResult store.GameResult
	if err := svc.store.WithTx(ctx, func(tx store.Store) error {
		if _, err := tx.RecordMove(ctx, store.RecordMoveParams{
			SessionID:  sessionID,
			PlayerID:   playerID,
			Payload:    payloadJSON,
			StateAfter: stateJSON,
			MoveNumber: moveNumber,
		}); err != nil {
			return fmt.Errorf("record move: %w", err)
		}
		if err := tx.UpdateSessionState(ctx, sessionID, stateJSON); err != nil {
			return fmt.Errorf("update state: %w", err)
		}
		if err := tx.TouchLastMoveAt(ctx, sessionID); err != nil {
			return fmt.Errorf("touch last_move_at: %w", err)
		}

		if over {
			if err := tx.FinishSession(ctx, sessionID); err != nil {
				return fmt.Errorf("finish session: %w", err)
			}
			// Non-fatal: stale room status is a minor inconsistency.
			if err := tx.UpdateRoomStatus(ctx, session.RoomID, store.RoomStatusFinished); err != nil {
				svc.log.Error("ApplyMove: update room status", "room_id", session.RoomID, "error", err)
			}
			players, err := tx.ListRoomPlayers(ctx, session.RoomID)
			if err != nil {
				return fmt.Errorf("list room players: %w", err)
			}
			resultParams := buildGameResultParams(session, result, players)
			gameResult, err = tx.CreateGameResult(ctx, resultParams)
			if err != nil {
				return fmt.Errorf("create game result: %w", err)
			}
		}
		return nil
	}); err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: %w", err)
	}

	// Reload session after commit to get updated state/move_count/finished_at.
	session, err = svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return MoveResult{}, fmt.Errorf("ApplyMove: reload session: %w", err)
	}

	if over {
		if svc.timer != nil {
			svc.timer.Cancel(sessionID)
		}

		players, _ := svc.store.ListRoomPlayers(ctx, session.RoomID)
		if session.Mode == store.SessionModeRanked {
			svc.publishGameFinished(ctx, session, players, gameResult)
		}

		if svc.events != nil {
			svc.events.Append(ctx, sessionID, events.TypeMoveApplied, &playerID, map[string]any{
				"move_number": moveNumber,
				"payload":     payload,
			})
			svc.events.Append(ctx, sessionID, events.TypeGameOver, nil, map[string]any{
				"winner_id": result.WinnerID,
				"status":    result.Status,
				"ended_by":  store.EndedByNormal,
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

// FilterState applies the game's StateFilter (if any) to produce a view of the
// state safe to send to the given player. Games without StateFilter return the
// state unchanged. This should be called before writing state to HTTP responses.
func (svc *Service) FilterState(gameID string, state engine.GameState, playerID uuid.UUID) engine.GameState {
	game, err := svc.registry.Get(gameID)
	if err != nil {
		return state
	}
	sf, ok := game.(engine.StateFilter)
	if !ok {
		return state
	}
	return sf.FilterState(state, engine.PlayerID(playerID.String()))
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

// GetStateAt returns the game state after a specific move number
// by replaying all moves from the beginning up to moveNumber.
func (svc *Service) GetStateAt(ctx context.Context, sessionID uuid.UUID, moveNumber int) (engine.GameState, error) {
	session, err := svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return engine.GameState{}, fmt.Errorf("GetStateAt: %w", err)
	}

	eng, err := svc.registry.Get(session.GameID)
	if err != nil {
		return engine.GameState{}, fmt.Errorf("GetStateAt: game not found: %w", err)
	}

	roomPlayers, err := svc.store.ListRoomPlayers(ctx, session.RoomID)
	if err != nil {
		return engine.GameState{}, fmt.Errorf("GetStateAt: list room players: %w", err)
	}
	players := make([]engine.Player, len(roomPlayers))
	for i, rp := range roomPlayers {
		players[i] = engine.Player{ID: engine.PlayerID(rp.PlayerID.String())}
	}

	state, err := eng.Init(players)
	if err != nil {
		return engine.GameState{}, fmt.Errorf("GetStateAt: init: %w", err)
	}

	moves, err := svc.store.ListSessionMoves(ctx, sessionID, 200, 0)
	if err != nil {
		return engine.GameState{}, fmt.Errorf("GetStateAt: list moves: %w", err)
	}

	for _, m := range moves {
		if m.MoveNumber > moveNumber {
			break
		}
		var p map[string]any
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			return engine.GameState{}, fmt.Errorf("GetStateAt: unmarshal move %d: %w", m.MoveNumber, err)
		}
		state, err = eng.ApplyMove(state, engine.Move{
			PlayerID: engine.PlayerID(m.PlayerID.String()),
			Payload:  p,
		})
		if err != nil {
			return engine.GameState{}, fmt.Errorf("GetStateAt: apply move %d: %w", m.MoveNumber, err)
		}
	}

	return state, nil
}

// ErrNotParticipant is returned when the player is not part of the session.
var ErrNotParticipant = errors.New("player is not a participant in this session")

// Surrender forfeits the session on behalf of playerID.
func (svc *Service) Surrender(ctx context.Context, sessionID, playerID uuid.UUID) (MoveResult, error) {
	session, err := svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return MoveResult{}, ErrSessionNotFound
	}
	if session.FinishedAt != nil {
		return MoveResult{}, ErrGameOver
	}
	// Surrender is allowed even when the session is paused — a player should
	// always be able to forfeit and leave.

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
		svc.log.Error("Surrender: finish room", "room_id", session.RoomID, "error", err)
	}

	resultPlayers := make([]store.GameResultPlayer, len(players))
	for i, p := range players {
		outcome := store.OutcomeLoss
		if opponentID != nil && p.PlayerID == *opponentID {
			outcome = store.OutcomeWin
		}
		resultPlayers[i] = store.GameResultPlayer{
			PlayerID: p.PlayerID,
			Seat:     p.Seat,
			Outcome:  outcome,
		}
	}

	gameResult, err := svc.store.CreateGameResult(ctx, store.CreateGameResultParams{
		SessionID: session.ID,
		GameID:    session.GameID,
		WinnerID:  opponentID,
		IsDraw:    false,
		EndedBy:   store.EndedByForfeit,
		Players:   resultPlayers,
	})
	if err != nil {
		svc.log.Error("Surrender: record game result", "session_id", sessionID, "error", err)
	}

	if session.Mode == store.SessionModeRanked {
		svc.publishGameFinished(ctx, session, players, gameResult)
	}

	if svc.events != nil {
		svc.events.Append(ctx, sessionID, events.TypePlayerSurrendered, &playerID, map[string]any{
			"player_id":   playerID.String(),
			"opponent_id": opponentID,
		})
		svc.events.Append(ctx, sessionID, events.TypeGameOver, nil, map[string]any{
			"winner_id": opponentID,
			"status":    store.OutcomeWin,
			"ended_by":  store.EndedByForfeit,
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

// LoadState replaces the persisted GameState of an active session with the
// provided raw bytes. Dev-only — callers gate access (e.g. behind
// ALLOW_DEBUG_SCENARIOS). The state is parsed once to validate it is
// syntactically a GameState; field-level semantics (per-game schema) are
// trusted to the caller. Active turn timers are cancelled so the loaded
// state's current player has a fresh turn window.
func (svc *Service) LoadState(ctx context.Context, sessionID uuid.UUID, rawState []byte) (MoveResult, error) {
	session, err := svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return MoveResult{}, ErrSessionNotFound
	}
	if session.FinishedAt != nil {
		return MoveResult{}, ErrGameOver
	}

	var state engine.GameState
	if err := json.Unmarshal(rawState, &state); err != nil {
		return MoveResult{}, fmt.Errorf("LoadState: parse state: %w", err)
	}

	if svc.timer != nil {
		svc.timer.Cancel(sessionID)
	}

	if err := svc.store.UpdateSessionState(ctx, sessionID, rawState); err != nil {
		return MoveResult{}, fmt.Errorf("LoadState: persist state: %w", err)
	}
	session.State = rawState

	if svc.timer != nil {
		svc.timer.Schedule(session)
	}

	return MoveResult{
		Session: session,
		State:   state,
		IsOver:  false,
	}, nil
}

func (svc *Service) OnAllReady(ctx context.Context, session store.GameSession, hub *ws.Hub) {
	if svc.timer != nil {
		svc.timer.CancelReady(session.ID)
		svc.timer.Schedule(session)
	}
	if err := svc.store.ClearReadyVotes(ctx, session.ID); err != nil {
		svc.log.Error("OnAllReady: clear ready votes", "session_id", session.ID, "error", err)
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

// ErrRematchNotAllowedRanked is returned when a player tries to rematch a
// finished ranked session. Ranked matches must go back through matchmaking so
// MMR pairs are recomputed — a same-room rematch would bypass seeding.
var ErrRematchNotAllowedRanked = errors.New("rematch not allowed in ranked matches")

// VoteRematch registers a rematch vote for playerID on the given session.
// Returns the session's Mode alongside the vote state so the caller can start
// the next session in the same mode — protects against silent ranked→casual
// downgrades if the ranked guard is ever relaxed.
func (svc *Service) VoteRematch(ctx context.Context, sessionID, playerID uuid.UUID) ([]store.RematchVote, int, uuid.UUID, bool, store.SessionMode, error) {
	session, err := svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return nil, 0, uuid.Nil, false, "", ErrSessionNotFound
	}
	if session.FinishedAt == nil {
		return nil, 0, uuid.Nil, false, session.Mode, ErrNotFinished
	}
	if session.Mode == store.SessionModeRanked {
		return nil, 0, uuid.Nil, false, session.Mode, ErrRematchNotAllowedRanked
	}

	players, err := svc.store.ListRoomPlayers(ctx, session.RoomID)
	if err != nil {
		return nil, 0, uuid.Nil, false, session.Mode, fmt.Errorf("VoteRematch: list players: %w", err)
	}

	callerFound := false
	for _, p := range players {
		if p.PlayerID == playerID {
			callerFound = true
			break
		}
	}
	if !callerFound {
		return nil, 0, uuid.Nil, false, session.Mode, ErrNotParticipant
	}

	if err := svc.store.UpsertRematchVote(ctx, sessionID, playerID); err != nil {
		return nil, 0, uuid.Nil, false, session.Mode, fmt.Errorf("VoteRematch: upsert vote: %w", err)
	}

	for _, p := range players {
		if svc.isBot(ctx, p.PlayerID) {
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
		return nil, 0, uuid.Nil, false, session.Mode, fmt.Errorf("VoteRematch: list votes: %w", err)
	}

	if len(votes) < len(players) {
		return votes, len(players), session.RoomID, false, session.Mode, nil
	}

	if err := svc.store.DeleteRematchVotes(ctx, sessionID); err != nil {
		return nil, 0, uuid.Nil, false, session.Mode, fmt.Errorf("VoteRematch: delete votes: %w", err)
	}

	if svc.timer != nil {
		svc.timer.Cancel(sessionID)
	}

	if err := svc.store.UpdateRoomStatus(ctx, session.RoomID, store.RoomStatusWaiting); err != nil {
		return nil, 0, uuid.Nil, false, session.Mode, fmt.Errorf("VoteRematch: reset room status: %w", err)
	}

	return votes, len(players), session.RoomID, true, session.Mode, nil
}

// --- Rating ------------------------------------------------------------------

const channelGameSessionFinished = "game.session.finished"

// publishGameFinished publishes a game.session.finished event to Redis.
// rating-service consumes this to update ELO. Errors are logged, never returned —
// a publish failure must not roll back a completed match.
func (svc *Service) publishGameFinished(
	ctx context.Context,
	session store.GameSession,
	players []store.RoomPlayer,
	result store.GameResult,
) {
	sessionPlayers := make([]sharedEvents.SessionPlayer, len(players))
	for i, rp := range players {
		sessionPlayers[i] = sharedEvents.SessionPlayer{
			PlayerID: rp.PlayerID.String(),
			Seat:     rp.Seat,
			Outcome:  outcomeFor(rp.PlayerID, result),
		}
	}

	evt := sharedEvents.GameSessionFinished{
		Meta: sharedEvents.Meta{
			EventID:    uuid.NewString(),
			OccurredAt: time.Now().UTC(),
			Version:    1,
		},
		SessionID:    session.ID.String(),
		ResultID:     result.ID.String(),
		RoomID:       session.RoomID.String(),
		GameID:       session.GameID,
		Mode:         string(session.Mode),
		EndedBy:      string(result.EndedBy),
		WinnerID:     winnerIDStr(result.WinnerID),
		IsDraw:       result.IsDraw,
		DurationSecs: durationSecs(result.DurationSecs),
		MoveCount:    session.MoveCount,
		Players:      sessionPlayers,
	}

	payload, err := json.Marshal(evt)
	if err != nil {
		svc.log.Error("publishGameFinished: marshal", "session_id", session.ID, "error", err)
		return
	}

	if err := svc.rdb.Publish(ctx, channelGameSessionFinished, payload).Err(); err != nil {
		svc.log.Error("publishGameFinished: redis publish", "session_id", session.ID, "error", err)
		return
	}

	svc.log.Info("publishGameFinished: published",
		"session_id", session.ID,
		"mode", session.Mode,
		"players", len(players),
	)
}

func outcomeFor(playerID uuid.UUID, result store.GameResult) string {
	if result.IsDraw {
		return "draw"
	}
	if result.EndedBy == store.EndedByForfeit {
		if result.WinnerID != nil && playerID == *result.WinnerID {
			return "win"
		}
		return "forfeit"
	}
	if result.WinnerID != nil && playerID == *result.WinnerID {
		return "win"
	}
	return "loss"
}

func winnerIDStr(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

// --- Helpers -----------------------------------------------------------------

func buildGameResultParams(session store.GameSession, result engine.Result, players []store.RoomPlayer) store.CreateGameResultParams {
	endedBy := store.EndedByDraw
	var winnerID *uuid.UUID

	switch result.Status {
	case engine.ResultWin:
		endedBy = store.EndedByNormal
		if result.WinnerID != nil {
			id, err := uuid.Parse(string(*result.WinnerID))
			if err == nil {
				winnerID = &id
			}
		}
	case engine.ResultDraw:
		endedBy = store.EndedByDraw
	}

	resultPlayers := make([]store.GameResultPlayer, len(players))
	for i, p := range players {
		outcome := store.OutcomeLoss
		if result.Status == engine.ResultDraw {
			outcome = store.OutcomeDraw
		} else if winnerID != nil && p.PlayerID == *winnerID {
			outcome = store.OutcomeWin
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
func durationSecs(d *int) int {
	if d == nil {
		return 0
	}
	return *d
}
func timePtr(t time.Time) *time.Time { return &t }

var _ = ws.EventGameOver
