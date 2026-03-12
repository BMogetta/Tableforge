package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// GetRating returns the rating record for a player.
// Returns an error if the player has no rating row yet.
func (s *PGStore) GetRating(ctx context.Context, playerID uuid.UUID) (Rating, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT player_id, mmr, display_rating, games_played, win_streak, loss_streak, updated_at
		 FROM ratings WHERE player_id = $1`,
		playerID,
	)
	return scanRating(row)
}

// UpsertRating inserts or updates the rating record for a player.
// All fields are replaced on conflict.
func (s *PGStore) UpsertRating(ctx context.Context, r Rating) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO ratings (player_id, mmr, display_rating, games_played, win_streak, loss_streak, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW())
		 ON CONFLICT (player_id) DO UPDATE
		   SET mmr            = EXCLUDED.mmr,
		       display_rating = EXCLUDED.display_rating,
		       games_played   = EXCLUDED.games_played,
		       win_streak     = EXCLUDED.win_streak,
		       loss_streak    = EXCLUDED.loss_streak,
		       updated_at     = NOW()`,
		r.PlayerID, r.MMR, r.DisplayRating, r.GamesPlayed, r.WinStreak, r.LossStreak,
	)
	if err != nil {
		return fmt.Errorf("UpsertRating: %w", err)
	}
	return nil
}

// GetRatingLeaderboard returns the top N players ordered by MMR descending.
func (s *PGStore) GetRatingLeaderboard(ctx context.Context, limit int) ([]Rating, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT player_id, mmr, display_rating, games_played, win_streak, loss_streak, updated_at
		 FROM ratings
		 ORDER BY mmr DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("GetRatingLeaderboard: %w", err)
	}
	defer rows.Close()

	entries := []Rating{}
	for rows.Next() {
		r, err := scanRating(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, r)
	}
	return entries, rows.Err()
}

func scanRating(row scanner) (Rating, error) {
	var r Rating
	if err := row.Scan(
		&r.PlayerID, &r.MMR, &r.DisplayRating,
		&r.GamesPlayed, &r.WinStreak, &r.LossStreak, &r.UpdatedAt,
	); err != nil {
		return Rating{}, fmt.Errorf("scanRating: %w", err)
	}
	return r, nil
}
