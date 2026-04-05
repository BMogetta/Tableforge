package runtime

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/platform/store"
)

func TestOutcomeFor_Draw(t *testing.T) {
	result := store.GameResult{IsDraw: true}
	got := outcomeFor(uuid.New(), result)
	if got != "draw" {
		t.Errorf("expected draw, got %q", got)
	}
}

func TestOutcomeFor_Winner(t *testing.T) {
	winner := uuid.New()
	loser := uuid.New()
	result := store.GameResult{WinnerID: &winner, EndedBy: store.EndedByNormal}

	if got := outcomeFor(winner, result); got != "win" {
		t.Errorf("expected win for winner, got %q", got)
	}
	if got := outcomeFor(loser, result); got != "loss" {
		t.Errorf("expected loss for loser, got %q", got)
	}
}

func TestOutcomeFor_Forfeit(t *testing.T) {
	winner := uuid.New()
	forfeiter := uuid.New()
	result := store.GameResult{WinnerID: &winner, EndedBy: store.EndedByForfeit}

	if got := outcomeFor(winner, result); got != "win" {
		t.Errorf("expected win for winner on forfeit, got %q", got)
	}
	if got := outcomeFor(forfeiter, result); got != "forfeit" {
		t.Errorf("expected forfeit for forfeiter, got %q", got)
	}
}

func TestWinnerIDStr_Nil(t *testing.T) {
	if got := winnerIDStr(nil); got != "" {
		t.Errorf("expected empty string for nil, got %q", got)
	}
}

func TestWinnerIDStr_NonNil(t *testing.T) {
	id := uuid.New()
	if got := winnerIDStr(&id); got != id.String() {
		t.Errorf("expected %s, got %q", id, got)
	}
}

func TestDurationSecs_Nil(t *testing.T) {
	if got := durationSecs(nil); got != 0 {
		t.Errorf("expected 0 for nil, got %d", got)
	}
}

func TestDurationSecs_NonNil(t *testing.T) {
	v := 42
	if got := durationSecs(&v); got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
}

func TestTimePtr(t *testing.T) {
	now := time.Now()
	got := timePtr(now)
	if got == nil || !got.Equal(now) {
		t.Errorf("expected %v, got %v", now, got)
	}
}

func TestBuildGameResultParams_Win(t *testing.T) {
	winnerPID := engine.PlayerID(uuid.New().String())
	winnerUUID, _ := uuid.Parse(string(winnerPID))
	loserUUID := uuid.New()

	session := store.GameSession{
		ID:     uuid.New(),
		GameID: "tictactoe",
	}
	result := engine.Result{
		Status:   engine.ResultWin,
		WinnerID: &winnerPID,
	}
	players := []store.RoomPlayer{
		{PlayerID: winnerUUID, Seat: 0},
		{PlayerID: loserUUID, Seat: 1},
	}

	params := buildGameResultParams(session, result, players)

	if params.EndedBy != store.EndedByNormal {
		t.Errorf("expected EndedByNormal, got %s", params.EndedBy)
	}
	if params.WinnerID == nil || *params.WinnerID != winnerUUID {
		t.Errorf("expected winner %s", winnerUUID)
	}
	if params.IsDraw {
		t.Error("expected IsDraw false")
	}

	for _, p := range params.Players {
		if p.PlayerID == winnerUUID && p.Outcome != store.OutcomeWin {
			t.Errorf("expected winner outcome win, got %s", p.Outcome)
		}
		if p.PlayerID == loserUUID && p.Outcome != store.OutcomeLoss {
			t.Errorf("expected loser outcome loss, got %s", p.Outcome)
		}
	}
}

func TestBuildGameResultParams_Draw(t *testing.T) {
	p1 := uuid.New()
	p2 := uuid.New()

	session := store.GameSession{ID: uuid.New(), GameID: "tictactoe"}
	result := engine.Result{Status: engine.ResultDraw}
	players := []store.RoomPlayer{
		{PlayerID: p1, Seat: 0},
		{PlayerID: p2, Seat: 1},
	}

	params := buildGameResultParams(session, result, players)

	if params.EndedBy != store.EndedByDraw {
		t.Errorf("expected EndedByDraw, got %s", params.EndedBy)
	}
	if !params.IsDraw {
		t.Error("expected IsDraw true")
	}
	if params.WinnerID != nil {
		t.Errorf("expected nil winner, got %v", params.WinnerID)
	}
	for _, p := range params.Players {
		if p.Outcome != store.OutcomeDraw {
			t.Errorf("expected draw outcome for all players, got %s for %s", p.Outcome, p.PlayerID)
		}
	}
}
