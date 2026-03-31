package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// GetRating returns the rating record for a player scoped to a specific game.
// Returns an error if the player has no rating row for that game yet.
func (s *PGStore) GetRating(ctx context.Context, playerID uuid.UUID, gameID string) (Rating, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT player_id, game_id, mmr, display_rating, games_played, win_streak, loss_streak, updated_at
		 FROM ratings WHERE player_id = $1 AND game_id = $2`,
		playerID, gameID,
	)
	return scanRating(row)
}

// UpsertRating inserts or updates the rating record for a player and game.
// All fields are replaced on conflict.
func (s *PGStore) UpsertRating(ctx context.Context, r Rating) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO ratings (player_id, game_id, mmr, display_rating, games_played, win_streak, loss_streak, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		 ON CONFLICT (player_id, game_id) DO UPDATE
		   SET mmr            = EXCLUDED.mmr,
		       display_rating = EXCLUDED.display_rating,
		       games_played   = EXCLUDED.games_played,
		       win_streak     = EXCLUDED.win_streak,
		       loss_streak    = EXCLUDED.loss_streak,
		       updated_at     = NOW()`,
		r.PlayerID, r.GameID, r.MMR, r.DisplayRating, r.GamesPlayed, r.WinStreak, r.LossStreak,
	)
	if err != nil {
		return fmt.Errorf("UpsertRating: %w", err)
	}
	return nil
}

func scanRating(row scanner) (Rating, error) {
	var r Rating
	if err := row.Scan(
		&r.PlayerID, &r.GameID, &r.MMR, &r.DisplayRating,
		&r.GamesPlayed, &r.WinStreak, &r.LossStreak, &r.UpdatedAt,
	); err != nil {
		return Rating{}, fmt.Errorf("scanRating: %w", err)
	}
	return r, nil
}

