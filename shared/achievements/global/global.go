// Package global registers every game-agnostic achievement from its init()
// function. Binaries that need achievements resolved (user-service, mainly)
// should blank-import this package.
package global

import (
	"github.com/recess/shared/achievements"
	"github.com/recess/shared/events"
)

func init() {
	achievements.Register(achievements.Definition{
		Key:     "games_played",
		NameKey: achievements.NameKey("games_played"),
		GameID:  "",
		Type:    achievements.TypeTiered,
		Tiers: []achievements.Tier{
			{Threshold: 1, NameKey: achievements.TierNameKey("games_played", 1), DescriptionKey: achievements.TierDescriptionKey("games_played", 1)},
			{Threshold: 10, NameKey: achievements.TierNameKey("games_played", 2), DescriptionKey: achievements.TierDescriptionKey("games_played", 2)},
			{Threshold: 50, NameKey: achievements.TierNameKey("games_played", 3), DescriptionKey: achievements.TierDescriptionKey("games_played", 3)},
			{Threshold: 100, NameKey: achievements.TierNameKey("games_played", 4), DescriptionKey: achievements.TierDescriptionKey("games_played", 4)},
			{Threshold: 500, NameKey: achievements.TierNameKey("games_played", 5), DescriptionKey: achievements.TierDescriptionKey("games_played", 5)},
		},
		ComputeProgress: func(cur achievements.Progress, _ events.GameSessionFinished, _ string) (int, bool) {
			return cur.Progress + 1, true
		},
	})

	achievements.Register(achievements.Definition{
		Key:     "games_won",
		NameKey: achievements.NameKey("games_won"),
		GameID:  "",
		Type:    achievements.TypeTiered,
		Tiers: []achievements.Tier{
			{Threshold: 1, NameKey: achievements.TierNameKey("games_won", 1), DescriptionKey: achievements.TierDescriptionKey("games_won", 1)},
			{Threshold: 10, NameKey: achievements.TierNameKey("games_won", 2), DescriptionKey: achievements.TierDescriptionKey("games_won", 2)},
			{Threshold: 50, NameKey: achievements.TierNameKey("games_won", 3), DescriptionKey: achievements.TierDescriptionKey("games_won", 3)},
			{Threshold: 100, NameKey: achievements.TierNameKey("games_won", 4), DescriptionKey: achievements.TierDescriptionKey("games_won", 4)},
		},
		ComputeProgress: func(cur achievements.Progress, _ events.GameSessionFinished, outcome string) (int, bool) {
			if outcome != "win" {
				return cur.Progress, false
			}
			return cur.Progress + 1, true
		},
	})

	achievements.Register(achievements.Definition{
		Key:     "win_streak",
		NameKey: achievements.NameKey("win_streak"),
		GameID:  "",
		Type:    achievements.TypeTiered,
		Tiers: []achievements.Tier{
			{Threshold: 3, NameKey: achievements.TierNameKey("win_streak", 1), DescriptionKey: achievements.TierDescriptionKey("win_streak", 1)},
			{Threshold: 5, NameKey: achievements.TierNameKey("win_streak", 2), DescriptionKey: achievements.TierDescriptionKey("win_streak", 2)},
			{Threshold: 10, NameKey: achievements.TierNameKey("win_streak", 3), DescriptionKey: achievements.TierDescriptionKey("win_streak", 3)},
		},
		// Progress tracks the current streak. Wins advance it; any other
		// outcome resets to 0 but keeps the achieved tier (checkTierUp
		// never demotes). A reset on cur.Progress == 0 is a no-op.
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

	achievements.Register(achievements.Definition{
		Key:            "first_draw",
		NameKey:        achievements.NameKey("first_draw"),
		DescriptionKey: achievements.DescriptionKey("first_draw"),
		GameID:         "",
		Type:           achievements.TypeFlat,
		Tiers: []achievements.Tier{
			{Threshold: 1, NameKey: achievements.TierNameKey("first_draw", 1), DescriptionKey: achievements.TierDescriptionKey("first_draw", 1)},
		},
		ComputeProgress: func(cur achievements.Progress, _ events.GameSessionFinished, outcome string) (int, bool) {
			if outcome != "draw" {
				return cur.Progress, false
			}
			return 1, true
		},
	})
}
