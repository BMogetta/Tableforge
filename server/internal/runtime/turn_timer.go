package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/engine"
	"github.com/tableforge/server/internal/store"
	"github.com/tableforge/server/internal/ws"
)

// TurnTimer manages per-session turn countdown timers.
// When a timer fires, it applies a timeout forfeit and broadcasts the result.
type TurnTimer struct {
	mu     sync.Mutex
	timers map[uuid.UUID]*time.Timer
	svc    *Service
	hub    *ws.Hub
	st     store.Store
}

// NewTurnTimer creates a TurnTimer backed by the given runtime service.
func NewTurnTimer(svc *Service, hub *ws.Hub, st store.Store) *TurnTimer {
	return &TurnTimer{
		timers: make(map[uuid.UUID]*time.Timer),
		svc:    svc,
		hub:    hub,
		st:     st,
	}
}

// Schedule sets or resets the turn timer for a session.
// If timeoutSecs is nil or zero, no timer is set.
// Call this after every move and when a session starts.
func (tt *TurnTimer) Schedule(session store.GameSession) {
	if session.TurnTimeoutSecs == nil || *session.TurnTimeoutSecs <= 0 {
		return
	}

	tt.mu.Lock()
	defer tt.mu.Unlock()

	// Cancel existing timer if any.
	if t, ok := tt.timers[session.ID]; ok {
		t.Stop()
	}

	deadline := time.Duration(*session.TurnTimeoutSecs) * time.Second
	tt.timers[session.ID] = time.AfterFunc(deadline, func() {
		tt.onTimeout(session.ID)
	})
}

// Cancel stops the timer for a session without triggering a timeout.
// Call this when a session ends normally.
func (tt *TurnTimer) Cancel(sessionID uuid.UUID) {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	if t, ok := tt.timers[sessionID]; ok {
		t.Stop()
		delete(tt.timers, sessionID)
	}
}

// onTimeout is called when a turn timer fires.
// It loads the session, determines the penalty, and applies it.
func (tt *TurnTimer) onTimeout(sessionID uuid.UUID) {
	tt.mu.Lock()
	delete(tt.timers, sessionID)
	tt.mu.Unlock()

	ctx := context.Background()

	session, err := tt.st.GetGameSession(ctx, sessionID)
	if err != nil || session.FinishedAt != nil {
		// Session gone or already finished — nothing to do.
		return
	}

	cfg, err := tt.st.GetGameConfig(ctx, session.GameID)
	if err != nil {
		log.Printf("TurnTimer: get game config for %s: %v", session.GameID, err)
		return
	}

	var state engine.GameState
	if err := json.Unmarshal(session.State, &state); err != nil {
		log.Printf("TurnTimer: unmarshal state for session %s: %v", sessionID, err)
		return
	}

	timedOutPlayerID := state.CurrentPlayerID

	switch cfg.TimeoutPenalty {
	case store.PenaltyLoseGame:
		tt.applyLoseGame(ctx, session, state, timedOutPlayerID)
	case store.PenaltyLoseTurn:
		tt.applyLoseTurn(ctx, session, state, timedOutPlayerID)
	default:
		log.Printf("TurnTimer: unknown penalty %q for game %s", cfg.TimeoutPenalty, session.GameID)
	}
}

// applyLoseGame ends the game, declaring the other player the winner.
func (tt *TurnTimer) applyLoseGame(ctx context.Context, session store.GameSession, state engine.GameState, timedOutPlayer engine.PlayerID) {
	// Load all players in the room to find the winner.
	players, err := tt.st.ListRoomPlayers(ctx, session.RoomID)
	if err != nil {
		log.Printf("TurnTimer: list room players for %s: %v", session.RoomID, err)
		return
	}

	// Find the winner: the player who is NOT the timed-out player.
	var winnerID *engine.PlayerID
	for _, p := range players {
		pid := engine.PlayerID(p.PlayerID.String())
		if pid != timedOutPlayer {
			w := pid
			winnerID = &w
			break
		}
	}

	result := engine.Result{
		Status:   engine.ResultWin,
		WinnerID: winnerID,
	}

	if err := tt.st.FinishSession(ctx, session.ID); err != nil {
		log.Printf("TurnTimer: finish session %s: %v", session.ID, err)
		return
	}

	// Record game result.
	resultParams := buildGameResultParams(session, result, players)
	resultParams.EndedBy = "timeout"
	if _, err := tt.st.CreateGameResult(ctx, resultParams); err != nil {
		log.Printf("TurnTimer: create game result for %s: %v", session.ID, err)
	}

	// Reload session after finishing so finished_at is populated.
	updatedSession, err := tt.st.GetGameSession(ctx, session.ID)
	if err != nil {
		log.Printf("TurnTimer: reload session %s: %v", session.ID, err)
		updatedSession = session
	}

	tt.hub.Broadcast(session.RoomID, ws.Event{
		Type: ws.EventGameOver,
		Payload: map[string]any{
			"session": updatedSession,
			"state":   state,
			"is_over": true,
			"result": map[string]any{
				"winner_id": winnerID,
				"status":    "win",
			},
			"timed_out_player": string(timedOutPlayer),
		},
	})

	log.Printf("TurnTimer: session %s ended by timeout (lose_game), timed out player: %s", session.ID, timedOutPlayer)
}

