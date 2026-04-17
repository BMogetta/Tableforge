package global_test

import (
	"testing"

	"github.com/recess/shared/achievements"
	_ "github.com/recess/shared/achievements/global"
)

// TestInit_RegistersAllGlobals locks in the set of game-agnostic achievements
// shipped from init(). Adding or removing a global here should be intentional
// and cross-referenced with the frontend locale files.
func TestInit_RegistersAllGlobals(t *testing.T) {
	want := []string{"games_played", "games_won", "win_streak", "first_draw"}
	for _, key := range want {
		if _, ok := achievements.Get(key); !ok {
			t.Errorf("expected %q to be registered", key)
		}
	}

	// Globals are the entries with empty GameID.
	globals := achievements.ForGame("")
	got := make(map[string]bool, len(globals))
	for _, d := range globals {
		got[d.Key] = true
	}
	for _, key := range want {
		if !got[key] {
			t.Errorf("ForGame(\"\") missing %q", key)
		}
	}
}
