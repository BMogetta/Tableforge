// Package achievements defines achievement definitions and evaluation logic.
//
// String fields hold i18n keys, not display text. Keys follow a positional
// scheme so the frontend can derive them from (key, tier) alone without a
// server-side lookup:
//
//	achievements.{Key}.name
//	achievements.{Key}.description
//	achievements.{Key}.tiers.{N}.name        (N is 1-based)
//	achievements.{Key}.tiers.{N}.description ({{threshold}} interpolates)
//
// Translations live in frontend/src/locales/{en,es}.json. Anything published
// over Redis (AchievementUnlocked.TierName, notification payloads) carries
// the i18n key, not the resolved string — clients resolve at render time.
package achievements

import "github.com/recess/shared/events"

// Tier represents a single milestone within an achievement.
type Tier struct {
	Threshold      int
	NameKey        string
	DescriptionKey string
}

// Definition describes an achievement that can be earned by players.
//
// ComputeProgress captures the rule for this specific achievement: given the
// player's current progress and a finished-game event, it returns the new
// progress value and whether the event applied. Co-locating the rule with
// the metadata keeps the evaluator generic — adding a new achievement means
// appending a Registry entry, never editing evaluator.go.
type Definition struct {
	Key             string
	NameKey         string // i18n key for the display name
	DescriptionKey  string // i18n key for the description (flat achievements)
	GameID          string // "" = global, "tictactoe" = game-specific
	Type            string // "flat" | "tiered"
	Tiers           []Tier
	ComputeProgress ProgressFunc
}

const (
	TypeFlat   = "flat"
	TypeTiered = "tiered"
)

// nameKey / descKey / tierNameKey / tierDescKey build the positional i18n
// keys the frontend resolves at render time. Centralised here so the
// registry entries below stay short and hard to get out of sync.
func nameKey(id string) string { return "achievements." + id + ".name" }
func descKey(id string) string { return "achievements." + id + ".description" }
func tierNameKey(id string, tier int) string {
	return "achievements." + id + ".tiers." + itoa(tier) + ".name"
}
func tierDescKey(id string, tier int) string {
	return "achievements." + id + ".tiers." + itoa(tier) + ".description"
}

