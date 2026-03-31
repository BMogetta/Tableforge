package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tableforge/game-server/internal/domain/engine"
	"github.com/tableforge/game-server/internal/platform/events"
	"github.com/tableforge/game-server/internal/platform/store"
	"github.com/tableforge/game-server/internal/platform/ws"
)

const timerKeyPrefix = "timer:session:"
const readyTimerKeyPrefix = "timer:ready:"

func timerKey(sessionID uuid.UUID) string {
	return timerKeyPrefix + sessionID.String()
}

type TurnTimer struct {
	rdb    *redis.Client
	svc    *Service
	hub    *ws.Hub
	st     store.Store
	events *events.Store
}

func NewTurnTimer(svc *Service, hub *ws.Hub, st store.Store, rdb *redis.Client, ev *events.Store) *TurnTimer {
	return &TurnTimer{rdb: rdb, svc: svc, hub: hub, st: st, events: ev}
}

func (tt *TurnTimer) Schedule(session store.GameSession) {
	if session.TurnTimeoutSecs == nil || *session.TurnTimeoutSecs <= 0 {
		return
	}
	ctx := context.Background()
	dur := time.Duration(*session.TurnTimeoutSecs) * time.Second
	if err := tt.rdb.Set(ctx, timerKey(session.ID), session.ID.String(), dur).Err(); err != nil {
		slog.Error("turn timer: schedule failed", "session_id", session.ID, "error", err)
	}
}

func (tt *TurnTimer) Cancel(sessionID uuid.UUID) {
	ctx := context.Background()
	if err := tt.rdb.Del(ctx, timerKey(sessionID)).Err(); err != nil {
		slog.Error("turn timer: cancel failed", "session_id", sessionID, "error", err)
	}
}

func (tt *TurnTimer) Start(ctx context.Context) {
	sub := tt.rdb.Subscribe(ctx, "__keyevent@0__:expired")
	defer sub.Close()
	slog.Info("turn timer: listening for keyspace expiration events")
	for {
		select {
		case <-ctx.Done():
			slog.Info("turn timer: stopping keyspace listener")
			return
		case msg, ok := <-sub.Channel():
			if !ok {
				return
			}
			key := msg.Payload

			if strings.HasPrefix(key, readyTimerKeyPrefix) {
				idStr := strings.TrimPrefix(key, readyTimerKeyPrefix)
				sessionID, err := uuid.Parse(idStr)
				if err != nil {
					slog.Error("turn timer: invalid session id in ready key", "key", key, "error", err)
					continue
				}
				go tt.onReadyTimeout(sessionID)
				continue
			}

			if strings.HasPrefix(key, timerKeyPrefix) {
				idStr := strings.TrimPrefix(key, timerKeyPrefix)
				sessionID, err := uuid.Parse(idStr)
				if err != nil {
					slog.Error("turn timer: invalid session id in key", "key", key, "error", err)
					continue
				}
				go tt.onTimeout(sessionID)
				continue
			}
		}
	}
}

func (tt *TurnTimer) ReschedulePending(ctx context.Context) {
	sessions, err := tt.st.ListSessionsNeedingTimer(ctx)
	if err != nil {
		slog.Error("turn timer: reschedule pending failed", "error", err)
		return
	}
	rescheduled, immediate := 0, 0
	for _, s := range sessions {
		if s.TurnTimeoutSecs == nil || *s.TurnTimeoutSecs <= 0 {
			continue
		}
		remaining := time.Until(s.LastMoveAt.Add(time.Duration(*s.TurnTimeoutSecs) * time.Second))
		if remaining <= 0 {
			immediate++
			go tt.onTimeout(s.ID)
			continue
		}
		if err := tt.rdb.Set(ctx, timerKey(s.ID), s.ID.String(), remaining).Err(); err != nil {
			slog.Error("turn timer: reschedule redis SET failed", "session_id", s.ID, "error", err)
			continue
		}
		rescheduled++
	}
	slog.Info("turn timer: reschedule pending complete", "rescheduled", rescheduled, "immediate", immediate)
}

