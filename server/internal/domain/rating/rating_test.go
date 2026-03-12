package rating_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/tableforge/server/internal/domain/rating"
)

// tolerance for floating-point comparisons.
const eps = 1e-6

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < eps
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newPair returns two players at the given MMRs, ready for a 1v1.
func newPair(mmrA, mmrB float64) (*rating.Player, *rating.Player) {
	a := rating.NewPlayer("a")
	b := rating.NewPlayer("b")
	a.MMR = mmrA
	a.DisplayRating = mmrA
	b.MMR = mmrB
	b.DisplayRating = mmrB
	return a, b
}

// play1v1 runs a single 1v1 match with rankA and rankB (1 = winner).
func play1v1(e *rating.Engine, a, b *rating.Player, rankA, rankB int) (map[string]float64, error) {
	result := &rating.MatchResult{
		Placements: []rating.Placement{
			{Team: &rating.Team{Players: []*rating.Player{a}}, Rank: rankA},
			{Team: &rating.Team{Players: []*rating.Player{b}}, Rank: rankB},
		},
	}
	return e.ProcessMatch(result)
}

// ---------------------------------------------------------------------------
// MatchResult.Validate
// ---------------------------------------------------------------------------

func TestMatchResult_Validate_TooFewTeams(t *testing.T) {
	mr := &rating.MatchResult{
		Placements: []rating.Placement{
			{Team: &rating.Team{Players: []*rating.Player{rating.NewPlayer("a")}}, Rank: 1},
		},
	}
	if err := mr.Validate(); err == nil {
		t.Error("expected error for single-team match")
	}
}

func TestMatchResult_Validate_InvalidRank(t *testing.T) {
	a, b := newPair(1500, 1500)
	mr := &rating.MatchResult{
		Placements: []rating.Placement{
			{Team: &rating.Team{Players: []*rating.Player{a}}, Rank: 0},
			{Team: &rating.Team{Players: []*rating.Player{b}}, Rank: 1},
		},
	}
	if err := mr.Validate(); err == nil {
		t.Error("expected error for rank 0")
	}
}

func TestMatchResult_Validate_EmptyTeam(t *testing.T) {
	mr := &rating.MatchResult{
		Placements: []rating.Placement{
			{Team: &rating.Team{Players: []*rating.Player{}}, Rank: 1},
			{Team: &rating.Team{Players: []*rating.Player{rating.NewPlayer("b")}}, Rank: 2},
		},
	}
	if err := mr.Validate(); err == nil {
		t.Error("expected error for empty team")
	}
}