// Tiny, allocation-cheap int-to-string for single-digit tiers. Avoids pulling
// strconv into the registry's init path for a known-small domain (1-9).
func itoa(n int) string {
	if n >= 0 && n <= 9 {
		return string(rune('0' + n))
	}
	// Fallback for future growth; covers 10-99.
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// Registry holds all known achievement definitions.
var Registry = []Definition{
	// --- Global achievements ---
	{
		Key:     "games_played",
		NameKey: nameKey("games_played"),
		GameID:  "",
		Type:    TypeTiered,
		Tiers: []Tier{
			{Threshold: 1, NameKey: tierNameKey("games_played", 1), DescriptionKey: tierDescKey("games_played", 1)},
			{Threshold: 10, NameKey: tierNameKey("games_played", 2), DescriptionKey: tierDescKey("games_played", 2)},
			{Threshold: 50, NameKey: tierNameKey("games_played", 3), DescriptionKey: tierDescKey("games_played", 3)},
			{Threshold: 100, NameKey: tierNameKey("games_played", 4), DescriptionKey: tierDescKey("games_played", 4)},
			{Threshold: 500, NameKey: tierNameKey("games_played", 5), DescriptionKey: tierDescKey("games_played", 5)},
		},
		ComputeProgress: func(cur Progress, _ events.GameSessionFinished, _ string) (int, bool) {
			return cur.Progress + 1, true
		},
	},
	{
		Key:     "games_won",
		NameKey: nameKey("games_won"),
		GameID:  "",
		Type:    TypeTiered,
		Tiers: []Tier{
			{Threshold: 1, NameKey: tierNameKey("games_won", 1), DescriptionKey: tierDescKey("games_won", 1)},
			{Threshold: 10, NameKey: tierNameKey("games_won", 2), DescriptionKey: tierDescKey("games_won", 2)},
			{Threshold: 50, NameKey: tierNameKey("games_won", 3), DescriptionKey: tierDescKey("games_won", 3)},
			{Threshold: 100, NameKey: tierNameKey("games_won", 4), DescriptionKey: tierDescKey("games_won", 4)},
		},
		ComputeProgress: func(cur Progress, _ events.GameSessionFinished, outcome string) (int, bool) {
			if outcome != "win" {
				return cur.Progress, false
			}
			return cur.Progress + 1, true
		},
	},
	{
		Key:     "win_streak",
		NameKey: nameKey("win_streak"),
		GameID:  "",
		Type:    TypeTiered,
		Tiers: []Tier{
			{Threshold: 3, NameKey: tierNameKey("win_streak", 1), DescriptionKey: tierDescKey("win_streak", 1)},
			{Threshold: 5, NameKey: tierNameKey("win_streak", 2), DescriptionKey: tierDescKey("win_streak", 2)},
			{Threshold: 10, NameKey: tierNameKey("win_streak", 3), DescriptionKey: tierDescKey("win_streak", 3)},
		},
		// Progress tracks the current streak. Wins advance it; any other
		// outcome resets to 0 but keeps the achieved tier (checkTierUp never
		// demotes). A reset on cur.Progress == 0 is a no-op to avoid churn.
		ComputeProgress: func(cur Progress, _ events.GameSessionFinished, outcome string) (int, bool) {
			if outcome == "win" {
				return cur.Progress + 1, true
			}
			if cur.Progress > 0 {
				return 0, true
			}
			return cur.Progress, false
		},
	},
	{
		Key:            "first_draw",
		NameKey:        nameKey("first_draw"),
		DescriptionKey: descKey("first_draw"),
		GameID:         "",
		Type:           TypeFlat,
		Tiers: []Tier{
			{Threshold: 1, NameKey: tierNameKey("first_draw", 1), DescriptionKey: tierDescKey("first_draw", 1)},
		},
		ComputeProgress: func(cur Progress, _ events.GameSessionFinished, outcome string) (int, bool) {
			if outcome != "draw" {
				return cur.Progress, false
			}
			return 1, true
		},
	},

	// --- TicTacToe achievements ---
	{
		Key:            "ttt_perfect_game",
		NameKey:        nameKey("ttt_perfect_game"),
		DescriptionKey: descKey("ttt_perfect_game"),
		GameID:         "tictactoe",
		Type:           TypeFlat,
		Tiers: []Tier{
			{Threshold: 1, NameKey: tierNameKey("ttt_perfect_game", 1), DescriptionKey: tierDescKey("ttt_perfect_game", 1)},
		},
		// A perfect game is exactly 5 total moves: 3 by the winner, 2 by the
		// loser. Only awarded to the winner.
		ComputeProgress: func(cur Progress, evt events.GameSessionFinished, outcome string) (int, bool) {
			if outcome != "win" || evt.MoveCount != 5 {
				return cur.Progress, false
			}
			return 1, true
		},
	},
	{
		Key:     "ttt_games_played",
		NameKey: nameKey("ttt_games_played"),
		GameID:  "tictactoe",
		Type:    TypeTiered,
		Tiers: []Tier{
			{Threshold: 5, NameKey: tierNameKey("ttt_games_played", 1), DescriptionKey: tierDescKey("ttt_games_played", 1)},
			{Threshold: 25, NameKey: tierNameKey("ttt_games_played", 2), DescriptionKey: tierDescKey("ttt_games_played", 2)},
			{Threshold: 100, NameKey: tierNameKey("ttt_games_played", 3), DescriptionKey: tierDescKey("ttt_games_played", 3)},
		},
		ComputeProgress: func(cur Progress, _ events.GameSessionFinished, _ string) (int, bool) {
			// ForGame already filtered to GameID == "tictactoe", so every event
			// that reaches this closure is a TTT game regardless of outcome.
			return cur.Progress + 1, true
		},
	},
}

// index is a pre-built map for fast lookup by key.
var index map[string]Definition

func init() {
	index = make(map[string]Definition, len(Registry))
	for _, d := range Registry {
		index[d.Key] = d
	}
}

// Get returns the definition for the given key, or false if not found.
func Get(key string) (Definition, bool) {
	d, ok := index[key]
	return d, ok
}

// ForGame returns all definitions that apply to the given game ID.
// Pass "" for global-only achievements.
func ForGame(gameID string) []Definition {
	var out []Definition
	for _, d := range Registry {
		if d.GameID == "" || d.GameID == gameID {
			out = append(out, d)
		}
	}
	return out
}
