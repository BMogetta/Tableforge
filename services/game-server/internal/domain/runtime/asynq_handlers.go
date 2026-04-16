package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/platform/events"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/game-server/internal/platform/ws"
)

// TimerHandlers processes Asynq tasks for turn and ready timeouts.
type TimerHandlers struct {
	svc    *Service
	hub    *ws.Hub
	st     store.Store
	events *events.Store
	timer  Timer
	reg    engine.Registry
}

func NewTimerHandlers(svc *Service, hub *ws.Hub, st store.Store, ev *events.Store, timer Timer, reg engine.Registry) *TimerHandlers {
	return &TimerHandlers{svc: svc, hub: hub, st: st, events: ev, timer: timer, reg: reg}
}

// HandleTurnTimeout processes a turn timeout task fired by Asynq.
func (h *TimerHandlers) HandleTurnTimeout(ctx context.Context, task *asynq.Task) error {
	sessionID, err := parseSessionID(task)
	if err != nil {
		return fmt.Errorf("turn timeout: %w", err)
	}
	h.onTimeout(sessionID)
	return nil
}

// HandleReadyTimeout processes a ready timeout task fired by Asynq.
func (h *TimerHandlers) HandleReadyTimeout(ctx context.Context, task *asynq.Task) error {
	sessionID, err := parseSessionID(task)
	if err != nil {
		return fmt.Errorf("ready timeout: %w", err)
	}
	h.onReadyTimeout(sessionID)
	return nil
}

func parseSessionID(task *asynq.Task) (uuid.UUID, error) {
	var p timerPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return uuid.Nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	id, err := uuid.Parse(p.SessionID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse session id: %w", err)
	}
	return id, nil
}

func (h *TimerHandlers) onTimeout(sessionID uuid.UUID) {
	ctx := context.Background()
	session, err := h.st.GetGameSession(ctx, sessionID)
	if err != nil || session.FinishedAt != nil || session.SuspendedAt != nil {
		return
	}
	cfg, err := h.st.GetGameConfig(ctx, session.GameID)
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

	game, err := h.reg.Get(session.GameID)
	if err != nil {
		slog.Error("turn timer: game not found", "game_id", session.GameID, "error", err)
		return
	}

	penalty := string(cfg.TimeoutPenalty)

	// All timeouts go through the engine. Games that implement
	// TurnTimeoutHandler provide a game-specific payload; the penalty string
	// lets them vary behaviour (e.g. skip turn vs. forfeit).
	th, ok := game.(engine.TurnTimeoutHandler)
	if !ok {
		slog.Error("turn timer: game does not implement TurnTimeoutHandler", "game_id", session.GameID)
		return
	}
	h.applyEngineTimeout(ctx, session, timedOutPlayerID, th.TimeoutMove(penalty))
}

// applyEngineTimeout delegates a timeout to the engine by calling ApplyMove
// with the game-provided timeout payload.
func (h *TimerHandlers) applyEngineTimeout(ctx context.Context, session store.GameSession, timedOutPlayer engine.PlayerID, payload map[string]any) {
	playerUUID, err := uuid.Parse(string(timedOutPlayer))
	if err != nil {
		slog.Error("turn timer: engine timeout invalid player id", "player_id", timedOutPlayer, "error", err)
		return
	}

	result, err := h.svc.ApplyMove(ctx, session.ID, playerUUID, payload)
	if err != nil {
		slog.Error("turn timer: engine timeout apply move failed", "session_id", session.ID, "error", err)
		return
	}

	eventType := ws.EventMoveApplied
	if result.IsOver {
		eventType = ws.EventGameOver
		// Override ended_by for the WS broadcast so clients see "timeout".
		if result.Result != nil {
			result.Result.EndedBy = string(store.EndedByTimeout)
		}
	}

	players, _ := h.st.ListRoomPlayers(ctx, session.RoomID)
	h.svc.BroadcastMove(ctx, h.hub, result, eventType, players)

	// ApplyMove records ended_by as "win"/"draw"; correct to "timeout".
	if result.IsOver {
		if err := h.st.UpdateGameResultEndedBy(ctx, session.ID, store.EndedByTimeout); err != nil {
			slog.Error("turn timer: update ended_by failed", "session_id", session.ID, "error", err)
		}
	}

	if h.events != nil {
		timedOutUUID, _ := uuid.Parse(string(timedOutPlayer))
		h.events.Append(ctx, session.ID, events.TypeTurnTimeout, &timedOutUUID, map[string]any{
			"timed_out_player": string(timedOutPlayer),
			"penalty":          "engine_timeout",
		})
	}

	if !result.IsOver {
		// Reschedule the turn timer for the next player.
		if updated, err := h.st.GetGameSession(ctx, session.ID); err == nil {
			h.timer.Schedule(updated)
		}
		// If the post-timeout state hands the turn to a bot, wake it.
		if nextUUID, err := uuid.Parse(string(result.State.CurrentPlayerID)); err == nil {
			h.svc.MaybeFireBot(ctx, h.hub, session.ID, nextUUID)
		}
	}

	slog.Info("turn timer: engine timeout applied", "session_id", session.ID, "timed_out_player", timedOutPlayer, "is_over", result.IsOver)
}

func (h *TimerHandlers) onReadyTimeout(sessionID uuid.UUID) {
	ctx := context.Background()
	session, err := h.st.GetGameSession(ctx, sessionID)
	if err != nil || session.FinishedAt != nil || session.SuspendedAt != nil {
		return
	}

	players, err := h.st.ListRoomPlayers(ctx, session.RoomID)
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

	if err := h.st.FinishSession(ctx, sessionID); err != nil {
		slog.Error("turn timer: ready timeout finish session failed", "session_id", sessionID, "error", err)
		return
	}
	if err := h.st.UpdateRoomStatus(ctx, session.RoomID, store.RoomStatusFinished); err != nil {
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

	if _, err := h.st.CreateGameResult(ctx, store.CreateGameResultParams{
		SessionID: sessionID,
		GameID:    session.GameID,
		WinnerID:  winnerID,
		IsDraw:    isDraw,
		EndedBy:   store.EndedByReadyTimeout,
		Players:   resultPlayers,
	}); err != nil {
		slog.Error("turn timer: ready timeout create result failed", "session_id", sessionID, "error", err)
	}

	h.hub.Broadcast(session.RoomID, ws.Event{
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

// ReschedulePending re-arms timers for sessions that were active when the server
// last shut down. Acts as a safety net on startup — Asynq tasks persist in Redis,
// but this handles edge cases like Redis data loss or first deploy migration.
func (h *TimerHandlers) ReschedulePending(ctx context.Context) {
	sessions, err := h.st.ListSessionsNeedingTimer(ctx)
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
			go h.onTimeout(s.ID)
			continue
		}
		h.timer.Schedule(s)
		rescheduled++
	}
	slog.Info("turn timer: reschedule pending complete", "rescheduled", rescheduled, "immediate", immediate)
}

