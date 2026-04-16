package runtime_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/domain/runtime"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/game-server/internal/platform/ws"
	"github.com/recess/game-server/internal/testutil"
)

// --- Helpers -----------------------------------------------------------------

func makeTask(taskType string, sessionID uuid.UUID) *asynq.Task {
	payload, _ := json.Marshal(map[string]string{"session_id": sessionID.String()})
	return asynq.NewTask(taskType, payload)
}

func makeInvalidTask(taskType string) *asynq.Task {
	return asynq.NewTask(taskType, []byte(`{"session_id":"not-a-uuid"}`))
}

func makeMalformedTask(taskType string) *asynq.Task {
	return asynq.NewTask(taskType, []byte(`{bad json`))
}

// timerFakeStore wraps FakeStore to override GetGameConfig with a configurable penalty.
type timerFakeStore struct {
	*testutil.FakeStore
	penalty store.TimeoutPenalty
}

func (s *timerFakeStore) GetGameConfig(_ context.Context, _ string) (store.GameConfig, error) {
	return store.GameConfig{
		DefaultTimeoutSecs: 60,
		MinTimeoutSecs:     10,
		MaxTimeoutSecs:     300,
		TimeoutPenalty:     s.penalty,
	}, nil
}

type noopTimer struct {
	scheduled []uuid.UUID
	cancelled []uuid.UUID
}

func (t *noopTimer) Schedule(session store.GameSession) {
	t.scheduled = append(t.scheduled, session.ID)
}
func (t *noopTimer) ScheduleIn(sessionID uuid.UUID, _ time.Duration) {
	t.scheduled = append(t.scheduled, sessionID)
}
func (t *noopTimer) Cancel(sessionID uuid.UUID) {
	t.cancelled = append(t.cancelled, sessionID)
}
func (t *noopTimer) ScheduleReady(sessionID uuid.UUID, _ time.Duration) {
	t.scheduled = append(t.scheduled, sessionID)
}
func (t *noopTimer) CancelReady(sessionID uuid.UUID) {
	t.cancelled = append(t.cancelled, sessionID)
}

func seedTimerSession(t *testing.T, fs *testutil.FakeStore, gameID string, players int) (store.GameSession, []engine.PlayerID) {
	t.Helper()
	pids := make([]engine.PlayerID, players)
	playerStrs := make([]any, players)
	for i := range pids {
		pids[i] = engine.PlayerID(uuid.New().String())
		playerStrs[i] = string(pids[i])
	}

	state := engine.GameState{
		CurrentPlayerID: pids[0],
		Data:            map[string]any{"turn": 0, "players": playerStrs},
	}
	stateJSON, _ := json.Marshal(state)

	gs, _ := fs.CreateGameSession(context.Background(), uuid.New(), gameID, stateJSON, nil, store.SessionModeCasual)

	for i, pid := range pids {
		pUUID, _ := uuid.Parse(string(pid))
		fs.RoomPlayers[gs.RoomID] = append(fs.RoomPlayers[gs.RoomID], store.RoomPlayer{
			PlayerID: pUUID,
			Seat:     i,
		})
	}

	return gs, pids
}

