package runtime

import (
	"testing"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/platform/store"
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

	// After p1 (seat 0), next seat = 1. No player at seat 1 → falls through.
	next := nextPlayerAfter(engine.PlayerID(p1.String()), players)
	// This returns `current` because no player has seat 1.
	if next != engine.PlayerID(p1.String()) {
		t.Errorf("non-contiguous seats: expected fallback to current, got %s", next)
	}
}

// --- ParseRedisAddr tests ----------------------------------------------------

func TestParseRedisAddr_Standard(t *testing.T) {
	addr, err := ParseRedisAddr("redis://localhost:6379")
	if err != nil {
		t.Fatalf("ParseRedisAddr: %v", err)
	}
	if addr != "localhost:6379" {
		t.Errorf("expected localhost:6379, got %s", addr)
	}
}

func TestParseRedisAddr_NoPort(t *testing.T) {
	addr, err := ParseRedisAddr("redis://myhost")
	if err != nil {
		t.Fatalf("ParseRedisAddr: %v", err)
	}
	if addr != "myhost:6379" {
		t.Errorf("expected myhost:6379, got %s", addr)
	}
}

func TestParseRedisAddr_CustomPort(t *testing.T) {
	addr, err := ParseRedisAddr("redis://redis.internal:6380")
	if err != nil {
		t.Fatalf("ParseRedisAddr: %v", err)
	}
	if addr != "redis.internal:6380" {
		t.Errorf("expected redis.internal:6380, got %s", addr)
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
