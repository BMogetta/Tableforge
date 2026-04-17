package achievements_test

import (
	"testing"

	"github.com/recess/shared/achievements"
	"github.com/recess/shared/events"
)

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

// registerWinStreak wires a win_streak-style definition into the registry
// for tests that care about streak semantics.
func registerWinStreak() {
	achievements.Register(achievements.Definition{
		Key:     "win_streak",
		NameKey: achievements.NameKey("win_streak"),
		GameID:  "",
		Type:    achievements.TypeTiered,
		Tiers: []achievements.Tier{
			{Threshold: 3, NameKey: achievements.TierNameKey("win_streak", 1), DescriptionKey: achievements.TierDescriptionKey("win_streak", 1)},
		},
		ComputeProgress: func(cur achievements.Progress, _ events.GameSessionFinished, outcome string) (int, bool) {
			if outcome == "win" {
				return cur.Progress + 1, true
			}
			if cur.Progress > 0 {
				return 0, true
			}
			return cur.Progress, false
		},
	})
}

func TestEvaluate_FlatUnlockOnDraw(t *testing.T) {
	seed(t)
	evt := baseEvt("tictactoe")
	evt.EndedBy = "draw"
	evt.IsDraw = true

	unlocks := achievements.Evaluate(evt, "draw", map[string]achievements.Progress{})

	var found bool
	for _, u := range unlocks {
		if u.Key == "first_draw" && u.NewTier == 1 {
			found = true
		}
	}
	if !found {
		t.Error("expected first_draw unlock")
	}
}

func TestEvaluate_FlatAlreadyUnlocked(t *testing.T) {
	seed(t)
	evt := baseEvt("tictactoe")
	evt.EndedBy = "draw"
	evt.IsDraw = true

	state := map[string]achievements.Progress{
		"first_draw": {Tier: 1, Progress: 1},
	}
	unlocks := achievements.Evaluate(evt, "draw", state)
	for _, u := range unlocks {
		if u.Key == "first_draw" {
			t.Error("should not unlock first_draw again")
		}
	}
}

func TestEvaluate_TieredProgression(t *testing.T) {
	seed(t)
	evt := baseEvt("tictactoe")
	evt.EndedBy = "win"

	state := map[string]achievements.Progress{
		"games_played": {Tier: 1, Progress: 9},
	}
	unlocks := achievements.Evaluate(evt, "win", state)

	var found bool
	for _, u := range unlocks {
		if u.Key == "games_played" {
			found = true
			if u.NewTier != 2 {
				t.Errorf("expected tier 2, got %d", u.NewTier)
			}
			if u.TierName != "achievements.games_played.tiers.2.name" {
				t.Errorf("expected i18n key for tier 2, got %s", u.TierName)
			}
		}
	}
	if !found {
		t.Error("expected games_played tier-up")
	}
}

func TestEvaluate_MaxTierNeverExceeded(t *testing.T) {
	seed(t)
	evt := baseEvt("tictactoe")
	evt.EndedBy = "win"

	state := map[string]achievements.Progress{
		"games_played": {Tier: 2, Progress: 10},
	}
	unlocks := achievements.Evaluate(evt, "win", state)
	for _, u := range unlocks {
		if u.Key == "games_played" {
			t.Error("should not tier-up past max")
		}
	}
}

func TestEvaluate_WinStreakFromCurProgress(t *testing.T) {
	t.Cleanup(achievements.Reset)
	achievements.Reset()
	registerWinStreak()

	evt := baseEvt("tictactoe")
	evt.EndedBy = "win"

	// cur.Progress=2 → closure returns 3 on win → crosses threshold 3.
	state := map[string]achievements.Progress{
		"win_streak": {Tier: 0, Progress: 2},
	}
	unlocks := achievements.Evaluate(evt, "win", state)
	var found bool
	for _, u := range unlocks {
		if u.Key == "win_streak" && u.NewTier == 1 {
			found = true
		}
	}
	if !found {
		t.Error("expected win_streak unlock when cur.Progress=2 and outcome=win")
	}
}

func TestComputeNewProgress_WinStreakResetOnLoss(t *testing.T) {
	t.Cleanup(achievements.Reset)
	achievements.Reset()
	registerWinStreak()

	def, _ := achievements.Get("win_streak")
	cur := achievements.Progress{Tier: 1, Progress: 5}
	newProg, changed := achievements.ComputeNewProgress(def, cur, baseEvt("tictactoe"), "loss")
	if !changed {
		t.Error("expected changed=true on loss (streak reset)")
	}
	if newProg != 0 {
		t.Errorf("expected progress reset to 0, got %d", newProg)
	}
}

func TestEvaluate_SkipsDefinitionsWithNilClosure(t *testing.T) {
	t.Cleanup(achievements.Reset)
	achievements.Reset()
	achievements.Register(achievements.Definition{
		Key:     "metadata_only",
		NameKey: achievements.NameKey("metadata_only"),
		GameID:  "",
		Type:    achievements.TypeFlat,
		Tiers: []achievements.Tier{
			{Threshold: 1, NameKey: achievements.TierNameKey("metadata_only", 1), DescriptionKey: achievements.TierDescriptionKey("metadata_only", 1)},
		},
		// ComputeProgress is nil — Evaluate must skip without panicking.
	})
	_ = achievements.Evaluate(baseEvt("tictactoe"), "win", map[string]achievements.Progress{})
}

func TestComputeNewProgress_NilClosureFallsBackToCur(t *testing.T) {
	def := achievements.Definition{Key: "x"}
	cur := achievements.Progress{Tier: 0, Progress: 3}
	newProg, changed := achievements.ComputeNewProgress(def, cur, baseEvt("x"), "win")
	if changed {
		t.Error("expected changed=false when ComputeProgress is nil")
	}
	if newProg != cur.Progress {
		t.Errorf("expected progress to remain %d, got %d", cur.Progress, newProg)
	}
}
