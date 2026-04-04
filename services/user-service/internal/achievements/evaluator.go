package achievements

import "github.com/recess/shared/events"

// Progress holds the current state for one achievement row.
type Progress struct {
	Tier     int
	Progress int
}

// Unlock is returned when progress crosses a tier threshold.
type Unlock struct {
	Key      string
	NewTier  int
	TierName string
	Progress int
}

// Evaluate checks all applicable achievements for a single player given the
// game session event and the player's current achievement state.
// Returns a list of new unlocks/tier-ups.
func Evaluate(
	evt events.GameSessionFinished,
	playerOutcome string, // "win" | "loss" | "draw" | "forfeit"
	currentState map[string]Progress,
	winStreak int,
) []Unlock {
	defs := ForGame(evt.GameID)
	var unlocks []Unlock

	for _, def := range defs {
		cur := currentState[def.Key]

		var newProgress int
		switch def.Key {
		case "games_played":
			newProgress = cur.Progress + 1
		case "games_won":
			if playerOutcome == "win" {
				newProgress = cur.Progress + 1
			} else {
				continue
			}
		case "win_streak":
			if playerOutcome == "win" {
				newProgress = winStreak
			} else {
				continue
			}
		case "first_draw":
			if playerOutcome == "draw" {
				newProgress = 1
			} else {
				continue
			}
		case "ttt_perfect_game":
			if evt.GameID != "tictactoe" || playerOutcome != "win" {
				continue
			}
			// Perfect game = 3 moves for the winner (seats alternate).
			// Count moves by this player from the event duration isn't available,
			// but we rely on the total duration: a perfect TTT win = 5 total moves
			// (3 winner + 2 loser), so the winner used exactly 3.
			// The caller should pass move count in the event, but since we only
			// have duration_secs, we check if duration < a threshold indicating
			// minimal moves. For now, we'll track this via a dedicated field
			// that the consumer computes from session data.
			// Simplified: if the session ended with "win" and DurationSecs is
			// very short (< 30s for a 3-move game), count it.
			// TODO: improve when move count is available in the event.
			if evt.DurationSecs > 60 {
				continue
			}
			newProgress = 1
		case "ttt_games_played":
			if evt.GameID != "tictactoe" {
				continue
			}
			newProgress = cur.Progress + 1
		default:
			continue
		}

		u := checkTierUp(def, cur, newProgress)
		if u != nil {
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
				TierName: def.Tiers[0].Name,
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
			TierName: def.Tiers[newTier-1].Name,
			Progress: newProgress,
		}
	}

	// Progress changed but no tier up — still need to persist progress.
	// Return nil for unlock, but the consumer will update progress separately.
	return nil
}

// ComputeNewProgress returns the updated progress value for a definition,
// without checking tier thresholds. Used by the consumer to persist progress
// even when no unlock happens.
func ComputeNewProgress(
	def Definition,
	cur Progress,
	evt events.GameSessionFinished,
	playerOutcome string,
	winStreak int,
) (int, bool) {
	switch def.Key {
	case "games_played":
		return cur.Progress + 1, true
	case "games_won":
		if playerOutcome == "win" {
			return cur.Progress + 1, true
		}
	case "win_streak":
		if playerOutcome == "win" {
			return winStreak, true
		}
		// Reset streak on non-win: progress goes to 0 but tier stays.
		if cur.Progress > 0 {
			return 0, true
		}
	case "first_draw":
		if playerOutcome == "draw" {
			return 1, true
		}
	case "ttt_perfect_game":
		if evt.GameID == "tictactoe" && playerOutcome == "win" && evt.DurationSecs <= 60 {
			return 1, true
		}
	case "ttt_games_played":
		if evt.GameID == "tictactoe" {
			return cur.Progress + 1, true
		}
	}
	return cur.Progress, false
}
