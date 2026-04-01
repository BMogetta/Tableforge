package matchmaking_test

import (
	"testing"
	"time"

	"github.com/recess/shared/domain/matchmaking"
	"github.com/recess/shared/domain/rating"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newPlayer creates a rating.Player with the given MMR.
func newPlayer(id string, mmr float64) *rating.Player {
	p := rating.NewPlayer(id)
	p.MMR = mmr
	return p
}

// cfg1v1 returns a QueueConfig for standard 1v1 with no quality gate.
func cfg1v1() matchmaking.QueueConfig {
	return matchmaking.QueueConfig{
		PlayersPerTeam:  1,
		TeamsPerMatch:   2,
		BaseSpread:      500,
		SpreadPerSecond: 0,
		MinQuality:      0,
	}
}

// now is a fixed reference time used across tests for reproducibility.
var now = time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

// ---------------------------------------------------------------------------
// Not enough players
// ---------------------------------------------------------------------------

func TestFindMatches_NotEnoughPlayers(t *testing.T) {
	mm := matchmaking.NewMatchmaker(cfg1v1())
	mm.Enqueue(newPlayer("a", 1500))
	// Only 1 player — need 2 for a 1v1.
	matches := mm.FindMatches(now)
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

// ---------------------------------------------------------------------------
// Basic 1v1
// ---------------------------------------------------------------------------

func TestFindMatches_Basic1v1(t *testing.T) {
	mm := matchmaking.NewMatchmaker(cfg1v1())
	mm.Enqueue(newPlayer("a", 1500))
	mm.Enqueue(newPlayer("b", 1500))

	matches := mm.FindMatches(now)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	m := matches[0]
	if len(m.Teams) != 2 {
		t.Errorf("expected 2 teams, got %d", len(m.Teams))
	}
	if len(m.Teams[0]) != 1 || len(m.Teams[1]) != 1 {
		t.Errorf("expected 1 player per team")
	}
}

func TestFindMatches_RemovesMatchedPlayers(t *testing.T) {
	mm := matchmaking.NewMatchmaker(cfg1v1())
	mm.Enqueue(newPlayer("a", 1500))
	mm.Enqueue(newPlayer("b", 1500))

	mm.FindMatches(now)

	// Both players were matched — queue should be empty.
	if mm.QueueSize() != 0 {
		t.Errorf("expected empty queue after match, got size %d", mm.QueueSize())
	}
}

func TestFindMatches_UnmatchedPlayersRemainInQueue(t *testing.T) {
	mm := matchmaking.NewMatchmaker(cfg1v1())
	// 3 players for a 1v1 — one should remain.
	mm.Enqueue(newPlayer("a", 1500))
	mm.Enqueue(newPlayer("b", 1500))
	mm.Enqueue(newPlayer("c", 1500))

	mm.FindMatches(now)

	if mm.QueueSize() != 1 {
		t.Errorf("expected 1 player remaining, got %d", mm.QueueSize())
	}
}

func TestFindMatches_MultipleMatches(t *testing.T) {
	mm := matchmaking.NewMatchmaker(cfg1v1())
	for i := 0; i < 6; i++ {
		mm.Enqueue(newPlayer(string(rune('a'+i)), float64(1400+i*20)))
	}

	matches := mm.FindMatches(now)
	if len(matches) != 3 {
		t.Errorf("expected 3 matches from 6 players, got %d", len(matches))
	}
	if mm.QueueSize() != 0 {
		t.Errorf("expected empty queue, got %d", mm.QueueSize())
	}
}

// ---------------------------------------------------------------------------
// Quality gate
// ---------------------------------------------------------------------------

func TestFindMatches_QualityGate_Rejects(t *testing.T) {
	cfg := matchmaking.QueueConfig{
		PlayersPerTeam:  1,
		TeamsPerMatch:   2,
		BaseSpread:      200,
		SpreadPerSecond: 0,
		MinQuality:      0.9, // very strict
	}
	mm := matchmaking.NewMatchmaker(cfg)
	// 500 MMR gap — quality will be (1 - 500/200) = negative → clamped to 0.
	mm.EnqueueAt(newPlayer("a", 1000), now)
	mm.EnqueueAt(newPlayer("b", 1500), now)

	matches := mm.FindMatches(now)
	if len(matches) != 0 {
		t.Errorf("expected match to be rejected by quality gate, got %d", len(matches))
	}
	// Both players should still be in the queue.
	if mm.QueueSize() != 2 {
		t.Errorf("expected 2 players still queued, got %d", mm.QueueSize())
	}
}

func TestFindMatches_QualityGate_Accepts(t *testing.T) {
	cfg := matchmaking.QueueConfig{
		PlayersPerTeam:  1,
		TeamsPerMatch:   2,
		BaseSpread:      200,
		SpreadPerSecond: 0,
		MinQuality:      0.4,
	}
	mm := matchmaking.NewMatchmaker(cfg)
	// 50 MMR gap — quality = 1 - 50/200 = 0.75, above threshold.
	mm.EnqueueAt(newPlayer("a", 1500), now)
	mm.EnqueueAt(newPlayer("b", 1550), now)

	matches := mm.FindMatches(now)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Quality < 0.4 {
		t.Errorf("match quality %.3f is below MinQuality 0.4", matches[0].Quality)
	}
}

// ---------------------------------------------------------------------------
// Spread relaxation with wait time
// ---------------------------------------------------------------------------

func TestFindMatches_SpreadRelaxation_AllowsLargeGapAfterWait(t *testing.T) {
	cfg := matchmaking.QueueConfig{
		PlayersPerTeam:  1,
		TeamsPerMatch:   2,
		BaseSpread:      100,
		SpreadPerSecond: 10, // +10 MMR spread per second
		MinQuality:      0.4,
	}
	mm := matchmaking.NewMatchmaker(cfg)

	// 400 MMR gap. At BaseSpread=100 quality = 1 - 400/100 = -3 → rejected.
	// After 40s wait: spread = 100 + 10*40 = 500, quality = 1 - 400/500 = 0.2 → still below 0.4.
	// After 70s wait: spread = 100 + 10*70 = 800, quality = 1 - 400/800 = 0.5 → accepted.
	joinTime := now.Add(-70 * time.Second)
	mm.EnqueueAt(newPlayer("a", 1100), joinTime)
	mm.EnqueueAt(newPlayer("b", 1500), joinTime)

	matches := mm.FindMatches(now)
	if len(matches) != 1 {
		t.Fatalf("expected relaxed spread to allow match after 70s wait, got %d matches", len(matches))
	}
}

func TestFindMatches_SpreadRelaxation_RejectsWithoutWait(t *testing.T) {
	cfg := matchmaking.QueueConfig{
		PlayersPerTeam:  1,
		TeamsPerMatch:   2,
		BaseSpread:      100,
		SpreadPerSecond: 10,
		MinQuality:      0.4,
	}
	mm := matchmaking.NewMatchmaker(cfg)

	// Same 400 MMR gap but players just joined — spread still 100, rejected.
	mm.EnqueueAt(newPlayer("a", 1100), now)
	mm.EnqueueAt(newPlayer("b", 1500), now)

	matches := mm.FindMatches(now)
	if len(matches) != 0 {
		t.Errorf("expected match to be rejected without wait time, got %d", len(matches))
	}
}

// ---------------------------------------------------------------------------
// Snake draft balance
// ---------------------------------------------------------------------------

func TestFindMatches_SnakeDraft_TeamsAreBalanced(t *testing.T) {
	// 2v2 with 4 players sorted by MMR: 1200, 1300, 1400, 1500.
	// Snake draft: T1=1200, T2=1300, T2=1400, T1=1500.
	// T1 avg = (1200+1500)/2 = 1350
	// T2 avg = (1300+1400)/2 = 1350
	// Both teams should have equal average MMR.
	cfg := matchmaking.QueueConfig{
		PlayersPerTeam:  2,
		TeamsPerMatch:   2,
		BaseSpread:      1000,
		SpreadPerSecond: 0,
		MinQuality:      0,
	}
	mm := matchmaking.NewMatchmaker(cfg)
	mm.EnqueueAt(newPlayer("a", 1200), now)
	mm.EnqueueAt(newPlayer("b", 1300), now)
	mm.EnqueueAt(newPlayer("c", 1400), now)
	mm.EnqueueAt(newPlayer("d", 1500), now)

	matches := mm.FindMatches(now)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}

	m := matches[0]
	if len(m.Teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(m.Teams))
	}
	if len(m.Teams[0]) != 2 || len(m.Teams[1]) != 2 {
		t.Fatalf("expected 2 players per team")
	}

	avg := func(players []*rating.Player) float64 {
		sum := 0.0
		for _, p := range players {
			sum += p.MMR
		}
		return sum / float64(len(players))
	}

	avgT1 := avg(m.Teams[0])
	avgT2 := avg(m.Teams[1])

	if avgT1 != avgT2 {
		t.Errorf("snake draft should produce equal team averages: T1=%.1f T2=%.1f", avgT1, avgT2)
	}
}

