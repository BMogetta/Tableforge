// Package achievements holds the cross-service achievement system: types,
// registry, and evaluator. Callers register Definitions from init() in
// leaf packages (shared/achievements/global for game-agnostic achievements,
// shared/achievements/games/{game} for game-specific ones) so that adding
// a new achievement is additive — no edits to evaluator or registry.
//
// The registry is the single source of truth consumed by user-service
// (evaluation + persistence) and the frontend (via the definitions API).
// String fields carry i18n keys, not resolved display text:
//
//	achievements.{Key}.name
//	achievements.{Key}.description              (flat achievements only)
//	achievements.{Key}.tiers.{N}.name           (N is 1-based)
//	achievements.{Key}.tiers.{N}.description    ({{threshold}} interpolates)
//
// Translations live in the frontend locale files. Anything published over
// Redis (AchievementUnlocked.TierName, notification payloads) carries the
// key verbatim — clients resolve at render time.
package achievements

import "github.com/recess/shared/events"

// Progress holds the current state for one achievement row.
type Progress struct {
	Tier     int
	Progress int
}

// Unlock is returned when progress crosses a tier threshold.
//
// TierName carries the i18n key for the tier's display name (not the resolved
// text). Downstream consumers — notification-service and WS clients — store
// and forward the key verbatim and resolve it at render time.
type Unlock struct {
	Key      string
	NewTier  int
	TierName string
	Progress int
}

// ProgressFunc computes the next progress value for an achievement given a
// finished game event. It returns the new value and whether the achievement
// was affected by this event.
//
//   - (value > cur.Progress, true)  — progress increased; may trigger tier-up
//   - (value, true) with value ≤ cur — progress was reset or adjusted;
//     persisted but cannot tier-up
//   - (_, false)                     — this event is not applicable; skip both
//
// Closures must be deterministic with respect to their inputs.
type ProgressFunc func(cur Progress, evt events.GameSessionFinished, outcome string) (int, bool)

// Tier represents a single milestone within an achievement.
type Tier struct {
	Threshold      int
	NameKey        string
	DescriptionKey string
}

// Definition describes an achievement that can be earned by players.
//
// ComputeProgress co-locates the rule with the metadata — the evaluator is a
// generic loop that delegates per-achievement logic to this closure.
type Definition struct {
	Key             string
	NameKey         string // i18n key for the display name
	DescriptionKey  string // i18n key for the description (flat achievements only)
	GameID          string // "" = global, "tictactoe" = game-specific, etc.
	Type            string // "flat" | "tiered"
	Tiers           []Tier
	ComputeProgress ProgressFunc
}

const (
	TypeFlat   = "flat"
	TypeTiered = "tiered"
)