func TestMatchResult_Validate_OK(t *testing.T) {
	a, b := newPair(1500, 1500)
	mr := &rating.MatchResult{
		Placements: []rating.Placement{
			{Team: &rating.Team{Players: []*rating.Player{a}}, Rank: 1},
			{Team: &rating.Team{Players: []*rating.Player{b}}, Rank: 2},
		},
	}
	if err := mr.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Team
// ---------------------------------------------------------------------------

func TestTeam_EffectiveMMR(t *testing.T) {
	p1 := rating.NewPlayer("p1")
	p2 := rating.NewPlayer("p2")
	p1.MMR = 1400
	p2.MMR = 1600
	team := &rating.Team{Players: []*rating.Player{p1, p2}}
	if got := team.EffectiveMMR(); !almostEqual(got, 1500) {
		t.Errorf("expected 1500, got %.2f", got)
	}
}

func TestTeam_EffectiveMMR_Empty(t *testing.T) {
	team := &rating.Team{}
	if got := team.EffectiveMMR(); got != 0 {
		t.Errorf("expected 0 for empty team, got %.2f", got)
	}
}

// ---------------------------------------------------------------------------
// DynamicK
// ---------------------------------------------------------------------------

func TestDynamicK_Calibration(t *testing.T) {
	cfg := rating.DefaultConfig()
	p := rating.NewPlayer("p")
	p.GamesPlayed = 0 // brand new player

	k := rating.DynamicK(cfg, p, 0)
	// No streak, no upset: K should equal KMax.
	if !almostEqual(k, cfg.KMax) {
		t.Errorf("expected KMax=%.1f for new player, got %.4f", cfg.KMax, k)
	}
}

func TestDynamicK_Veteran(t *testing.T) {
	cfg := rating.DefaultConfig()
	p := rating.NewPlayer("p")
	p.GamesPlayed = cfg.VeteranGames + 100 // well past veteran threshold

	k := rating.DynamicK(cfg, p, 0)
	// No streak, no upset: K should equal KMin.
	if !almostEqual(k, cfg.KMin) {
		t.Errorf("expected KMin=%.1f for veteran, got %.4f", cfg.KMin, k)
	}
}

func TestDynamicK_StreakMultiplier(t *testing.T) {
	cfg := rating.DefaultConfig()
	p := rating.NewPlayer("p")
	p.GamesPlayed = cfg.VeteranGames + 100 // use KMin as base for clarity
	p.WinStreak = cfg.StreakThreshold      // exactly at threshold

	kNoStreak := rating.DynamicK(cfg, p, 0)
	p.WinStreak = cfg.StreakThreshold + 3 // deeper streak
	kWithStreak := rating.DynamicK(cfg, p, 0)

	if kWithStreak <= kNoStreak {
		t.Errorf("expected streak to increase K: noStreak=%.4f withStreak=%.4f",
			kNoStreak, kWithStreak)
	}
	if kWithStreak > cfg.KMin*cfg.StreakMultiplierMax+eps {
		t.Errorf("streak K exceeds KMin*StreakMultiplierMax cap: %.4f", kWithStreak)
	}
}

func TestDynamicK_UpsetMultiplier(t *testing.T) {
	cfg := rating.DefaultConfig()
	p := rating.NewPlayer("p")
	p.GamesPlayed = 0 // calibration — use KMax as base

	kNormal := rating.DynamicK(cfg, p, 0)  // no surprise
	kUpset := rating.DynamicK(cfg, p, 0.9) // near-maximum surprise

	if kUpset <= kNormal {
		t.Errorf("expected upset to increase K: normal=%.4f upset=%.4f", kNormal, kUpset)
	}
	// Upset multiplier is capped at 2x, so K ≤ KMax * StreakMultiplierMax * 2.
	maxPossible := cfg.KMax * cfg.StreakMultiplierMax * 2
	if kUpset > maxPossible+eps {
		t.Errorf("upset K=%.4f exceeds theoretical max %.4f", kUpset, maxPossible)
	}
}

// ---------------------------------------------------------------------------
// Engine — 1v1
// ---------------------------------------------------------------------------

func TestEngine_1v1_WinnerGainsMMR(t *testing.T) {
	e := rating.NewDefaultEngine()
	a, b := newPair(1500, 1500)
	mmrBefore := a.MMR

	deltas, err := play1v1(e, a, b, 1, 2) // a wins
	if err != nil {
		t.Fatalf("ProcessMatch: %v", err)
	}

	if a.MMR <= mmrBefore {
		t.Errorf("winner MMR should increase: before=%.2f after=%.2f", mmrBefore, a.MMR)
	}
	if b.MMR >= 1500 {
		t.Errorf("loser MMR should decrease: got %.2f", b.MMR)
	}
	if deltas["a"] <= 0 {
		t.Errorf("winner delta should be positive, got %.4f", deltas["a"])
	}
	if deltas["b"] >= 0 {
		t.Errorf("loser delta should be negative, got %.4f", deltas["b"])
	}
}

func TestEngine_1v1_ZeroSum(t *testing.T) {
	// Total MMR in the system must be conserved after every match.
	e := rating.NewDefaultEngine()
	a, b := newPair(1500, 1500)
	totalBefore := a.MMR + b.MMR

	play1v1(e, a, b, 1, 2)

	totalAfter := a.MMR + b.MMR
	if math.Abs(totalAfter-totalBefore) > eps {
		t.Errorf("MMR not conserved: before=%.4f after=%.4f", totalBefore, totalAfter)
	}
}

func TestEngine_1v1_Draw(t *testing.T) {
	// Equal-rated players drawing should produce near-zero deltas.
	e := rating.NewDefaultEngine()
	a, b := newPair(1500, 1500)

	deltas, err := play1v1(e, a, b, 1, 1) // both rank 1 = draw
	if err != nil {
		t.Fatalf("ProcessMatch: %v", err)
	}

	if math.Abs(deltas["a"]) > eps {
		t.Errorf("expected ~0 delta for equal draw, got %.6f", deltas["a"])
	}
	if math.Abs(deltas["b"]) > eps {
		t.Errorf("expected ~0 delta for equal draw, got %.6f", deltas["b"])
	}
}

func TestEngine_1v1_Upset_LargerDelta(t *testing.T) {
	// A weaker player beating a stronger one should yield a larger MMR gain
	// than a stronger player beating a weaker one.
	e := rating.NewDefaultEngine()

	// Upset: low-rated beats high-rated.
	weak, strong := newPair(1200, 1800)
	deltasUpset, _ := play1v1(e, weak, strong, 1, 2)

	// Expected: high-rated beats low-rated.
	fav, underdog := newPair(1800, 1200)
	deltasExpected, _ := play1v1(e, fav, underdog, 1, 2)

	if deltasUpset["a"] <= deltasExpected["a"] {
		t.Errorf(
			"upset winner should gain more MMR: upset=%.4f expected=%.4f",
			deltasUpset["a"], deltasExpected["a"],
		)
	}
}

func TestEngine_1v1_GamesPlayedIncrement(t *testing.T) {
	e := rating.NewDefaultEngine()
	a, b := newPair(1500, 1500)

	play1v1(e, a, b, 1, 2)

	if a.GamesPlayed != 1 {
		t.Errorf("expected GamesPlayed=1 for a, got %d", a.GamesPlayed)
	}
	if b.GamesPlayed != 1 {
		t.Errorf("expected GamesPlayed=1 for b, got %d", b.GamesPlayed)
	}
}

func TestEngine_1v1_WinStreakUpdated(t *testing.T) {
	e := rating.NewDefaultEngine()
	a, b := newPair(1500, 1500)

	play1v1(e, a, b, 1, 2)

	if a.WinStreak != 1 {
		t.Errorf("expected WinStreak=1 for winner, got %d", a.WinStreak)
	}
	if b.LossStreak != 1 {
		t.Errorf("expected LossStreak=1 for loser, got %d", b.LossStreak)
	}
}

func TestEngine_1v1_DisplayRatingConverges(t *testing.T) {
	// After a win, DisplayRating should move toward the new MMR.
	e := rating.NewDefaultEngine()
	a, b := newPair(1500, 1500)
	displayBefore := a.DisplayRating

	play1v1(e, a, b, 1, 2)

	// Winner's MMR went up; DisplayRating should follow toward it.
	if a.DisplayRating <= displayBefore {
		t.Errorf("winner DisplayRating should increase: before=%.2f after=%.2f",
			displayBefore, a.DisplayRating)
	}
}

// ---------------------------------------------------------------------------
// Engine — multi-team
// ---------------------------------------------------------------------------

func TestEngine_MultiTeam_NormalisedDeltas(t *testing.T) {
	// In a 3-team match all at equal MMR, the winner (rank 1) should gain
	// and the last place (rank 3) should lose. Middle placement (rank 2)
	// should have a smaller absolute delta than the extremes.
	e := rating.NewDefaultEngine()

	p1 := rating.NewPlayer("p1")
	p2 := rating.NewPlayer("p2")
	p3 := rating.NewPlayer("p3")

	result := &rating.MatchResult{
		Placements: []rating.Placement{
			{Team: &rating.Team{Players: []*rating.Player{p1}}, Rank: 1},
			{Team: &rating.Team{Players: []*rating.Player{p2}}, Rank: 2},
			{Team: &rating.Team{Players: []*rating.Player{p3}}, Rank: 3},
		},
	}

	deltas, err := e.ProcessMatch(result)
	if err != nil {
		t.Fatalf("ProcessMatch: %v", err)
	}

	if deltas["p1"] <= 0 {
		t.Errorf("rank-1 player should gain MMR, delta=%.4f", deltas["p1"])
	}
	if deltas["p3"] >= 0 {
		t.Errorf("rank-3 player should lose MMR, delta=%.4f", deltas["p3"])
	}
	if math.Abs(deltas["p2"]) >= math.Abs(deltas["p1"]) {
		t.Errorf("mid-place delta should be smaller than winner delta: mid=%.4f win=%.4f",
			deltas["p2"], deltas["p1"])
	}
}

func TestEngine_MultiTeam_ZeroSum(t *testing.T) {
	// Total MMR must be conserved in a 4-team match.
	e := rating.NewDefaultEngine()

	players := make([]*rating.Player, 4)
	placements := make([]rating.Placement, 4)
	totalBefore := 0.0
	for i := range players {
		players[i] = rating.NewPlayer(fmt.Sprintf("p%d", i))
		players[i].MMR = float64(1400 + i*50)
		totalBefore += players[i].MMR
		placements[i] = rating.Placement{
			Team: &rating.Team{Players: []*rating.Player{players[i]}},
			Rank: i + 1,
		}
	}

	if _, err := e.ProcessMatch(&rating.MatchResult{Placements: placements}); err != nil {
		t.Fatalf("ProcessMatch: %v", err)
	}

	totalAfter := 0.0
	for _, p := range players {
		totalAfter += p.MMR
	}
	if math.Abs(totalAfter-totalBefore) > eps {
		t.Errorf("MMR not conserved in 4-team match: before=%.4f after=%.4f",
			totalBefore, totalAfter)
	}
}
