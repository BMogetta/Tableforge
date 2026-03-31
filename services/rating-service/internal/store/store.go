package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tableforge/shared/domain/rating"
)

// --- Models ------------------------------------------------------------------

// PlayerRating maps 1:1 to the ratings table row.
type PlayerRating struct {
	PlayerID      uuid.UUID
	GameID        string
	MMR           float64
	DisplayRating float64
	GamesPlayed   int
	WinStreak     int
	LossStreak    int
	UpdatedAt     time.Time
}

// HistoryEntry is a single row to insert into rating_history.
type HistoryEntry struct {
	PlayerID  uuid.UUID
	GameID    string
	ResultID  uuid.UUID // game_results.id
	MMRBefore float64
	MMRAfter  float64
	Delta     float64
}

// --- Interface ---------------------------------------------------------------

type Store interface {
	// GetRating returns the current rating for (playerID, gameID).
	// If no row exists, returns a default-initialised PlayerRating (MMR=1500).
	GetRating(ctx context.Context, playerID uuid.UUID, gameID string) (*PlayerRating, error)

	// GetRatings returns ratings for a batch of playerIDs under the same game.
	// Missing rows are returned with default values — never an error.
	GetRatings(ctx context.Context, playerIDs []uuid.UUID, gameID string) (map[uuid.UUID]*PlayerRating, error)

	// GetLeaderboard returns up to limit rows ordered by display_rating DESC.
	// Only players with at least minGames games are included.
	GetLeaderboard(ctx context.Context, gameID string, limit, offset, minGames int) ([]*PlayerRating, error)

	// CountLeaderboard returns the total number of players eligible for the leaderboard.
	CountLeaderboard(ctx context.Context, gameID string, minGames int) (int, error)

	// UpsertRatings writes updated ratings and inserts history rows in one transaction.
	UpsertRatings(ctx context.Context, updates []*PlayerRating, history []HistoryEntry) error
}

// --- Postgres implementation -------------------------------------------------

type pgStore struct {
	db *pgxpool.Pool
}

func New(ctx context.Context, connStr string) (Store, error) {
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &pgStore{db: pool}, nil
}

func (s *pgStore) Close() {
	s.db.Close()
}

func (s *pgStore) GetRating(ctx context.Context, playerID uuid.UUID, gameID string) (*PlayerRating, error) {
	const q = `
		SELECT player_id, game_id, mmr, display_rating, games_played,
		       win_streak, loss_streak, updated_at
		FROM ratings
		WHERE player_id = $1 AND game_id = $2`

	pr := &PlayerRating{}
	err := s.db.QueryRow(ctx, q, playerID, gameID).Scan(
		&pr.PlayerID, &pr.GameID, &pr.MMR, &pr.DisplayRating,
		&pr.GamesPlayed, &pr.WinStreak, &pr.LossStreak, &pr.UpdatedAt,
	)
	if err != nil {
		// pgx returns pgx.ErrNoRows — return defaults, not an error.
		return &PlayerRating{
			PlayerID:      playerID,
			GameID:        gameID,
			MMR:           rating.DefaultMMR,
			DisplayRating: rating.DefaultMMR,
		}, nil
	}
	return pr, nil
}

func (s *pgStore) GetRatings(ctx context.Context, playerIDs []uuid.UUID, gameID string) (map[uuid.UUID]*PlayerRating, error) {
	result := make(map[uuid.UUID]*PlayerRating, len(playerIDs))

	// Pre-populate with defaults so every requested ID is present in the map.
	for _, id := range playerIDs {
		result[id] = &PlayerRating{
			PlayerID:      id,
			GameID:        gameID,
			MMR:           rating.DefaultMMR,
			DisplayRating: rating.DefaultMMR,
		}
	}

	if len(playerIDs) == 0 {
		return result, nil
	}

	const q = `
		SELECT player_id, game_id, mmr, display_rating, games_played,
		       win_streak, loss_streak, updated_at
		FROM ratings
		WHERE player_id = ANY($1) AND game_id = $2`

	rows, err := s.db.Query(ctx, q, playerIDs, gameID)
	if err != nil {
		return nil, fmt.Errorf("get ratings batch: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		pr := &PlayerRating{}
		if err := rows.Scan(
			&pr.PlayerID, &pr.GameID, &pr.MMR, &pr.DisplayRating,
			&pr.GamesPlayed, &pr.WinStreak, &pr.LossStreak, &pr.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan rating row: %w", err)
		}
		result[pr.PlayerID] = pr // overwrite default with real row
	}
	return result, rows.Err()
}

func (s *pgStore) GetLeaderboard(ctx context.Context, gameID string, limit, offset, minGames int) ([]*PlayerRating, error) {
	const q = `
		SELECT player_id, game_id, mmr, display_rating, games_played,
		       win_streak, loss_streak, updated_at
		FROM ratings
		WHERE game_id = $1
		  AND games_played >= $2
		ORDER BY display_rating DESC
		LIMIT $3 OFFSET $4`

	rows, err := s.db.Query(ctx, q, gameID, minGames, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get leaderboard: %w", err)
	}
	defer rows.Close()

	var out []*PlayerRating
	for rows.Next() {
		pr := &PlayerRating{}
		if err := rows.Scan(
			&pr.PlayerID, &pr.GameID, &pr.MMR, &pr.DisplayRating,
			&pr.GamesPlayed, &pr.WinStreak, &pr.LossStreak, &pr.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan leaderboard row: %w", err)
		}
		out = append(out, pr)
	}
	return out, rows.Err()
}

func (s *pgStore) CountLeaderboard(ctx context.Context, gameID string, minGames int) (int, error) {
	var count int
	err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM ratings WHERE game_id = $1 AND games_played >= $2`,
		gameID, minGames,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count leaderboard: %w", err)
	}
	return count, nil
}

func (s *pgStore) UpsertRatings(ctx context.Context, updates []*PlayerRating, history []HistoryEntry) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const upsertQ = `
		INSERT INTO ratings (player_id, game_id, mmr, display_rating, games_played,
		                     win_streak, loss_streak, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())
		ON CONFLICT (player_id, game_id) DO UPDATE SET
			mmr            = EXCLUDED.mmr,
			display_rating = EXCLUDED.display_rating,
			games_played   = EXCLUDED.games_played,
			win_streak     = EXCLUDED.win_streak,
			loss_streak    = EXCLUDED.loss_streak,
			updated_at     = NOW()`

	for _, u := range updates {
		if _, err := tx.Exec(ctx, upsertQ,
			u.PlayerID, u.GameID, u.MMR, u.DisplayRating,
			u.GamesPlayed, u.WinStreak, u.LossStreak,
		); err != nil {
			return fmt.Errorf("upsert rating %s: %w", u.PlayerID, err)
		}
	}

	const histQ = `
		INSERT INTO rating_history (player_id, game_id, result_id, mmr_before, mmr_after, delta)
		VALUES ($1,$2,$3,$4,$5,$6)`

	for _, h := range history {
		if _, err := tx.Exec(ctx, histQ,
			h.PlayerID, h.GameID, h.ResultID, h.MMRBefore, h.MMRAfter, h.Delta,
		); err != nil {
			return fmt.Errorf("insert history %s: %w", h.PlayerID, err)
		}
	}

	return tx.Commit(ctx)
}
