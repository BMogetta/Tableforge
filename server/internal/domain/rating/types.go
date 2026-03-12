// Package rating implements an Elo-family rating system with hidden MMR,
// dynamic K-factor, and support for arbitrary team configurations (1v1
// through 5v5, multi-team free-for-all such as 2v2v2v2, etc.).
//
// # Architecture overview
//
// Each player carries two rating values:
//
//   - MMR (Match-Making Rating): the hidden, true skill estimate used for
//     matchmaking. This value is never exposed to end-users.
//   - DisplayRating: the public-facing rating shown on leaderboards.
//     DisplayRating converges toward MMR over many games but may lag behind.
//
// Match results are processed pairwise between every pair of opposing teams
// using a generalised multi-team Elo algorithm (see engine.go for the full
// derivation).
package rating

import "fmt"

// ---------------------------------------------------------------------------
// Player
// ---------------------------------------------------------------------------

// Player represents a single human player in the rating system.
type Player struct {
	ID            string  // Unique identifier.
	MMR           float64 // Hidden skill estimate (default 1500).
	DisplayRating float64 // Public leaderboard rating (default 1500).
	GamesPlayed   int     // Lifetime game count — drives dynamic K.
	WinStreak     int     // Current consecutive-win streak (resets on loss).
	LossStreak    int     // Current consecutive-loss streak (resets on win).
}

// NewPlayer creates a player with default ratings.
func NewPlayer(id string) *Player {
	return &Player{
		ID:            id,
		MMR:           DefaultMMR,
		DisplayRating: DefaultMMR,
	}
}

func (p *Player) String() string {
	return fmt.Sprintf("Player{%s mmr=%.1f display=%.1f games=%d}",
		p.ID, p.MMR, p.DisplayRating, p.GamesPlayed)
}

// ---------------------------------------------------------------------------
// Team
// ---------------------------------------------------------------------------

// Team is an ordered group of players competing as a single unit in a match.
// The team's effective MMR is the arithmetic mean of its members' MMRs.
type Team struct {
	Players []*Player
}

// EffectiveMMR returns the arithmetic mean MMR of every player on the team.
//
//	TeamMMR = (1/N) * Σ player_i.MMR    for i in 1..N
func (t *Team) EffectiveMMR() float64 {
	if len(t.Players) == 0 {
		return 0
	}
	sum := 0.0
	for _, p := range t.Players {
		sum += p.MMR
	}
	return sum / float64(len(t.Players))
}

// EffectiveDisplayRating mirrors EffectiveMMR but uses the public rating.
func (t *Team) EffectiveDisplayRating() float64 {
	if len(t.Players) == 0 {
		return 0
	}
	sum := 0.0
	for _, p := range t.Players {
		sum += p.DisplayRating
	}
	return sum / float64(len(t.Players))
}

// AverageGamesPlayed returns the floor-average of games played across all
// team members. Used to decide whether the team is still in calibration.
func (t *Team) AverageGamesPlayed() int {
	if len(t.Players) == 0 {
		return 0
	}
	sum := 0
	for _, p := range t.Players {
		sum += p.GamesPlayed
	}
	return sum / len(t.Players)
}

// ---------------------------------------------------------------------------
// MatchResult
// ---------------------------------------------------------------------------

// Placement describes where a team finished. Lower rank is better (1 = winner).
// Two teams may share the same rank to represent a draw.
type Placement struct {
	Team *Team
	Rank int // 1-based finish position. 1 = first place.
}

// MatchResult holds the full outcome of one match.
type MatchResult struct {
	Placements []Placement
}

// Validate performs basic sanity checks on a MatchResult.
func (mr *MatchResult) Validate() error {
	if len(mr.Placements) < 2 {
		return fmt.Errorf("match must have at least 2 teams, got %d", len(mr.Placements))
	}
	for i, p := range mr.Placements {
		if p.Rank < 1 {
			return fmt.Errorf("placement[%d].Rank must be >= 1, got %d", i, p.Rank)
		}
		if len(p.Team.Players) == 0 {
			return fmt.Errorf("placement[%d] has an empty team", i)
		}
	}
	return nil
}
