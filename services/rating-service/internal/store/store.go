package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/recess/shared/domain/rating"
)

// ErrSessionAlreadyProcessed is returned by UpsertRatings when the
// rating_history row for (session_id, player_id) already exists. The caller
// must treat this as an idempotent success — re-delivery of the same
// game.session.finished event must not apply the delta twice.
var ErrSessionAlreadyProcessed = errors.New("rating history already recorded for this session")

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

// LeaderboardRow enriches a rating with player identity so the frontend
// can render names + bot indicators without a follow-up fetch.
type LeaderboardRow struct {
	PlayerRating
	Username   string
	IsBot      bool
	BotProfile *string
}

// HistoryEntry is a single row to insert into rating_history.
type HistoryEntry struct {
	PlayerID  uuid.UUID
	GameID    string
	SessionID uuid.UUID // game_sessions.id — part of the dedupe unique key
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
	// Only players with at least minGames games are included. When includeBots
	// is false, rows for bot accounts are filtered out so human rankings are
	// not distorted by the always-on backfill pool.
	GetLeaderboard(ctx context.Context, gameID string, limit, offset, minGames int, includeBots bool) ([]*LeaderboardRow, error)

	// CountLeaderboard returns the total number of players eligible for the
	// leaderboard. Mirrors GetLeaderboard's includeBots semantics.
	CountLeaderboard(ctx context.Context, gameID string, minGames int, includeBots bool) (int, error)

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
		FROM ratings.ratings
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
		FROM ratings.ratings
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

func (s *pgStore) GetLeaderboard(ctx context.Context, gameID string, limit, offset, minGames int, includeBots bool) ([]*LeaderboardRow, error) {
	// JOIN players so we can show username + bot indicators on the list
	// without a second round-trip. The WHERE clause filters bots out by
	// default; opting in is a $5 bool so the same prepared statement path
	// covers both modes.
	const q = `
		SELECT r.player_id, r.game_id, r.mmr, r.display_rating, r.games_played,
		       r.win_streak, r.loss_streak, r.updated_at,
		       p.username, p.is_bot, p.bot_profile
		FROM ratings.ratings r
		JOIN players p ON p.id = r.player_id
		WHERE r.game_id = $1
		  AND r.games_played >= $2
		  AND p.deleted_at IS NULL
		  AND ($5::bool OR p.is_bot = FALSE)
		ORDER BY r.display_rating DESC
		LIMIT $3 OFFSET $4`

	rows, err := s.db.Query(ctx, q, gameID, minGames, limit, offset, includeBots)
	if err != nil {
		return nil, fmt.Errorf("get leaderboard: %w", err)
	}
	defer rows.Close()

	var out []*LeaderboardRow
	for rows.Next() {
		row := &LeaderboardRow{}
		if err := rows.Scan(
			&row.PlayerID, &row.GameID, &row.MMR, &row.DisplayRating,
			&row.GamesPlayed, &row.WinStreak, &row.LossStreak, &row.UpdatedAt,
			&row.Username, &row.IsBot, &row.BotProfile,
		); err != nil {
			return nil, fmt.Errorf("scan leaderboard row: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *pgStore) CountLeaderboard(ctx context.Context, gameID string, minGames int, includeBots bool) (int, error) {
	var count int
	err := s.db.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM ratings.ratings r
		 JOIN players p ON p.id = r.player_id
		 WHERE r.game_id = $1
		   AND r.games_played >= $2
		   AND p.deleted_at IS NULL
		   AND ($3::bool OR p.is_bot = FALSE)`,
		gameID, minGames, includeBots,
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

	// History inserts run first so the UNIQUE(session_id, player_id) constraint
	// is the authoritative idempotency guard — a re-delivered event aborts here
	// before any rating delta is applied.
	const histQ = `
		INSERT INTO ratings.rating_history (player_id, game_id, session_id, result_id, mmr_before, mmr_after, delta)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`

	for _, h := range history {
		if _, err := tx.Exec(ctx, histQ,
			h.PlayerID, h.GameID, h.SessionID, h.ResultID, h.MMRBefore, h.MMRAfter, h.Delta,
		); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrSessionAlreadyProcessed
			}
			return fmt.Errorf("insert history %s: %w", h.PlayerID, err)
		}
	}

	const upsertQ = `
		INSERT INTO ratings.ratings (player_id, game_id, mmr, display_rating, games_played,
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

	return tx.Commit(ctx)
}
