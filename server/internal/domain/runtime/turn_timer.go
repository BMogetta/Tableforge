package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tableforge/server/internal/domain/engine"
	"github.com/tableforge/server/internal/platform/events"
	"github.com/tableforge/server/internal/platform/store"
	"github.com/tableforge/server/internal/platform/ws"
)

const timerKeyPrefix = "timer:session:"

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
		log.Printf("TurnTimer.Schedule: redis SET %s: %v", session.ID, err)
	}
}

func (tt *TurnTimer) Cancel(sessionID uuid.UUID) {
	ctx := context.Background()
	if err := tt.rdb.Del(ctx, timerKey(sessionID)).Err(); err != nil {
		log.Printf("TurnTimer.Cancel: redis DEL %s: %v", sessionID, err)
	}
}

func (tt *TurnTimer) Start(ctx context.Context) {
	sub := tt.rdb.Subscribe(ctx, "__keyevent@0__:expired")
	defer sub.Close()
	log.Println("TurnTimer: listening for Redis keyspace expiration events")
	for {
		select {
		case <-ctx.Done():
			log.Println("TurnTimer: stopping keyspace listener")
			return
		case msg, ok := <-sub.Channel():
			if !ok {
				return
			}
			key := msg.Payload
			if !strings.HasPrefix(key, timerKeyPrefix) {
				continue
			}
			idStr := strings.TrimPrefix(key, timerKeyPrefix)
			sessionID, err := uuid.Parse(idStr)
			if err != nil {
				log.Printf("TurnTimer: invalid session id in key %q: %v", key, err)
				continue
			}
			go tt.onTimeout(sessionID)
		}
	}
}

func (tt *TurnTimer) ReschedulePending(ctx context.Context) {
	sessions, err := tt.st.ListSessionsNeedingTimer(ctx)
	if err != nil {
		log.Printf("TurnTimer.ReschedulePending: list sessions: %v", err)
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
			log.Printf("TurnTimer.ReschedulePending: redis SET %s: %v", s.ID, err)
			continue
		}
		rescheduled++
	}
	log.Printf("TurnTimer.ReschedulePending: %d rescheduled, %d fired immediately", rescheduled, immediate)
}

func (tt *TurnTimer) onTimeout(sessionID uuid.UUID) {
	ctx := context.Background()
	session, err := tt.st.GetGameSession(ctx, sessionID)
	if err != nil || session.FinishedAt != nil {
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
		log.Printf("TurnTimer: unknown penalty %q for game %s", cfg.TimeoutPenalty, session.GameID)
	}
}

func (tt *TurnTimer) applyLoseGame(ctx context.Context, session store.GameSession, state engine.GameState, timedOutPlayer engine.PlayerID) {
	players, err := tt.st.ListRoomPlayers(ctx, session.RoomID)
	if err != nil {
		log.Printf("TurnTimer: list room players for %s: %v", session.RoomID, err)
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
		log.Printf("TurnTimer: finish session %s: %v", session.ID, err)
		return
	}
	if err := tt.st.UpdateRoomStatus(ctx, session.RoomID, store.RoomStatusFinished); err != nil {
		log.Printf("TurnTimer: finish room %s: %v", session.RoomID, err)
	}
	resultParams := buildGameResultParams(session, result, players)
	resultParams.EndedBy = "timeout"
	if _, err := tt.st.CreateGameResult(ctx, resultParams); err != nil {
		log.Printf("TurnTimer: create game result for %s: %v", session.ID, err)
	}
	if tt.events != nil {
		timedOutUUID, _ := uuid.Parse(string(timedOutPlayer))
		tt.events.Append(ctx, session.ID, events.TypeTurnTimeout, &timedOutUUID, map[string]any{
			"timed_out_player": string(timedOutPlayer),
			"penalty":          "lose_game",
		})
		tt.events.Append(ctx, session.ID, events.TypeGameOver, nil, map[string]any{
			"winner_id": winnerID,
			"status":    "win",
			"ended_by":  "timeout",
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
	log.Printf("TurnTimer: session %s ended by timeout (lose_game), timed out: %s", session.ID, timedOutPlayer)
}

func (tt *TurnTimer) applyLoseTurn(ctx context.Context, session store.GameSession, state engine.GameState, timedOutPlayer engine.PlayerID) {
	players, err := tt.st.ListRoomPlayers(ctx, session.RoomID)
	if err != nil {
		log.Printf("TurnTimer: list room players for %s: %v", session.RoomID, err)
		return
	}
	state.CurrentPlayerID = nextPlayerAfter(timedOutPlayer, players)
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
	session, err = tt.st.GetGameSession(ctx, session.ID)
	if err != nil {
		log.Printf("TurnTimer: reload session %s: %v", session.ID, err)
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
	log.Printf("TurnTimer: session %s turn skipped (lose_turn), timed out: %s, next: %s",
		session.ID, timedOutPlayer, state.CurrentPlayerID)
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
		log.Printf("TurnTimer: applyEngineTimeout: invalid player id %q: %v", timedOutPlayer, err)
		return
	}

	result, err := tt.svc.ApplyMove(ctx, session.ID, playerUUID, payload)
	if err != nil {
		log.Printf("TurnTimer: applyEngineTimeout: ApplyMove for session %s: %v", session.ID, err)
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

	log.Printf("TurnTimer: session %s engine timeout applied, timed out: %s, is_over: %v",
		session.ID, timedOutPlayer, result.IsOver)
}

var _ = fmt.Sprintf