func (tt *TurnTimer) onTimeout(sessionID uuid.UUID) {
	ctx := context.Background()
	session, err := tt.st.GetGameSession(ctx, sessionID)
	if err != nil || session.FinishedAt != nil {
		return
	}
	cfg, err := tt.st.GetGameConfig(ctx, session.GameID)
	if err != nil {
		slog.Error("turn timer: get game config failed", "game_id", session.GameID, "error", err)
		return
	}
	var state engine.GameState
	if err := json.Unmarshal(session.State, &state); err != nil {
		slog.Error("turn timer: unmarshal state failed", "session_id", sessionID, "error", err)
		return
	}

	timedOutPlayerID := state.CurrentPlayerID

	// If the game provides its own timeout move, delegate to ApplyMove so the
	// engine handles all state transitions (e.g. Love Letter penalty_lose).
	game, err := tt.svc.registry.Get(session.GameID)
	if err == nil {
		if th, ok := game.(engine.TurnTimeoutHandler); ok {
			tt.applyEngineTimeout(ctx, session, timedOutPlayerID, th.TimeoutMove())
			return
		}
	}

	switch cfg.TimeoutPenalty {
	case store.PenaltyLoseGame:
		tt.applyLoseGame(ctx, session, state, timedOutPlayerID)
	case store.PenaltyLoseTurn:
		tt.applyLoseTurn(ctx, session, state, timedOutPlayerID)
	default:
		slog.Warn("turn timer: unknown timeout penalty", "penalty", cfg.TimeoutPenalty, "game_id", session.GameID)
	}
}

func (tt *TurnTimer) applyLoseGame(ctx context.Context, session store.GameSession, state engine.GameState, timedOutPlayer engine.PlayerID) {
	players, err := tt.st.ListRoomPlayers(ctx, session.RoomID)
	if err != nil {
		slog.Error("turn timer: list room players failed", "room_id", session.RoomID, "error", err)
		return
	}
	var winnerID *engine.PlayerID
	for _, p := range players {
		pid := engine.PlayerID(p.PlayerID.String())
		if pid != timedOutPlayer {
			w := pid
			winnerID = &w
			break
		}
	}
	result := engine.Result{Status: engine.ResultWin, WinnerID: winnerID}
	if err := tt.st.FinishSession(ctx, session.ID); err != nil {
		slog.Error("turn timer: finish session failed", "session_id", session.ID, "error", err)
		return
	}
	if err := tt.st.UpdateRoomStatus(ctx, session.RoomID, store.RoomStatusFinished); err != nil {
		slog.Error("turn timer: finish room failed", "room_id", session.RoomID, "error", err)
	}
	resultParams := buildGameResultParams(session, result, players)
	resultParams.EndedBy = store.EndedByTimeout
	if _, err := tt.st.CreateGameResult(ctx, resultParams); err != nil {
		slog.Error("turn timer: create game result failed", "session_id", session.ID, "error", err)
	}
	if tt.events != nil {
		timedOutUUID, _ := uuid.Parse(string(timedOutPlayer))
		tt.events.Append(ctx, session.ID, events.TypeTurnTimeout, &timedOutUUID, map[string]any{
			"timed_out_player": string(timedOutPlayer),
			"penalty":          "lose_game",
		})
		tt.events.Append(ctx, session.ID, events.TypeGameOver, nil, map[string]any{
			"winner_id": winnerID,
			"status":    store.OutcomeWin,
			"ended_by":  store.EndedByTimeout,
		})
		tt.events.Persist(ctx, session.ID)
	}
	updatedSession, err := tt.st.GetGameSession(ctx, session.ID)
	if err != nil {
		updatedSession = session
	}
	tt.hub.Broadcast(session.RoomID, ws.Event{
		Type: ws.EventGameOver,
		Payload: map[string]any{
			"session":          updatedSession,
			"state":            state,
			"is_over":          true,
			"result":           map[string]any{"winner_id": winnerID, "status": "win"},
			"timed_out_player": string(timedOutPlayer),
		},
	})
	slog.Info("turn timer: session ended by timeout", "session_id", session.ID, "penalty", "lose_game", "timed_out_player", timedOutPlayer)
}

