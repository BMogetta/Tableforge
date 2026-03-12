// Command simulate runs long-duration simulations to validate the rating and
// matchmaking systems before deploying changes to production.
//
// It creates a pool of simulated players with normally-distributed "true
// skill", repeatedly forms matches via the matchmaker, resolves outcomes
// probabilistically, feeds results into the Elo engine, and then measures
// how closely the computed MMRs converge to the ground-truth skill values.
//
// # Usage
//
//	go run ./cmd/simulate [flags]
//
// # Flags
//
//	-players    Number of simulated players (default 200)
//	-matches    Number of matches to simulate (default 10000)
//	-team-size  Players per team: 1..5 (default 1)
//	-teams      Teams per match: 2..8 (default 2)
//	-noise      Per-game performance noise σ (default 150)
//	-seed       RNG seed; 0 = random (default 0)
//	-verbose    Print convergence progress every 500 matches
//
// # Examples
//
//	# Standard 1v1, 10k matches
//	go run ./cmd/simulate -players 100 -matches 10000
//
//	# 5v5, 10k matches
//	go run ./cmd/simulate -players 200 -matches 10000 -team-size 5 -teams 2
//
//	# 2v2v2v2 free-for-all
//	go run ./cmd/simulate -players 100 -matches 5000 -team-size 2 -teams 4
//
//	# Solo FFA (8 players per match)
//	go run ./cmd/simulate -players 200 -matches 10000 -team-size 1 -teams 8
//
//	# Deterministic run for regression testing
//	go run ./cmd/simulate -seed 42 -verbose
//
// # Interpreting results
//
// | Metric            | Healthy range    | Meaning                          |
// |-------------------|------------------|----------------------------------|
// | Avg MMR Error     | <80 after 5k     | System finds true skill          |
// | Rating σ          | Close to skill σ | Distribution is not compressed   |
// | Match quality     | >0.7             | Matchmaker produces fair games   |
// | Q4 win-rate > Q1  | Always           | Better players win more          |
// | Convergence curve | Decreasing       | System improves over time        |
package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/tableforge/server/internal/domain/matchmaking"
	"github.com/tableforge/server/internal/domain/rating"
	"github.com/tableforge/server/internal/platform/mathutil"
)

func main() {
	cfg := defaultSimConfig()

	flag.IntVar(&cfg.numPlayers, "players", cfg.numPlayers, "number of simulated players")
	flag.IntVar(&cfg.numMatches, "matches", cfg.numMatches, "number of matches to simulate")
	flag.IntVar(&cfg.playersPerTeam, "team-size", cfg.playersPerTeam, "players per team")
	flag.IntVar(&cfg.teamsPerMatch, "teams", cfg.teamsPerMatch, "teams per match")
	flag.Float64Var(&cfg.noiseFactor, "noise", cfg.noiseFactor, "per-game performance noise σ")
	flag.Int64Var(&cfg.seed, "seed", cfg.seed, "RNG seed (0 = random)")
	flag.BoolVar(&cfg.verbose, "verbose", false, "print convergence progress every 500 matches")
	flag.Parse()

	fmt.Println("Starting simulation...")
	fmt.Printf("Mode: %dv%d", cfg.playersPerTeam, cfg.playersPerTeam)
	if cfg.teamsPerMatch > 2 {
		fmt.Printf(" x%d teams", cfg.teamsPerMatch)
	}
	fmt.Printf(" | %d players | %d matches | noise=%.0f\n\n",
		cfg.numPlayers, cfg.numMatches, cfg.noiseFactor)

	report := runSimulation(cfg)
	fmt.Println(report)
}

// ---------------------------------------------------------------------------
// Simulation config
// ---------------------------------------------------------------------------

type simConfig struct {
	numPlayers      int
	trueSkillMean   float64
	trueSkillStdDev float64
	numMatches      int
	playersPerTeam  int
	teamsPerMatch   int
	noiseFactor     float64
	seed            int64
	verbose         bool
}

func defaultSimConfig() simConfig {
	return simConfig{
		numPlayers:      200,
		trueSkillMean:   1500,
		trueSkillStdDev: 300,
		numMatches:      10000,
		playersPerTeam:  1,
		teamsPerMatch:   2,
		noiseFactor:     150,
		seed:            0,
	}
}

// ---------------------------------------------------------------------------
// Simulated player
// ---------------------------------------------------------------------------

