// Package matchmaking provides a queue-based matchmaker that groups players
// into balanced matches based on their hidden MMR.
//
// # Design
//
// The matchmaker operates on a FIFO queue of players. When asked to form
// matches it:
//
//  1. Sorts the queue by MMR.
//  2. Uses a sliding window of size PlayersPerTeam * TeamsPerMatch.
//  3. Within each window, assigns players to teams via snake draft
//     (zigzag ordering) so each team gets a balanced mix of the strongest
//     and weakest candidates in the window.
//  4. Evaluates match quality; only emits matches at or above MinQuality.
//
// # Quality metric
//
//	Quality = 1 - (maxTeamMMR - minTeamMMR) / MaxAcceptableSpread
//
// MaxAcceptableSpread widens linearly with the longest individual wait time
// in the window:
//
//	MaxAcceptableSpread = BaseSpread + WaitSeconds * SpreadPerSecond
//
// This guarantees tight matches initially while ensuring every player
// eventually finds a game.
//
// # Snake draft example (3v3, sorted MMRs [1200..1700])
//
//	Pass 1 (→): T1=1200, T2=1300
//	Pass 2 (←): T2=1400, T1=1500
//	Pass 3 (→): T1=1600, T2=1700
//	T1 avg = (1200+1500+1600)/3 = 1433
//	T2 avg = (1300+1400+1700)/3 = 1467   → difference = 34 MMR
package matchmaking

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/tableforge/shared/domain/rating"
)

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

// QueueConfig controls matchmaking behaviour.
type QueueConfig struct {
	// PlayersPerTeam is the team size (e.g. 1 for 1v1, 5 for 5v5).
	PlayersPerTeam int

	// TeamsPerMatch is the number of opposing teams (2 for standard,
	// 4 for a 2v2v2v2, etc.).
	TeamsPerMatch int

	// BaseSpread is the initial acceptable MMR spread between the
	// strongest and weakest team in a proposed match.
	BaseSpread float64

	// SpreadPerSecond widens the acceptable spread by this amount for each
	// second the longest-waiting player in the window has been queued.
	// Default 2.0 means after 60 s the spread relaxes by 120 MMR.
	SpreadPerSecond float64

	// MinQuality is the minimum quality score [0, 1] a match must reach
	// before it is emitted. Set to 0 to accept any match.
	MinQuality float64
}

// DefaultQueueConfig returns sensible defaults for competitive 1v1.
// Callers building 5v5 or multi-team modes should adjust PlayersPerTeam
// and TeamsPerMatch accordingly.
func DefaultQueueConfig() QueueConfig {
	return QueueConfig{
		PlayersPerTeam:  1,
		TeamsPerMatch:   2,
		BaseSpread:      200,
		SpreadPerSecond: 2.0,
		MinQuality:      0.4,
	}
}

// ---------------------------------------------------------------------------
// Queue entry
// ---------------------------------------------------------------------------

// QueueEntry represents a single player waiting for a match.
type QueueEntry struct {
	Player   *rating.Player
	JoinedAt time.Time
}

// ---------------------------------------------------------------------------
// ProposedMatch
// ---------------------------------------------------------------------------

// ProposedMatch groups players into teams, ready to start a game.
type ProposedMatch struct {
	Teams   [][]*rating.Player // Teams[i] = players assigned to team i.
	Quality float64            // Quality score in [0, 1].
}

func (pm *ProposedMatch) String() string {
	s := fmt.Sprintf("Match(quality=%.2f) ", pm.Quality)
	for i, t := range pm.Teams {
		s += fmt.Sprintf("Team%d[", i+1)
		for j, p := range t {
			if j > 0 {
				s += ","
			}
			s += fmt.Sprintf("%s(%.0f)", p.ID, p.MMR)
		}
		s += "] "
	}
	return s
}

// ---------------------------------------------------------------------------
// Matchmaker
// ---------------------------------------------------------------------------

// Matchmaker is a thread-safe, queue-based matchmaker.
type Matchmaker struct {
	mu    sync.Mutex
	queue []QueueEntry
	cfg   QueueConfig
}

// NewMatchmaker creates a Matchmaker with the given config.
func NewMatchmaker(cfg QueueConfig) *Matchmaker {
	return &Matchmaker{cfg: cfg}
}

// Enqueue adds a player to the matchmaking queue with the current time.
func (m *Matchmaker) Enqueue(p *rating.Player) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queue = append(m.queue, QueueEntry{Player: p, JoinedAt: time.Now()})
}

