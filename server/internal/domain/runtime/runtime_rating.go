package runtime

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/domain/rating"
	"github.com/tableforge/server/internal/platform/store"
)

// applyRatings updates Elo ratings for all players after a ranked session ends.
// winnerID is nil on a draw. players is the full list of room participants.
// Errors are logged but never returned — a rating failure must not roll back
// a completed match.
func (svc *Service) applyRatings(
	ctx context.Context,
	session store.GameSession,
	players []store.RoomPlayer,
	winnerID *uuid.UUID,
	isDraw bool,
) {
	if svc.ratingEngine == nil {
		return
	}

	// Build rating.Player slice, fetching current ratings from the store.
	// Fall back to default values for players with no rating row yet.
	ratingPlayers := make([]*rating.Player, len(players))
	for i, rp := range players {
		r, err := svc.store.GetRating(ctx, rp.PlayerID, session.GameID)
		if err != nil {
			// No row yet — start from defaults.
			ratingPlayers[i] = rating.NewPlayer(rp.PlayerID.String())
		} else {
			ratingPlayers[i] = &rating.Player{
				ID:            rp.PlayerID.String(),
				MMR:           r.MMR,
				DisplayRating: r.DisplayRating,
				GamesPlayed:   r.GamesPlayed,
				WinStreak:     r.WinStreak,
				LossStreak:    r.LossStreak,
			}
		}
	}

	// Build placements. For 1v1: winner gets rank 1, loser rank 2, draw = both rank 1.
	// For N>2 this would need seat-order or explicit placement data — currently
	// all non-winner players share rank 2, which is correct for 1v1.
	placements := make([]rating.Placement, len(players))
	for i, rp := range players {
		rank := 2
		if isDraw {
			rank = 1
		} else if winnerID != nil && rp.PlayerID == *winnerID {
			rank = 1
		}
		placements[i] = rating.Placement{
			Team: &rating.Team{Players: []*rating.Player{ratingPlayers[i]}},
			Rank: rank,
		}
	}

	result := &rating.MatchResult{Placements: placements}
	if _, err := svc.ratingEngine.ProcessMatch(result); err != nil {
		log.Printf("applyRatings: ProcessMatch session=%s: %v", session.ID, err)
		return
	}

	// Persist updated ratings.
	for i, rp := range players {
		p := ratingPlayers[i]
		playerID := rp.PlayerID
		if err := svc.store.UpsertRating(ctx, store.Rating{
			PlayerID:      playerID,
			GameID:        session.GameID,
			MMR:           p.MMR,
			DisplayRating: p.DisplayRating,
			GamesPlayed:   p.GamesPlayed,
			WinStreak:     p.WinStreak,
			LossStreak:    p.LossStreak,
		}); err != nil {
			log.Printf("applyRatings: UpsertRating player=%s session=%s: %v",
				playerID, session.ID, err)
		}
	}

	log.Printf("applyRatings: ratings updated for session=%s mode=ranked players=%d",
		session.ID, len(players))
}

// buildRatingResult is kept separate for testability — it constructs the
// rating.MatchResult without any store interaction.
func buildRatingResult(ratingPlayers []*rating.Player, players []store.RoomPlayer, winnerID *uuid.UUID, isDraw bool) (*rating.MatchResult, error) {
	if len(ratingPlayers) != len(players) {
		return nil, fmt.Errorf("buildRatingResult: player slice length mismatch")
	}
	placements := make([]rating.Placement, len(players))
	for i, rp := range players {
		rank := 2
		if isDraw {
			rank = 1
		} else if winnerID != nil && rp.PlayerID == *winnerID {
			rank = 1
		}
		placements[i] = rating.Placement{
			Team: &rating.Team{Players: []*rating.Player{ratingPlayers[i]}},
			Rank: rank,
		}
	}
	return &rating.MatchResult{Placements: placements}, nil
}