// simPlayer wraps a rating.Player with its hidden true skill.
// The simulation uses trueSkill as a ground-truth oracle to measure how
// accurately the Elo engine recovers each player's real ability.
type simPlayer struct {
	player    *rating.Player
	trueSkill float64
}

// ---------------------------------------------------------------------------
// Report
// ---------------------------------------------------------------------------

type report struct {
	cfg               simConfig
	matchesPlayed     int
	avgMMRError       float64
	medianMMRError    float64
	maxMMRError       float64
	ratingStdDev      float64
	convergenceByGame []float64 // sampled every 100 matches
	winRateByQuartile [4]float64
	avgMatchQuality   float64
	elapsed           time.Duration
}

func (r *report) String() string {
	var sb strings.Builder
	sb.WriteString("=== Simulation Report ===\n")
	sb.WriteString(fmt.Sprintf("Players: %d | Matches: %d | Mode: %dv%d",
		r.cfg.numPlayers, r.matchesPlayed,
		r.cfg.playersPerTeam, r.cfg.playersPerTeam))
	if r.cfg.teamsPerMatch > 2 {
		sb.WriteString(fmt.Sprintf(" x%d teams", r.cfg.teamsPerMatch))
	}
	sb.WriteString(fmt.Sprintf("\nNoise factor: %.0f\n", r.cfg.noiseFactor))
	sb.WriteString(fmt.Sprintf("Elapsed: %s\n\n", r.elapsed.Round(time.Millisecond)))

	sb.WriteString(fmt.Sprintf("MMR Error  — avg: %.1f  median: %.1f  max: %.1f\n",
		r.avgMMRError, r.medianMMRError, r.maxMMRError))
	sb.WriteString(fmt.Sprintf("Rating σ   — %.1f  (true skill σ = %.1f)\n",
		r.ratingStdDev, r.cfg.trueSkillStdDev))
	sb.WriteString(fmt.Sprintf("Match quality avg: %.3f\n", r.avgMatchQuality))
	sb.WriteString(fmt.Sprintf(
		"Win-rate by true-skill quartile: Q1=%.2f  Q2=%.2f  Q3=%.2f  Q4=%.2f\n",
		r.winRateByQuartile[0], r.winRateByQuartile[1],
		r.winRateByQuartile[2], r.winRateByQuartile[3]))

	if len(r.convergenceByGame) > 0 {
		sb.WriteString("Convergence (avg MMR error every 100 matches):\n")
		for i, v := range r.convergenceByGame {
			sb.WriteString(fmt.Sprintf("  %5d matches: %.1f\n", (i+1)*100, v))
		}
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Runner
// ---------------------------------------------------------------------------

func runSimulation(cfg simConfig) *report {
	start := time.Now()

	rng := rand.New(rand.NewSource(cfg.seed))
	if cfg.seed == 0 {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	// Build player pool with normally-distributed true skill.
	pool := make([]*simPlayer, cfg.numPlayers)
	for i := 0; i < cfg.numPlayers; i++ {
		trueSkill := cfg.trueSkillMean + rng.NormFloat64()*cfg.trueSkillStdDev
		p := rating.NewPlayer(fmt.Sprintf("p%04d", i))
		pool[i] = &simPlayer{player: p, trueSkill: trueSkill}
	}

	engine := rating.NewDefaultEngine()
	mmCfg := matchmaking.QueueConfig{
		PlayersPerTeam:  cfg.playersPerTeam,
		TeamsPerMatch:   cfg.teamsPerMatch,
		BaseSpread:      500, // relaxed for simulation throughput
		SpreadPerSecond: 10,
		MinQuality:      0, // accept all matches in simulation
	}
	mm := matchmaking.NewMatchmaker(mmCfg)

	totalPerMatch := cfg.playersPerTeam * cfg.teamsPerMatch

	// Assign each player to a true-skill quartile for win-rate tracking.
	quartileMap := buildQuartileMap(pool)
	var quartileWins [4]int
	var quartileGames [4]int

	matchesPlayed := 0
	var convergence []float64
	var totalQuality float64

	for matchesPlayed < cfg.numMatches {
		// Enqueue a random subset — enough candidates for several matches.
		perm := rng.Perm(len(pool))
		enqueued := 0
		for _, idx := range perm {
			mm.Enqueue(pool[idx].player)
			enqueued++
			if enqueued >= totalPerMatch*3 {
				break
			}
		}

		matches := mm.FindMatches(time.Now())
		for _, m := range matches {
			if matchesPlayed >= cfg.numMatches {
				break
			}

			// Resolve outcome: each player's performance = trueSkill + noise.
			teamPerf := make([]float64, len(m.Teams))
			for ti, team := range m.Teams {
				perf := 0.0
				for _, p := range team {
					sp := findSim(pool, p.ID)
					perf += sp.trueSkill + rng.NormFloat64()*cfg.noiseFactor
				}
				teamPerf[ti] = perf / float64(len(team))
			}

			// Higher performance → lower (better) rank.
			ranks := rankFromPerformance(teamPerf)

			placements := make([]rating.Placement, len(m.Teams))
			for ti, team := range m.Teams {
				t := &rating.Team{Players: team}
				placements[ti] = rating.Placement{Team: t, Rank: ranks[ti]}
			}

			result := &rating.MatchResult{Placements: placements}
			if _, err := engine.ProcessMatch(result); err != nil {
				continue
			}

			totalQuality += m.Quality
			matchesPlayed++

			// Track per-quartile win counts (rank 1 = win).
			for ti, team := range m.Teams {
				for _, p := range team {
					q := quartileMap[p.ID]
					quartileGames[q]++
					if ranks[ti] == 1 {
						quartileWins[q]++
					}
				}
			}

			// Sample convergence every 100 matches.
			if matchesPlayed%100 == 0 {
				convergence = append(convergence, avgError(pool))
			}

			if cfg.verbose && matchesPlayed%500 == 0 {
				fmt.Printf("[%d/%d] avg error = %.1f\n",
					matchesPlayed, cfg.numMatches, avgError(pool))
			}
		}
	}

	// Compile final statistics.
	errors := make([]float64, len(pool))
	mmrs := make([]float64, len(pool))
	for i, sp := range pool {
		errors[i] = math.Abs(sp.player.MMR - sp.trueSkill)
		mmrs[i] = sp.player.MMR
	}
	sort.Float64s(errors)

	var qWinRates [4]float64
	for q := 0; q < 4; q++ {
		if quartileGames[q] > 0 {
			qWinRates[q] = float64(quartileWins[q]) / float64(quartileGames[q])
		}
	}

	return &report{
		cfg:               cfg,
		matchesPlayed:     matchesPlayed,
		avgMMRError:       mathutil.Mean(errors),
		medianMMRError:    errors[len(errors)/2],
		maxMMRError:       errors[len(errors)-1],
		ratingStdDev:      math.Sqrt(mathutil.Variance(mmrs)),
		convergenceByGame: convergence,
		winRateByQuartile: qWinRates,
		avgMatchQuality:   totalQuality / float64(matchesPlayed),
		elapsed:           time.Since(start),
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildQuartileMap assigns each player to a true-skill quartile (0..3).
// Q0 = weakest 25%, Q3 = strongest 25%.
func buildQuartileMap(pool []*simPlayer) map[string]int {
	sorted := make([]*simPlayer, len(pool))
	copy(sorted, pool)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].trueSkill < sorted[j].trueSkill
	})
	m := make(map[string]int, len(pool))
	qSize := len(sorted) / 4
	for i, sp := range sorted {
		q := i / qSize
		if q > 3 {
			q = 3
		}
		m[sp.player.ID] = q
	}
	return m
}

// findSim looks up a simPlayer by player ID.
func findSim(pool []*simPlayer, id string) *simPlayer {
	for _, sp := range pool {
		if sp.player.ID == id {
			return sp
		}
	}
	return nil
}

// avgError returns the mean absolute difference between MMR and true skill
// across all players in the pool.
func avgError(pool []*simPlayer) float64 {
	sum := 0.0
	for _, sp := range pool {
		sum += math.Abs(sp.player.MMR - sp.trueSkill)
	}
	return sum / float64(len(pool))
}

// rankFromPerformance converts a slice of performance scores into 1-based
// ranks. Higher performance → rank 1 (first place).
func rankFromPerformance(perf []float64) []int {
	type kv struct {
		idx  int
		perf float64
	}
	sorted := make([]kv, len(perf))
	for i, p := range perf {
		sorted[i] = kv{idx: i, perf: p}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].perf > sorted[j].perf // descending: higher = better
	})
	ranks := make([]int, len(perf))
	for rank, s := range sorted {
		ranks[s.idx] = rank + 1
	}
	return ranks
}