// ---------------------------------------------------------------------------
// Quality score bounds
// ---------------------------------------------------------------------------

func TestFindMatches_QualityInBounds(t *testing.T) {
	mm := matchmaking.NewMatchmaker(cfg1v1())
	mm.EnqueueAt(newPlayer("a", 1500), now)
	mm.EnqueueAt(newPlayer("b", 1500), now)

	matches := mm.FindMatches(now)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match")
	}
	q := matches[0].Quality
	if q < 0 || q > 1 {
		t.Errorf("quality %.4f is outside [0, 1]", q)
	}
}

func TestFindMatches_EqualMMR_MaxQuality(t *testing.T) {
	mm := matchmaking.NewMatchmaker(cfg1v1())
	mm.EnqueueAt(newPlayer("a", 1500), now)
	mm.EnqueueAt(newPlayer("b", 1500), now)

	matches := mm.FindMatches(now)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match")
	}
	if matches[0].Quality != 1.0 {
		t.Errorf("equal MMR players should produce quality=1.0, got %.4f", matches[0].Quality)
	}
}

// ---------------------------------------------------------------------------
// Thread safety (smoke test)
// ---------------------------------------------------------------------------

func TestMatchmaker_Enqueue_Concurrent(t *testing.T) {
	mm := matchmaking.NewMatchmaker(cfg1v1())
	done := make(chan struct{})

	for i := 0; i < 50; i++ {
		go func(i int) {
			mm.Enqueue(newPlayer(string(rune('a'+i%26)), float64(1400+i*10)))
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 50; i++ {
		<-done
	}

	if mm.QueueSize() != 50 {
		t.Errorf("expected 50 players queued, got %d", mm.QueueSize())
	}
}
