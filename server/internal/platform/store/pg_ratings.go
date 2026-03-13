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

// GetRatingLeaderboard returns the top N players for a game ordered by
// display_rating descending. Player username and avatar are joined in.
// MMR is intentionally excluded from the result — it must not reach the frontend.
func (s *PGStore) GetRatingLeaderboard(ctx context.Context, gameID string, limit int) ([]RatingLeaderboardEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT r.player_id, r.game_id, p.username, COALESCE(p.avatar_url, ''),
		        r.display_rating, r.games_played, r.win_streak, r.loss_streak, r.updated_at
		 FROM ratings r
		 JOIN players p ON p.id = r.player_id
		 WHERE r.game_id = $1
		   AND p.deleted_at IS NULL
		 ORDER BY r.display_rating DESC
		 LIMIT $2`,
		gameID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("GetRatingLeaderboard: %w", err)
	}
	defer rows.Close()

	entries := []RatingLeaderboardEntry{}
	for rows.Next() {
		e, err := scanLeaderboardEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
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

func scanLeaderboardEntry(row scanner) (RatingLeaderboardEntry, error) {
	var e RatingLeaderboardEntry
	if err := row.Scan(
		&e.PlayerID, &e.GameID, &e.Username, &e.AvatarURL,
		&e.DisplayRating, &e.GamesPlayed, &e.WinStreak, &e.LossStreak, &e.UpdatedAt,
	); err != nil {
		return RatingLeaderboardEntry{}, fmt.Errorf("scanLeaderboardEntry: %w", err)
	}
	return e, nil
}
