package tictactoe_test

import (
	"testing"

	"github.com/recess/shared/achievements"
	_ "github.com/recess/shared/achievements/games/tictactoe"
)

func TestInit_RegistersTttAchievements(t *testing.T) {
	want := []string{"ttt_perfect_game", "ttt_games_played"}
	for _, key := range want {
		d, ok := achievements.Get(key)
		if !ok {
			t.Errorf("expected %q to be registered", key)
			continue
		}
		if d.GameID != "tictactoe" {
			t.Errorf("%s: GameID=%q want \"tictactoe\"", key, d.GameID)
		}
	}
}