// EnqueueAt adds a player with an explicit join time. Useful for testing
// wait-time-dependent spread relaxation without sleeping.
func (m *Matchmaker) EnqueueAt(p *rating.Player, joinedAt time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queue = append(m.queue, QueueEntry{Player: p, JoinedAt: joinedAt})
}

// QueueSize returns the number of players currently waiting.
func (m *Matchmaker) QueueSize() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.queue)
}

// FindMatches attempts to form as many balanced matches as possible from the
// current queue. Players that are matched are removed from the queue.
// now is used to compute per-player wait times; pass time.Now() in production.
func (m *Matchmaker) FindMatches(now time.Time) []ProposedMatch {
	m.mu.Lock()
	defer m.mu.Unlock()

	totalPerMatch := m.cfg.PlayersPerTeam * m.cfg.TeamsPerMatch
	if len(m.queue) < totalPerMatch {
		return nil
	}

	// Sort queue by MMR ascending so adjacent windows are skill-balanced.
	sort.Slice(m.queue, func(i, j int) bool {
		return m.queue[i].Player.MMR < m.queue[j].Player.MMR
	})

	var matches []ProposedMatch
	used := make([]bool, len(m.queue))

	// Sliding window: try to form one match per contiguous block.
	for start := 0; start+totalPerMatch <= len(m.queue); start++ {
		// Skip windows that contain an already-matched player.
		windowOK := true
		for k := start; k < start+totalPerMatch; k++ {
			if used[k] {
				windowOK = false
				break
			}
		}
		if !windowOK {
			continue
		}

		entries := m.queue[start : start+totalPerMatch]

		// MaxAcceptableSpread is driven by the longest waiter in the window.
		maxSpread := 0.0
		for _, e := range entries {
			waitSec := now.Sub(e.JoinedAt).Seconds()
			spread := m.cfg.BaseSpread + m.cfg.SpreadPerSecond*waitSec
			if spread > maxSpread {
				maxSpread = spread
			}
		}

		// Assign players to teams using a snake draft for balance.
		// Players are already MMR-sorted; zigzag assignment ensures each
		// team receives a mix of high and low rated players in the window.
		teams := make([][]*rating.Player, m.cfg.TeamsPerMatch)
		for i := range teams {
			teams[i] = make([]*rating.Player, 0, m.cfg.PlayersPerTeam)
		}

		sorted := make([]*rating.Player, totalPerMatch)
		for i, e := range entries {
			sorted[i] = e.Player
		}

		idx := 0
		forward := true
		for _, p := range sorted {
			teams[idx] = append(teams[idx], p)
			if forward {
				idx++
				if idx >= m.cfg.TeamsPerMatch {
					idx = m.cfg.TeamsPerMatch - 1
					forward = false
				}
			} else {
				idx--
				if idx < 0 {
					idx = 0
					forward = true
				}
			}
		}

		quality := matchQuality(teams, maxSpread)
		if quality < m.cfg.MinQuality {
			continue
		}

		// Accept the match and mark all players in the window as used.
		for k := start; k < start+totalPerMatch; k++ {
			used[k] = true
		}
		matches = append(matches, ProposedMatch{Teams: teams, Quality: quality})
	}

	// Remove matched players from the queue.
	newQueue := m.queue[:0]
	for i, e := range m.queue {
		if !used[i] {
			newQueue = append(newQueue, e)
		}
	}
	m.queue = newQueue

	return matches
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// matchQuality returns a quality score in [0, 1].
//
//	Quality = 1 - (maxTeamMMR - minTeamMMR) / maxSpread
func matchQuality(teams [][]*rating.Player, maxSpread float64) float64 {
	if maxSpread <= 0 {
		return 0
	}
	minMMR := avgMMR(teams[0])
	maxMMR := minMMR
	for _, t := range teams[1:] {
		mmr := avgMMR(t)
		if mmr < minMMR {
			minMMR = mmr
		}
		if mmr > maxMMR {
			maxMMR = mmr
		}
	}
	q := 1.0 - (maxMMR-minMMR)/maxSpread
	if q < 0 {
		return 0
	}
	if q > 1 {
		return 1
	}
	return q
}

// avgMMR returns the mean MMR of a group of players.
func avgMMR(players []*rating.Player) float64 {
	if len(players) == 0 {
		return 0
	}
	sum := 0.0
	for _, p := range players {
		sum += p.MMR
	}
	return sum / float64(len(players))
}