func (tt *TurnTimer) applyLoseTurn(ctx context.Context, session store.GameSession, state engine.GameState, timedOutPlayer engine.PlayerID) {
	players, err := tt.st.ListRoomPlayers(ctx, session.RoomID)
	if err != nil {
		slog.Error("turn timer: list room players failed", "room_id", session.RoomID, "error", err)
		return
	}
	state.CurrentPlayerID = nextPlayerAfter(timedOutPlayer, players)
	stateJSON, err := json.Marshal(state)
	if err != nil {
		slog.Error("turn timer: marshal state failed", "session_id", session.ID, "error", err)
		return
	}
	if err := tt.st.UpdateSessionState(ctx, session.ID, stateJSON); err != nil {
		slog.Error("turn timer: update state failed", "session_id", session.ID, "error", err)
		return
	}
	if err := tt.st.TouchLastMoveAt(ctx, session.ID); err != nil {
		slog.Error("turn timer: touch last_move_at failed", "session_id", session.ID, "error", err)
	}
	session, err = tt.st.GetGameSession(ctx, session.ID)
	if err != nil {
		slog.Error("turn timer: reload session failed", "session_id", session.ID, "error", err)
		return
	}
	if tt.events != nil {
		timedOutUUID, _ := uuid.Parse(string(timedOutPlayer))
		tt.events.Append(ctx, session.ID, events.TypeTurnTimeout, &timedOutUUID, map[string]any{
			"timed_out_player": string(timedOutPlayer),
			"next_player":      string(state.CurrentPlayerID),
			"penalty":          "lose_turn",
		})
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
	tt.Schedule(session)
	slog.Info("turn timer: turn skipped", "session_id", session.ID, "penalty", "lose_turn", "timed_out_player", timedOutPlayer, "next_player", state.CurrentPlayerID)
}

func nextPlayerAfter(current engine.PlayerID, players []store.RoomPlayer) engine.PlayerID {
	if len(players) == 0 {
		return current
	}
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
	nextSeat := (currentSeat + 1) % len(players)
	for _, p := range players {
		if p.Seat == nextSeat {
			return engine.PlayerID(p.PlayerID.String())
		}
	}
	return current
}

func readyTimerKey(sessionID uuid.UUID) string {
	return readyTimerKeyPrefix + sessionID.String()
}

// ScheduleReady sets a TTL-based timer for the ready handshake.
// When it expires, players who haven't confirmed are forfeited.
// timeout is injected so tests can use a shorter duration.
func (tt *TurnTimer) ScheduleReady(sessionID uuid.UUID, timeout time.Duration) {
	ctx := context.Background()
	if err := tt.rdb.Set(ctx, readyTimerKey(sessionID), sessionID.String(), timeout).Err(); err != nil {
		slog.Error("turn timer: schedule ready failed", "session_id", sessionID, "error", err)
	}
}

func (tt *TurnTimer) CancelReady(sessionID uuid.UUID) {
	ctx := context.Background()
	if err := tt.rdb.Del(ctx, readyTimerKey(sessionID)).Err(); err != nil {
		slog.Error("turn timer: cancel ready failed", "session_id", sessionID, "error", err)
	}
}

type TimeoutResult struct {
	SessionID      uuid.UUID         `json:"session_id"`
	TimedOutPlayer string            `json:"timed_out_player"`
	Result         engine.Result     `json:"result,omitempty"`
	State          *engine.GameState `json:"state,omitempty"`
	IsOver         bool              `json:"is_over"`
}

// applyEngineTimeout delegates a timeout to the engine by calling ApplyMove
// with the game-provided penalty payload. Used for games that implement
// engine.TurnTimeoutHandler (e.g. Love Letter penalty_lose).
func (tt *TurnTimer) applyEngineTimeout(ctx context.Context, session store.GameSession, timedOutPlayer engine.PlayerID, payload map[string]any) {
	playerUUID, err := uuid.Parse(string(timedOutPlayer))
	if err != nil {
		slog.Error("turn timer: engine timeout invalid player id", "player_id", timedOutPlayer, "error", err)
		return
	}

	result, err := tt.svc.ApplyMove(ctx, session.ID, playerUUID, payload)
	if err != nil {
		slog.Error("turn timer: engine timeout apply move failed", "session_id", session.ID, "error", err)
		return
	}

	eventType := ws.EventMoveApplied
	if result.IsOver {
		eventType = ws.EventGameOver
	}

	players, _ := tt.st.ListRoomPlayers(ctx, session.RoomID)
	tt.svc.BroadcastMove(ctx, tt.hub, result, eventType, players)

	if tt.events != nil {
		timedOutUUID, _ := uuid.Parse(string(timedOutPlayer))
		tt.events.Append(ctx, session.ID, events.TypeTurnTimeout, &timedOutUUID, map[string]any{
			"timed_out_player": string(timedOutPlayer),
			"penalty":          "engine_timeout",
		})
	}

	slog.Info("turn timer: engine timeout applied", "session_id", session.ID, "timed_out_player", timedOutPlayer, "is_over", result.IsOver)
}

func (tt *TurnTimer) onReadyTimeout(sessionID uuid.UUID) {
	ctx := context.Background()
	session, err := tt.st.GetGameSession(ctx, sessionID)
	if err != nil || session.FinishedAt != nil {
		return
	}

	players, err := tt.st.ListRoomPlayers(ctx, session.RoomID)
	if err != nil {
		slog.Error("turn timer: ready timeout list players failed", "session_id", sessionID, "error", err)
		return
	}

	readySet := make(map[string]bool, len(session.ReadyPlayers))
	for _, id := range session.ReadyPlayers {
		readySet[id] = true
	}

	var notReady []uuid.UUID
	for _, p := range players {
		if !readySet[p.PlayerID.String()] {
			notReady = append(notReady, p.PlayerID)
		}
	}

	if len(notReady) == 0 {
		return
	}

	isDraw := len(notReady) == len(players)

	if err := tt.st.FinishSession(ctx, sessionID); err != nil {
		slog.Error("turn timer: ready timeout finish session failed", "session_id", sessionID, "error", err)
		return
	}
	if err := tt.st.UpdateRoomStatus(ctx, session.RoomID, store.RoomStatusFinished); err != nil {
		slog.Error("turn timer: ready timeout finish room failed", "room_id", session.RoomID, "error", err)
	}

	var winnerID *uuid.UUID
	resultPlayers := make([]store.GameResultPlayer, len(players))
	for i, p := range players {
		outcome := store.OutcomeWin
		if isDraw {
			outcome = store.OutcomeDraw
		} else {
			absent := false
			for _, id := range notReady {
				if id == p.PlayerID {
					absent = true
					break
				}
			}
			if absent {
				outcome = store.OutcomeLoss
			} else {
				w := p.PlayerID
				winnerID = &w
			}
		}
		resultPlayers[i] = store.GameResultPlayer{
			PlayerID: p.PlayerID,
			Seat:     p.Seat,
			Outcome:  outcome,
		}
	}

	if _, err := tt.st.CreateGameResult(ctx, store.CreateGameResultParams{
		SessionID: sessionID,
		GameID:    session.GameID,
		WinnerID:  winnerID,
		IsDraw:    isDraw,
		EndedBy:   store.EndedByReadyTimeout,
		Players:   resultPlayers,
	}); err != nil {
		slog.Error("turn timer: ready timeout create result failed", "session_id", sessionID, "error", err)
	}

	tt.hub.Broadcast(session.RoomID, ws.Event{
		Type: ws.EventGameOver,
		Payload: map[string]any{
			"session": session,
			"is_over": true,
			"result": map[string]any{
				"winner_id": winnerID,
				"is_draw":   isDraw,
			},
			"ended_by": store.EndedByReadyTimeout,
		},
	})

	slog.Info("turn timer: ready timeout ended session", "session_id", sessionID, "is_draw", isDraw, "not_ready_count", len(notReady))
}

var _ = fmt.Sprintf