func newTimerHandlers(st store.Store, _ store.TimeoutPenalty) (*runtime.TimerHandlers, *noopTimer) {
	hub := ws.NewHub()
	timer := &noopTimer{}
	reg := &fakeRegistry{&stubGame{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := runtime.New(st, reg, nil, nil, log)
	h := runtime.NewTimerHandlers(svc, hub, st, nil, timer, reg)
	return h, timer
}

// --- HandleTurnTimeout -------------------------------------------------------

func TestHandleTurnTimeout_InvalidPayload(t *testing.T) {
	fs := testutil.NewFakeStore()
	h, _ := newTimerHandlers(fs, "")

	if err := h.HandleTurnTimeout(context.Background(), makeMalformedTask("timer:turn")); err == nil {
		t.Fatal("expected error for malformed JSON payload")
	}
	if err := h.HandleTurnTimeout(context.Background(), makeInvalidTask("timer:turn")); err == nil {
		t.Fatal("expected error for invalid UUID")
	}
}

func TestHandleTurnTimeout_SessionNotFound(t *testing.T) {
	fs := testutil.NewFakeStore()
	h, _ := newTimerHandlers(fs, "")

	err := h.HandleTurnTimeout(context.Background(), makeTask("timer:turn", uuid.New()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleTurnTimeout_AlreadyFinished(t *testing.T) {
	fs := testutil.NewFakeStore()
	st := &timerFakeStore{FakeStore: fs, penalty: store.PenaltyLoseGame}
	h, _ := newTimerHandlers(st, store.PenaltyLoseGame)

	gs, _ := seedTimerSession(t, fs, "stub", 2)
	now := time.Now()
	s := fs.Sessions[gs.ID]
	s.FinishedAt = &now
	fs.Sessions[gs.ID] = s

	if err := h.HandleTurnTimeout(context.Background(), makeTask("timer:turn", gs.ID)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fs.GameResults) > 0 {
		t.Error("expected no game result for already-finished session")
	}
}

func TestHandleTurnTimeout_LoseGame(t *testing.T) {
	fs := testutil.NewFakeStore()
	st := &timerFakeStore{FakeStore: fs, penalty: store.PenaltyLoseGame}
	h, _ := newTimerHandlers(st, store.PenaltyLoseGame)

	gs, _ := seedTimerSession(t, fs, "stub", 2)

	if err := h.HandleTurnTimeout(context.Background(), makeTask("timer:turn", gs.ID)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := fs.Sessions[gs.ID]
	if updated.FinishedAt == nil {
		t.Error("expected session to be finished after lose_game timeout")
	}

	if len(fs.GameResults) != 1 {
		t.Fatalf("expected 1 game result, got %d", len(fs.GameResults))
	}
	result := fs.GameResults[gs.ID]
	if result.EndedBy != store.EndedByTimeout {
		t.Errorf("expected ended_by=timeout, got %s", result.EndedBy)
	}
}

func TestHandleTurnTimeout_LoseTurn(t *testing.T) {
	fs := testutil.NewFakeStore()
	st := &timerFakeStore{FakeStore: fs, penalty: store.PenaltyLoseTurn}
	h, timer := newTimerHandlers(st, store.PenaltyLoseTurn)

	gs, pids := seedTimerSession(t, fs, "stub", 2)

	if err := h.HandleTurnTimeout(context.Background(), makeTask("timer:turn", gs.ID)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := fs.Sessions[gs.ID]
	if updated.FinishedAt != nil {
		t.Error("expected session to NOT be finished after lose_turn timeout")
	}

	var state engine.GameState
	json.Unmarshal(updated.State, &state)
	if state.CurrentPlayerID == pids[0] {
		t.Error("expected current player to change after lose_turn timeout")
	}

	if len(timer.scheduled) == 0 {
		t.Error("expected timer to be rescheduled after lose_turn")
	}
}

// --- HandleReadyTimeout ------------------------------------------------------

func TestHandleReadyTimeout_InvalidPayload(t *testing.T) {
	fs := testutil.NewFakeStore()
	h, _ := newTimerHandlers(fs, "")

	if err := h.HandleReadyTimeout(context.Background(), makeMalformedTask("timer:ready")); err == nil {
		t.Fatal("expected error for malformed JSON payload")
	}
}

func TestHandleReadyTimeout_AllReady(t *testing.T) {
	fs := testutil.NewFakeStore()
	h, _ := newTimerHandlers(fs, "")

	gs, pids := seedTimerSession(t, fs, "stub", 2)
	s := fs.Sessions[gs.ID]
	s.ReadyPlayers = []string{string(pids[0]), string(pids[1])}
	fs.Sessions[gs.ID] = s

	if err := h.HandleReadyTimeout(context.Background(), makeTask("timer:ready", gs.ID)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := fs.Sessions[gs.ID]
	if updated.FinishedAt != nil {
		t.Error("expected session to remain active when all players are ready")
	}
}

func TestHandleReadyTimeout_OneNotReady(t *testing.T) {
	fs := testutil.NewFakeStore()
	h, _ := newTimerHandlers(fs, "")

	gs, pids := seedTimerSession(t, fs, "stub", 2)
	s := fs.Sessions[gs.ID]
	s.ReadyPlayers = []string{string(pids[0])}
	fs.Sessions[gs.ID] = s

	if err := h.HandleReadyTimeout(context.Background(), makeTask("timer:ready", gs.ID)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := fs.Sessions[gs.ID]
	if updated.FinishedAt == nil {
		t.Error("expected session to be finished when a player wasn't ready")
	}

	if len(fs.GameResults) != 1 {
		t.Fatalf("expected 1 game result, got %d", len(fs.GameResults))
	}
	result := fs.GameResults[gs.ID]
	if result.EndedBy != store.EndedByReadyTimeout {
		t.Errorf("expected ended_by=ready_timeout, got %s", result.EndedBy)
	}
	if result.WinnerID == nil {
		t.Error("expected a winner when only one player was not ready")
	}
}

func TestHandleReadyTimeout_NoneReady_Draw(t *testing.T) {
	fs := testutil.NewFakeStore()
	h, _ := newTimerHandlers(fs, "")

	gs, _ := seedTimerSession(t, fs, "stub", 2)

	if err := h.HandleReadyTimeout(context.Background(), makeTask("timer:ready", gs.ID)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := fs.Sessions[gs.ID]
	if updated.FinishedAt == nil {
		t.Error("expected session to be finished")
	}

	if len(fs.GameResults) != 1 {
		t.Fatalf("expected 1 game result, got %d", len(fs.GameResults))
	}
	result := fs.GameResults[gs.ID]
	if !result.IsDraw {
		t.Error("expected draw when no players were ready")
	}
}