// applyLoseTurn advances the turn to the next player without ending the game.
func (tt *TurnTimer) applyLoseTurn(ctx context.Context, session store.GameSession, state engine.GameState, timedOutPlayer engine.PlayerID) {
	players, err := tt.st.ListRoomPlayers(ctx, session.RoomID)
	if err != nil {
		log.Printf("TurnTimer: list room players for %s: %v", session.RoomID, err)
		return
	}

	// Advance to the next player in seat order.
	nextPlayer := nextPlayerAfter(timedOutPlayer, players)
	state.CurrentPlayerID = nextPlayer

	stateJSON, err := json.Marshal(state)
	if err != nil {
		log.Printf("TurnTimer: marshal state for session %s: %v", session.ID, err)
		return
	}

	if err := tt.st.UpdateSessionState(ctx, session.ID, stateJSON); err != nil {
		log.Printf("TurnTimer: update state for session %s: %v", session.ID, err)
		return
	}

	if err := tt.st.TouchLastMoveAt(ctx, session.ID); err != nil {
		log.Printf("TurnTimer: touch last_move_at for session %s: %v", session.ID, err)
	}

	// Reload session for the broadcast.
	session, err = tt.st.GetGameSession(ctx, session.ID)
	if err != nil {
		log.Printf("TurnTimer: reload session %s: %v", session.ID, err)
		return
	}

	tt.hub.Broadcast(session.RoomID, ws.Event{
		Type: ws.EventMoveApplied,
		Payload: TimeoutResult{
			SessionID:      session.ID,
			TimedOutPlayer: string(timedOutPlayer),
			State:          &state,
			IsOver:         false,
		},
	})

	// Reschedule for the next player's turn.
	tt.Schedule(session)

	log.Printf("TurnTimer: session %s turn skipped (lose_turn), timed out player: %s, next: %s",
		session.ID, timedOutPlayer, nextPlayer)
}

// nextPlayerAfter returns the PlayerID of the next player in seat order after current.
func nextPlayerAfter(current engine.PlayerID, players []store.RoomPlayer) engine.PlayerID {
	if len(players) == 0 {
		return current
	}

	// Find current player's seat.
	currentSeat := -1
	for _, p := range players {
		if engine.PlayerID(p.PlayerID.String()) == current {
			currentSeat = p.Seat
			break
		}
	}
	if currentSeat == -1 {
		return engine.PlayerID(players[0].PlayerID.String())
	}

	// Find the player in the next seat (wraps around).
	nextSeat := (currentSeat + 1) % len(players)
	for _, p := range players {
		if p.Seat == nextSeat {
			return engine.PlayerID(p.PlayerID.String())
		}
	}

	return current
}

// TimeoutResult is the WebSocket payload sent when a turn times out.
type TimeoutResult struct {
	SessionID      uuid.UUID         `json:"session_id"`
	TimedOutPlayer string            `json:"timed_out_player"`
	Result         engine.Result     `json:"result,omitempty"`
	State          *engine.GameState `json:"state,omitempty"`
	IsOver         bool              `json:"is_over"`
}

// buildGameResultParams is duplicated here to avoid circular imports.
// Keep in sync with the one in runtime.go.
func buildGameResultParamsFromTimeout(session store.GameSession, result engine.Result, players []store.RoomPlayer) store.CreateGameResultParams {
	endedBy := "timeout"
	var winnerID *uuid.UUID

	if result.Status == engine.ResultWin && result.WinnerID != nil {
		id, err := uuid.Parse(string(*result.WinnerID))
		if err == nil {
			winnerID = &id
		}
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

// resolveWinnerID converts engine.PlayerID to uuid.UUID.
func resolveWinnerID(winnerID *engine.PlayerID) *uuid.UUID {
	if winnerID == nil {
		return nil
	}
	id, err := uuid.Parse(string(*winnerID))
	if err != nil {
		return nil
	}
	return &id
}

// Ensure buildGameResultParams in turn_timer.go uses the local version.
// The one in runtime.go is identical — consider extracting to a shared file.
var _ = fmt.Sprintf // keep import
