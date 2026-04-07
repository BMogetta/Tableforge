package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/game-server/internal/testutil"
)

// --- nextPlayerAfter tests ---------------------------------------------------

func TestNextPlayerAfter_TwoPlayers(t *testing.T) {
	p1 := uuid.New()
	p2 := uuid.New()
	players := []store.RoomPlayer{
		{PlayerID: p1, Seat: 0},
		{PlayerID: p2, Seat: 1},
	}

	next := nextPlayerAfter(engine.PlayerID(p1.String()), players)
	if next != engine.PlayerID(p2.String()) {
		t.Errorf("expected p2, got %s", next)
	}

	next = nextPlayerAfter(engine.PlayerID(p2.String()), players)
	if next != engine.PlayerID(p1.String()) {
		t.Errorf("expected p1 (wrap around), got %s", next)
	}
}

func TestNextPlayerAfter_ThreePlayers(t *testing.T) {
	p1 := uuid.New()
	p2 := uuid.New()
	p3 := uuid.New()
	players := []store.RoomPlayer{
		{PlayerID: p1, Seat: 0},
		{PlayerID: p2, Seat: 1},
		{PlayerID: p3, Seat: 2},
	}

	next := nextPlayerAfter(engine.PlayerID(p1.String()), players)
	if next != engine.PlayerID(p2.String()) {
		t.Errorf("expected p2 after p1, got %s", next)
	}

	next = nextPlayerAfter(engine.PlayerID(p2.String()), players)
	if next != engine.PlayerID(p3.String()) {
		t.Errorf("expected p3 after p2, got %s", next)
	}

	next = nextPlayerAfter(engine.PlayerID(p3.String()), players)
	if next != engine.PlayerID(p1.String()) {
		t.Errorf("expected p1 after p3 (wrap), got %s", next)
	}
}

func TestNextPlayerAfter_EmptyPlayers(t *testing.T) {
	current := engine.PlayerID(uuid.New().String())
	next := nextPlayerAfter(current, nil)
	if next != current {
		t.Errorf("expected same player back with empty slice, got %s", next)
	}
}

func TestNextPlayerAfter_UnknownPlayer(t *testing.T) {
	p1 := uuid.New()
	p2 := uuid.New()
	players := []store.RoomPlayer{
		{PlayerID: p1, Seat: 0},
		{PlayerID: p2, Seat: 1},
	}

	unknown := engine.PlayerID(uuid.New().String())
	next := nextPlayerAfter(unknown, players)
	// When player not found, currentSeat = -1, nextSeat = 0, returns player at seat 0.
	if next != engine.PlayerID(p1.String()) {
		t.Errorf("expected first player for unknown, got %s", next)
	}
}

func TestNextPlayerAfter_NonContiguousSeats(t *testing.T) {
	p1 := uuid.New()
	p2 := uuid.New()
	players := []store.RoomPlayer{
		{PlayerID: p1, Seat: 0},
		{PlayerID: p2, Seat: 3}, // gap in seat numbers
	}

	// After p1 (seat 0), next player by seat order is p2 (seat 3).
	next := nextPlayerAfter(engine.PlayerID(p1.String()), players)
	if next != engine.PlayerID(p2.String()) {
		t.Errorf("non-contiguous seats: expected p2, got %s", next)
	}

	// After p2 (seat 3), wraps back to p1 (seat 0).
	next = nextPlayerAfter(engine.PlayerID(p2.String()), players)
	if next != engine.PlayerID(p1.String()) {
		t.Errorf("non-contiguous seats wrap: expected p1, got %s", next)
	}
}

// --- ParseRedisOpt tests -----------------------------------------------------

func TestParseRedisOpt_Standard(t *testing.T) {
	addr, pass, err := ParseRedisOpt("redis://localhost:6379")
	if err != nil {
		t.Fatalf("ParseRedisOpt: %v", err)
	}
	if addr != "localhost:6379" {
		t.Errorf("expected localhost:6379, got %s", addr)
	}
	if pass != "" {
		t.Errorf("expected empty password, got %s", pass)
	}
}

func TestParseRedisOpt_NoPort(t *testing.T) {
	addr, _, err := ParseRedisOpt("redis://myhost")
	if err != nil {
		t.Fatalf("ParseRedisOpt: %v", err)
	}
	if addr != "myhost:6379" {
		t.Errorf("expected myhost:6379, got %s", addr)
	}
}

