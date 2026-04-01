package rating

import (
	"math"

	"github.com/recess/shared/platform/mathutil"
)

// ---------------------------------------------------------------------------
// Multi-team generalised Elo engine
// ---------------------------------------------------------------------------
//
// # Problem statement
//
// Classic Elo is defined for two-player (or two-team) zero-sum games. We
// need a rating system that handles:
//   - 1v1, 2v2, 3v3, 4v4, 5v5 (two teams of varying size)
//   - Free-for-all / multi-team: 2v2v2v2, 1v1v1v1, FFA solo, etc.
//   - Draws at any placement tier
//
// # Generalisation via pairwise decomposition
//
// An N-team match is decomposed into C(N,2) = N*(N-1)/2 virtual
// head-to-head pairings. Standard Elo is applied to each pair.
//
// For each ordered pair of teams (A, B):
//
//  1. Expected score of A vs B:
//
//     E_AB = 1 / (1 + 10^((MMR_B - MMR_A) / Scale))       ... (1)
//
//     where Scale = 400 by default (logistic sigmoid).
//
//  2. Actual score of A vs B based on placements:
//
//     S_AB = 1.0   if rank_A < rank_B    (A finished higher)
//     S_AB = 0.5   if rank_A == rank_B   (draw / tie)
//     S_AB = 0.0   if rank_A > rank_B    (A finished lower)
//
//  3. Per-pairing Elo delta for team A (before K):
//
//     Δ_AB = S_AB - E_AB                                   ... (2)
//
//  4. Normalised aggregate delta for team A across all opponents:
//
//     Δ_A = (1 / (N-1)) * Σ_{B ≠ A} Δ_AB                 ... (3)
//
//     The 1/(N-1) factor keeps rating changes comparable in magnitude
//     regardless of the number of teams. Without it an 8-team FFA would
//     produce ~7× larger swings than a 1v1.
//
//  5. Per-player rating update:
//
//     K_i        = DynamicK(player_i, |Δ_A|)
//     MMR_i     += K_i * Δ_A
//     Display_i += ConvergenceRate * (MMR_i - Display_i)   ... (4)
//
// # Properties
//
//   - Conservation: total Elo in the system is conserved (zero-sum per pairing).
//   - 1v1 consistency: when N=2 the formula collapses to standard Elo.
//   - Ordering-awareness: finishing 1st out of 8 rewards more than 4th.

// Engine is the stateless entry point for processing match results.
type Engine struct {
	Cfg Config
}

// NewEngine creates an Engine with the given configuration.
func NewEngine(cfg Config) *Engine {
	return &Engine{Cfg: cfg}
}

// NewDefaultEngine creates an Engine with DefaultConfig.
func NewDefaultEngine() *Engine {
	return NewEngine(DefaultConfig())
}

// ProcessMatch applies rating changes for every player in the match.
// It mutates the Player values in-place and returns a map from player ID
// to the MMR delta applied.
func (e *Engine) ProcessMatch(result *MatchResult) (map[string]float64, error) {
	if err := result.Validate(); err != nil {
		return nil, err
	}

	n := len(result.Placements)
	deltas := make(map[string]float64)

	// --- Step 1: compute per-team aggregate Elo delta ---
	teamDeltas := make([]float64, n)
	for i := 0; i < n; i++ {
		sumDelta := 0.0
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			mmrA := result.Placements[i].Team.EffectiveMMR()
			mmrB := result.Placements[j].Team.EffectiveMMR()

			expected := mathutil.Sigmoid(mmrA-mmrB, e.Cfg.EloScale) // E_AB  eq(1)
			actual := actualScore(result.Placements[i].Rank, result.Placements[j].Rank)

			sumDelta += actual - expected // Δ_AB  eq(2)
		}
		teamDeltas[i] = sumDelta / float64(n-1) // eq(3)
	}

	// --- Step 2: apply per-player updates ---
	for i, pl := range result.Placements {
		delta := teamDeltas[i]
		surpriseFactor := math.Abs(delta) // |Δ_A|

		for _, p := range pl.Team.Players {
			k := DynamicK(e.Cfg, p, surpriseFactor)
			mmrChange := k * delta

			p.MMR += mmrChange
			p.GamesPlayed++

			// Converge display toward hidden MMR — eq(4).
			p.DisplayRating += e.Cfg.DisplayConvergenceRate * (p.MMR - p.DisplayRating)

			// Update streaks: positive delta = win, negative = loss.
			if delta > 0 {
				p.WinStreak++
				p.LossStreak = 0
			} else if delta < 0 {
				p.LossStreak++
				p.WinStreak = 0
			}

			deltas[p.ID] = mmrChange
		}
	}

	return deltas, nil
}

// actualScore returns S_AB: the score of team A against team B given their
// respective placements (lower rank = better finish).
//
//	rank_A < rank_B  →  1.0  (A won)
//	rank_A == rank_B →  0.5  (draw)
//	rank_A > rank_B  →  0.0  (A lost)
func actualScore(rankA, rankB int) float64 {
	if rankA < rankB {
		return 1.0
	}
	if rankA == rankB {
		return 0.5
	}
	return 0.0
}
