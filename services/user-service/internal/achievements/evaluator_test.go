package achievements

import (
	"testing"

	"github.com/recess/shared/events"
)

func TestGet(t *testing.T) {
	d, ok := Get("games_played")
	if !ok {
		t.Fatal("expected to find games_played")
	}
	if d.Type != TypeTiered {
		t.Errorf("expected tiered, got %s", d.Type)
	}
	if len(d.Tiers) != 5 {
		t.Errorf("expected 5 tiers, got %d", len(d.Tiers))
	}

	_, ok = Get("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent key")
	}
}

func TestForGame(t *testing.T) {
	global := ForGame("")
	if len(global) != 4 {
		t.Errorf("expected 4 global achievements, got %d", len(global))
	}

	ttt := ForGame("tictactoe")
	// 4 global + 2 tictactoe
	if len(ttt) != 6 {
		t.Errorf("expected 6 tictactoe-applicable achievements, got %d", len(ttt))
	}
}

func baseEvt(gameID string) events.GameSessionFinished {
	return events.GameSessionFinished{
		SessionID:    "sess-1",
		RoomID:       "room-1",
		GameID:       gameID,
		Mode:         "casual",
		DurationSecs: 120,
		Players: []events.SessionPlayer{
			{PlayerID: "p1", Seat: 0, Outcome: "win"},
			{PlayerID: "p2", Seat: 1, Outcome: "loss"},
		},
	}
}

func TestEvaluate_FlatUnlock(t *testing.T) {
	evt := baseEvt("tictactoe")
	evt.EndedBy = "draw"
	evt.IsDraw = true
	evt.Players[0].Outcome = "draw"
	evt.Players[1].Outcome = "draw"

	state := map[string]Progress{}
	unlocks := Evaluate(evt, "draw", state, 0)

	var found bool
	for _, u := range unlocks {
		if u.Key == "first_draw" {
			found = true
			if u.NewTier != 1 {
				t.Errorf("expected tier 1, got %d", u.NewTier)
			}
		}
	}
	if !found {
		t.Error("expected first_draw unlock")
	}
}

func TestEvaluate_FlatAlreadyUnlocked(t *testing.T) {
	evt := baseEvt("tictactoe")
	evt.EndedBy = "draw"
	evt.IsDraw = true

	state := map[string]Progress{
		"first_draw": {Tier: 1, Progress: 1},
	}
	unlocks := Evaluate(evt, "draw", state, 0)

	for _, u := range unlocks {
		if u.Key == "first_draw" {
			t.Error("should not unlock first_draw again")
		}
	}
}

func TestEvaluate_TieredProgression(t *testing.T) {
	evt := baseEvt("tictactoe")
	evt.EndedBy = "win"

	// Player has played 9 games, this is game 10 → should tier up to tier 2.
	state := map[string]Progress{
		"games_played": {Tier: 1, Progress: 9},
	}
	unlocks := Evaluate(evt, "win", state, 1)

	var found bool
	for _, u := range unlocks {
		if u.Key == "games_played" {
			found = true
			if u.NewTier != 2 {
				t.Errorf("expected tier 2, got %d", u.NewTier)
			}
			if u.TierName != "Regular" {
				t.Errorf("expected tier name Regular, got %s", u.TierName)
			}
		}
	}
	if !found {
		t.Error("expected games_played tier-up")
	}
}

func TestEvaluate_MaxTier(t *testing.T) {
	evt := baseEvt("tictactoe")
	evt.EndedBy = "win"

	// games_played at max tier (5).
	state := map[string]Progress{
		"games_played": {Tier: 5, Progress: 500},
	}
	unlocks := Evaluate(evt, "win", state, 1)

	for _, u := range unlocks {
		if u.Key == "games_played" {
			t.Error("should not tier-up past max")
		}
	}
}

func TestEvaluate_WinStreak(t *testing.T) {
	evt := baseEvt("tictactoe")
	evt.EndedBy = "win"

	state := map[string]Progress{
		"win_streak": {Tier: 0, Progress: 2},
	}
	// winStreak is 3 (2 existing + 1 new — caller passes the updated value).
	unlocks := Evaluate(evt, "win", state, 3)

	var found bool
	for _, u := range unlocks {
		if u.Key == "win_streak" {
			found = true
			if u.NewTier != 1 {
				t.Errorf("expected tier 1 (Hot Streak), got %d", u.NewTier)
			}
		}
	}
	if !found {
		t.Error("expected win_streak unlock at 3")
	}
}

func TestEvaluate_WinStreakNotOnLoss(t *testing.T) {
	evt := baseEvt("tictactoe")
	evt.EndedBy = "win"

	state := map[string]Progress{
		"win_streak": {Tier: 0, Progress: 2},
	}
	unlocks := Evaluate(evt, "loss", state, 0)

	for _, u := range unlocks {
		if u.Key == "win_streak" {
			t.Error("should not unlock win_streak on loss")
		}
	}
}

func TestComputeNewProgress_WinStreakReset(t *testing.T) {
	def, _ := Get("win_streak")
	cur := Progress{Tier: 1, Progress: 5}
	evt := baseEvt("tictactoe")

	newProg, changed := ComputeNewProgress(def, cur, evt, "loss", 0)
	if !changed {
		t.Error("expected changed=true on loss (streak reset)")
	}
	if newProg != 0 {
		t.Errorf("expected progress reset to 0, got %d", newProg)
	}
}

func TestEvaluate_TttPerfectGame(t *testing.T) {
	evt := baseEvt("tictactoe")
	evt.EndedBy = "win"
	evt.DurationSecs = 20 // short game

	state := map[string]Progress{}
	unlocks := Evaluate(evt, "win", state, 1)

	var found bool
	for _, u := range unlocks {
		if u.Key == "ttt_perfect_game" {
			found = true
		}
	}
	if !found {
		t.Error("expected ttt_perfect_game unlock for short TTT win")
	}
}

func TestEvaluate_TttPerfectGameTooLong(t *testing.T) {
	evt := baseEvt("tictactoe")
	evt.EndedBy = "win"
	evt.DurationSecs = 120 // too long

	state := map[string]Progress{}
	unlocks := Evaluate(evt, "win", state, 1)

	for _, u := range unlocks {
		if u.Key == "ttt_perfect_game" {
			t.Error("should not unlock ttt_perfect_game for long game")
		}
	}
}
