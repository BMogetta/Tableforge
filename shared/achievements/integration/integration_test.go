// Package integration exercises the production achievement closures from
// shared/achievements/global and shared/achievements/games/tictactoe end to
// end. It lives in its own directory (and therefore its own test binary) so
// the Reset() calls in other shared/achievements tests can't wipe the
// init()-registered definitions out from under us.
package integration_test

import (
	"testing"

	"github.com/recess/shared/achievements"
	_ "github.com/recess/shared/achievements/games/tictactoe"
	_ "github.com/recess/shared/achievements/global"
	"github.com/recess/shared/events"
)

// productionEvt builds a minimal finished-game event for gameID.
func productionEvt(gameID string, moveCount int) events.GameSessionFinished {
	return events.GameSessionFinished{
		SessionID: "sess-int",
		RoomID:    "room-int",
		GameID:    gameID,
		Mode:      "casual",
		MoveCount: moveCount,
	}
}

func unlocksContain(unlocks []achievements.Unlock, key string, tier int) bool {
	for _, u := range unlocks {
		if u.Key == key && u.NewTier == tier {
			return true
		}
	}
	return false
}

func progressFor(t *testing.T, key string, cur achievements.Progress, evt events.GameSessionFinished, outcome string) (int, bool) {
	t.Helper()
	def, ok := achievements.Get(key)
	if !ok {
		t.Fatalf("%q not registered — check blank imports", key)
	}
	return achievements.ComputeNewProgress(def, cur, evt, outcome)
}

func TestGamesPlayed_IncrementsOnAnyOutcome(t *testing.T) {
	evt := productionEvt("tictactoe", 5)
	for _, outcome := range []string{"win", "loss", "draw", "forfeit"} {
		cur := achievements.Progress{Tier: 0, Progress: 4}
		got, changed := progressFor(t, "games_played", cur, evt, outcome)
		if !changed {
			t.Errorf("outcome=%s: expected games_played to apply, got changed=false", outcome)
		}
		if got != 5 {
			t.Errorf("outcome=%s: got=%d want 5", outcome, got)
		}
	}
}

// A common regression: forgetting the outcome guard turns games_won into
// games_played.
func TestGamesWon_OnlyOnWin(t *testing.T) {
	evt := productionEvt("tictactoe", 5)
	for _, outcome := range []string{"loss", "draw", "forfeit"} {
		cur := achievements.Progress{Tier: 0, Progress: 2}
		_, changed := progressFor(t, "games_won", cur, evt, outcome)
		if changed {
			t.Errorf("outcome=%s: games_won must not apply", outcome)
		}
	}
	got, changed := progressFor(t, "games_won", achievements.Progress{Progress: 2}, evt, "win")
	if !changed || got != 3 {
		t.Errorf("win: want (3, true), got (%d, %v)", got, changed)
	}
}

// TestWinStreak_AdvancesOnWinResetsOnNonWin protects the subtle cur.Progress+1
// semantics the evaluator lost its winStreak parameter for.
func TestWinStreak_AdvancesOnWinResetsOnNonWin(t *testing.T) {
	evt := productionEvt("tictactoe", 5)

	got, changed := progressFor(t, "win_streak", achievements.Progress{Progress: 2}, evt, "win")
	if !changed || got != 3 {
		t.Errorf("win with streak=2: want (3, true), got (%d, %v)", got, changed)
	}

	got, changed = progressFor(t, "win_streak", achievements.Progress{Progress: 5}, evt, "loss")
	if !changed || got != 0 {
		t.Errorf("loss with streak=5: want (0, true), got (%d, %v)", got, changed)
	}

	// No-op reset: losing with streak=0 must not churn the DB.
	got, changed = progressFor(t, "win_streak", achievements.Progress{Progress: 0}, evt, "loss")
	if changed {
		t.Errorf("loss with streak=0: want changed=false, got (%d, true)", got)
	}
}

