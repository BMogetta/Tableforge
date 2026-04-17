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
//   - (value, true) with value ≤ cur — progress was reset or adjusted; persisted
//     but cannot tier-up
//   - (_, false)                     — this event is not applicable; skip both
//
// The closure receives the player's pre-event Progress, the session event,
// and the outcome string ("win" | "loss" | "draw" | "forfeit"). It must be
// deterministic with respect to those inputs.
type ProgressFunc func(cur Progress, evt events.GameSessionFinished, outcome string) (int, bool)

// Evaluate checks all applicable achievements for a single player given the
// game session event and the player's current achievement state.
// Returns a list of new unlocks/tier-ups.
//
// The generic loop: every Definition.ComputeProgress closure decides whether
// and how this event affects its achievement. Adding a new achievement is a
// single Registry entry — no switch here to edit.
func Evaluate(
	evt events.GameSessionFinished,
	playerOutcome string,
	currentState map[string]Progress,
) []Unlock {
	defs := ForGame(evt.GameID)
	var unlocks []Unlock

	for _, def := range defs {
		if def.ComputeProgress == nil {
			continue
		}
		cur := currentState[def.Key]
		newProgress, applicable := def.ComputeProgress(cur, evt, playerOutcome)
		if !applicable {
			continue
		}
		if u := checkTierUp(def, cur, newProgress); u != nil {
			unlocks = append(unlocks, *u)
		}
	}

	return unlocks
}

// checkTierUp determines if the new progress crosses a tier threshold.
// Returns nil if no tier change.
func checkTierUp(def Definition, cur Progress, newProgress int) *Unlock {
	if def.Type == TypeFlat {
		if cur.Tier >= 1 {
			return nil // already unlocked
		}
		if newProgress >= def.Tiers[0].Threshold {
			return &Unlock{
				Key:      def.Key,
				NewTier:  1,
				TierName: def.Tiers[0].NameKey,
				Progress: newProgress,
			}
		}
		return nil
	}

	// Tiered: find the highest tier crossed by newProgress.
	maxTier := len(def.Tiers)
	if cur.Tier >= maxTier {
		return nil // already at max tier
	}

	newTier := cur.Tier
	for i := cur.Tier; i < maxTier; i++ {
		if newProgress >= def.Tiers[i].Threshold {
			newTier = i + 1
		} else {
			break
		}
	}

	if newTier > cur.Tier {
		return &Unlock{
			Key:      def.Key,
			NewTier:  newTier,
			TierName: def.Tiers[newTier-1].NameKey,
			Progress: newProgress,
		}
	}
	return nil
}

// ComputeNewProgress returns the updated progress value for a definition,
// without checking tier thresholds. Used by the consumer to persist progress
// even when no unlock happens.
//
// Thin wrapper around Definition.ComputeProgress that keeps the old call
// shape and returns the cur.Progress + false sentinel for Definitions with
// no closure (e.g. future metadata-only entries).
func ComputeNewProgress(
	def Definition,
	cur Progress,
	evt events.GameSessionFinished,
	playerOutcome string,
) (int, bool) {
	if def.ComputeProgress == nil {
		return cur.Progress, false
	}
	return def.ComputeProgress(cur, evt, playerOutcome)
}