func TestParseRedisOpt_CustomPort(t *testing.T) {
	addr, _, err := ParseRedisOpt("redis://redis.internal:6380")
	if err != nil {
		t.Fatalf("ParseRedisOpt: %v", err)
	}
	if addr != "redis.internal:6380" {
		t.Errorf("expected redis.internal:6380, got %s", addr)
	}
}

func TestParseRedisOpt_WithPassword(t *testing.T) {
	addr, pass, err := ParseRedisOpt("redis://:secret@redis:6379")
	if err != nil {
		t.Fatalf("ParseRedisOpt: %v", err)
	}
	if addr != "redis:6379" {
		t.Errorf("expected redis:6379, got %s", addr)
	}
	if pass != "secret" {
		t.Errorf("expected password 'secret', got %s", pass)
	}
}

// --- timerPayload/taskID tests -----------------------------------------------

func TestTurnTaskID(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	got := turnTaskID(id)
	want := "timer:session:11111111-1111-1111-1111-111111111111"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestReadyTaskID(t *testing.T) {
	id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	got := readyTaskID(id)
	want := "timer:ready:22222222-2222-2222-2222-222222222222"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

// --- ResumePenalty constant --------------------------------------------------

func TestResumePenalty_Value(t *testing.T) {
	if ResumePenalty != 0.4 {
		t.Errorf("expected ResumePenalty 0.4, got %f", ResumePenalty)
	}
}

// --- stubRegistry for TimerHandlers tests ------------------------------------

type stubReg struct{}

func (stubReg) Get(_ string) (engine.Game, error) {
	return nil, fmt.Errorf("game not found")
}

// newTimerHandlersForTest creates a TimerHandlers wired to a FakeStore + stub registry.
func newTimerHandlersForTest(t *testing.T) (*TimerHandlers, *testutil.FakeStore) {
	t.Helper()
	fs := testutil.NewFakeStore()
	h := NewTimerHandlers(nil, nil, fs, nil, nil, stubReg{})
	return h, fs
}

// makeTestSession creates a minimal session in the FakeStore for timeout tests.
func makeTestSession(t *testing.T, fs *testutil.FakeStore) store.GameSession {
	t.Helper()
	ctx := context.Background()
	p1, _ := fs.CreatePlayer(ctx, "timeout_p1")
	p2, _ := fs.CreatePlayer(ctx, "timeout_p2")
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: uuid.NewString()[:8], GameID: "tictactoe", OwnerID: p1.ID, MaxPlayers: 2,
	})
	fs.AddPlayerToRoom(ctx, room.ID, p1.ID, 0)
	fs.AddPlayerToRoom(ctx, room.ID, p2.ID, 1)
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(p1.ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	return session
}

// --- onTimeout guard tests ---------------------------------------------------

func TestOnTimeout_SkipsFinishedSession(t *testing.T) {
	h, fs := newTimerHandlersForTest(t)
	ctx := context.Background()
	session := makeTestSession(t, fs)
	fs.FinishSession(ctx, session.ID)

	// Should be a no-op — session is already finished.
	h.onTimeout(session.ID)

	refetched, _ := fs.GetGameSession(ctx, session.ID)
	if refetched.FinishedAt == nil {
		t.Error("session should still be finished")
	}
}

func TestOnTimeout_SkipsSuspendedSession(t *testing.T) {
	h, fs := newTimerHandlersForTest(t)
	ctx := context.Background()
	session := makeTestSession(t, fs)
	fs.SuspendSession(ctx, session.ID, "pause_vote")

	// Should be a no-op — session is suspended.
	h.onTimeout(session.ID)

	refetched, _ := fs.GetGameSession(ctx, session.ID)
	if refetched.SuspendedAt == nil {
		t.Error("session should still be suspended")
	}
	if refetched.FinishedAt != nil {
		t.Error("suspended session should NOT be finished by timeout")
	}
}

func TestOnTimeout_SkipsNonExistentSession(t *testing.T) {
	h, _ := newTimerHandlersForTest(t)
	// Should not panic on unknown session.
	h.onTimeout(uuid.New())
}

func TestOnReadyTimeout_SkipsSuspendedSession(t *testing.T) {
	h, fs := newTimerHandlersForTest(t)
	ctx := context.Background()
	session := makeTestSession(t, fs)
	fs.SuspendSession(ctx, session.ID, "pause_vote")

	// Should be a no-op — session is suspended.
	h.onReadyTimeout(session.ID)

	refetched, _ := fs.GetGameSession(ctx, session.ID)
	if refetched.SuspendedAt == nil {
		t.Error("session should still be suspended")
	}
	if refetched.FinishedAt != nil {
		t.Error("suspended session should NOT be finished by ready timeout")
	}
}