func TestFirstDraw_OnlyOnDraw(t *testing.T) {
	evt := productionEvt("tictactoe", 5)
	for _, outcome := range []string{"win", "loss", "forfeit"} {
		_, changed := progressFor(t, "first_draw", achievements.Progress{}, evt, outcome)
		if changed {
			t.Errorf("outcome=%s: first_draw must not apply", outcome)
		}
	}
	got, changed := progressFor(t, "first_draw", achievements.Progress{}, evt, "draw")
	if !changed || got != 1 {
		t.Errorf("draw: want (1, true), got (%d, %v)", got, changed)
	}
}

// Closure-level cases use ComputeNewProgress directly (ForGame filter
// already applied upstream). Cross-game isolation is covered separately
// by TestEvaluate_NonTttGameSkipsTttPerfectGame below.
func TestTttPerfectGame_WinsFiveMoves(t *testing.T) {
	cases := []struct {
		name      string
		moveCount int
		outcome   string
		want      bool
	}{
		{"win in 5", 5, "win", true},
		{"win in 6", 6, "win", false},
		{"win in 5 but loser", 5, "loss", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			def, ok := achievements.Get("ttt_perfect_game")
			if !ok {
				t.Fatal("ttt_perfect_game not registered")
			}
			_, changed := achievements.ComputeNewProgress(
				def, achievements.Progress{}, productionEvt("tictactoe", tc.moveCount), tc.outcome,
			)
			if changed != tc.want {
				t.Errorf("changed=%v want %v", changed, tc.want)
			}
		})
	}
}

// TestEvaluate_NonTttGameSkipsTttPerfectGame asserts that the ForGame filter
// stops TTT-specific achievements from being evaluated for other games —
// the closure's outcome/move-count checks are only reached for TTT events.
func TestEvaluate_NonTttGameSkipsTttPerfectGame(t *testing.T) {
	evt := productionEvt("rootaccess", 5)
	evt.EndedBy = "win"

	unlocks := achievements.Evaluate(evt, "win", map[string]achievements.Progress{})
	for _, u := range unlocks {
		if u.Key == "ttt_perfect_game" {
			t.Errorf("rootaccess event triggered ttt_perfect_game unlock: %+v", u)
		}
	}
}

func TestTttGamesPlayed_EveryOutcome(t *testing.T) {
	for _, outcome := range []string{"win", "loss", "draw", "forfeit"} {
		got, changed := progressFor(t, "ttt_games_played",
			achievements.Progress{Progress: 10}, productionEvt("tictactoe", 5), outcome)
		if !changed || got != 11 {
			t.Errorf("outcome=%s: want (11, true), got (%d, %v)", outcome, got, changed)
		}
	}
}

// TestEvaluate_UnlocksAcrossFullPipeline runs Evaluate with the real
// production registry for a 5-move TTT win as the first-ever game,
// asserting that the winner gets the expected unlock bundle in one call.
func TestEvaluate_UnlocksAcrossFullPipeline(t *testing.T) {
	evt := productionEvt("tictactoe", 5)
	evt.EndedBy = "win"

	unlocks := achievements.Evaluate(evt, "win", map[string]achievements.Progress{})

	wantPairs := []struct {
		key  string
		tier int
	}{
		{"games_played", 1},
		{"games_won", 1},
		{"ttt_perfect_game", 1},
	}
	for _, w := range wantPairs {
		if !unlocksContain(unlocks, w.key, w.tier) {
			t.Errorf("missing unlock %s tier %d — got %+v", w.key, w.tier, unlocks)
		}
	}
}

// TestForGame_RegistersTttOnlyWhereExpected guards against a Registry entry
// forgetting its GameID filter.
func TestForGame_RegistersTttOnlyWhereExpected(t *testing.T) {
	ttt := achievements.ForGame("tictactoe")
	got := map[string]bool{}
	for _, d := range ttt {
		got[d.Key] = true
	}
	for _, key := range []string{"games_played", "games_won", "win_streak", "first_draw", "ttt_perfect_game", "ttt_games_played"} {
		if !got[key] {
			t.Errorf("ForGame(tictactoe) missing %q", key)
		}
	}

	// Non-TTT games must not see TTT-specific entries.
	chess := achievements.ForGame("chess")
	for _, d := range chess {
		if d.GameID == "tictactoe" {
			t.Errorf("ForGame(chess) leaked TTT-specific %q", d.Key)
		}
	}
}
