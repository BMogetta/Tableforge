package achievements

import "github.com/recess/shared/events"

// Evaluate checks all applicable achievements for a single player given the
// finished-game event and the player's current achievement state. Returns a
// list of new unlocks / tier-ups.
//
// Definitions without a ComputeProgress closure are silently skipped so
// metadata-only entries (e.g. externally-awarded achievements) can coexist
// with event-driven ones.
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

// ComputeNewProgress returns the updated progress value for a Definition,
// without checking tier thresholds. Used by consumers to persist progress
// even when no unlock happens. Falls back to (cur.Progress, false) for
// definitions with no closure.
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

// checkTierUp determines whether newProgress crosses a tier threshold that
// cur has not already reached, and returns the resulting Unlock. Never
// demotes: progress resets below cur.Tier's threshold keep the tier.
func checkTierUp(def Definition, cur Progress, newProgress int) *Unlock {
	if def.Type == TypeFlat {
		if cur.Tier >= 1 {
			return nil
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

	maxTier := len(def.Tiers)
	if cur.Tier >= maxTier {
		return nil
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
