// Package tictactoe registers every TicTacToe-specific achievement from its
// init() function. Binaries that need TicTacToe achievements resolved should
// blank-import this package alongside shared/achievements/global.
package tictactoe

import (
	"github.com/recess/shared/achievements"
	"github.com/recess/shared/events"
)

const gameID = "tictactoe"

func init() {
	achievements.Register(achievements.Definition{
		Key:            "ttt_perfect_game",
		NameKey:        achievements.NameKey("ttt_perfect_game"),
		DescriptionKey: achievements.DescriptionKey("ttt_perfect_game"),
		GameID:         gameID,
		Type:           achievements.TypeFlat,
		Tiers: []achievements.Tier{
			{
				Threshold:      1,
				NameKey:        achievements.TierNameKey("ttt_perfect_game", 1),
				DescriptionKey: achievements.TierDescriptionKey("ttt_perfect_game", 1),
			},
		},
		// A perfect game is exactly 5 total moves: 3 by the winner, 2 by the
		// loser. Only awarded to the winner.
		ComputeProgress: func(cur achievements.Progress, evt events.GameSessionFinished, outcome string) (int, bool) {
			if outcome != "win" || evt.MoveCount != 5 {
				return cur.Progress, false
			}
			return 1, true
		},
	})

	achievements.Register(achievements.Definition{
		Key:     "ttt_games_played",
		NameKey: achievements.NameKey("ttt_games_played"),
		GameID:  gameID,
		Type:    achievements.TypeTiered,
		Tiers: []achievements.Tier{
			{Threshold: 5, NameKey: achievements.TierNameKey("ttt_games_played", 1), DescriptionKey: achievements.TierDescriptionKey("ttt_games_played", 1)},
			{Threshold: 25, NameKey: achievements.TierNameKey("ttt_games_played", 2), DescriptionKey: achievements.TierDescriptionKey("ttt_games_played", 2)},
			{Threshold: 100, NameKey: achievements.TierNameKey("ttt_games_played", 3), DescriptionKey: achievements.TierDescriptionKey("ttt_games_played", 3)},
		},
		ComputeProgress: func(cur achievements.Progress, _ events.GameSessionFinished, _ string) (int, bool) {
			// ForGame already filtered to GameID == "tictactoe"; every event
			// that reaches this closure is a TTT game regardless of outcome.
			return cur.Progress + 1, true
		},
	})
}
