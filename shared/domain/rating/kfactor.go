package rating

import (
	"math"

	"github.com/recess/shared/platform/mathutil"
)

// ---------------------------------------------------------------------------
// Dynamic K-factor
// ---------------------------------------------------------------------------
//
// The K-factor controls how much a single game can shift a player's rating.
// A static K leads to two problems: new players converge too slowly and
// veteran ratings swing too much. We therefore compute K dynamically from
// three independent signals, then multiply them together.
//
// # Base K  (experience curve)
//
// We model the transition from calibration (high K) to veteran (low K) as
// a smooth linear decay:
//
//	if gamesPlayed <= CalibrationGames:
//	    K_base = KMax
//	else:
//	    t = (gamesPlayed - CalibrationGames) / (VeteranGames - CalibrationGames)
//	    t = clamp(t, 0, 1)
//	    K_base = KMin + (KDefault - KMin) * (1 - t)
//
// During calibration the system needs to find the player's true skill as
// quickly as possible, so K_base = KMax (48 by default). After calibration
// it linearly interpolates from KDefault (24) down to KMin (10) by the
// time the player reaches VeteranGames.
//
// # Streak multiplier
//
// When a player is on a winning or losing streak of length L >=
// StreakThreshold, the system suspects the current MMR is lagging behind
// reality. The multiplier accelerates convergence:
//
//	extra = min(StreakMultiplierMax, 1 + 0.1*(L - StreakThreshold))
//	K_streak = K_base * extra
//
// # Upset multiplier  (per-game)
//
// When the actual outcome deviates significantly from the expected outcome
// (|S - E| is large), K is nudged upward to correct faster:
//
//	upsetFactor = clamp(1 + |S - E|, 1, 2)
//	K_final = K_streak * upsetFactor
//
// An extreme upset (expected 10%, actual 100%) can almost double the
// rating change, helping correct under-/over-rated players quickly.

// DynamicK computes the per-player K-factor for a single game.
// surpriseFactor is |S - E| from the Elo calculation (0 ≤ x ≤ 1).
func DynamicK(cfg Config, p *Player, surpriseFactor float64) float64 {
	// --- base K from experience curve ---
	kBase := cfg.KMax
	if p.GamesPlayed > cfg.CalibrationGames {
		t := float64(p.GamesPlayed-cfg.CalibrationGames) / float64(cfg.VeteranGames-cfg.CalibrationGames)
		t = mathutil.Clamp(t, 0, 1)
		kBase = cfg.KMin + (cfg.KDefault-cfg.KMin)*(1-t)
	}

	// --- streak multiplier ---
	streak := p.WinStreak
	if p.LossStreak > streak {
		streak = p.LossStreak
	}
	streakMul := 1.0
	if streak >= cfg.StreakThreshold {
		streakMul = math.Min(cfg.StreakMultiplierMax, 1.0+0.1*float64(streak-cfg.StreakThreshold))
	}

	// --- upset multiplier ---
	upsetMul := mathutil.Clamp(1.0+surpriseFactor, 1.0, 2.0)

	return kBase * streakMul * upsetMul
}
