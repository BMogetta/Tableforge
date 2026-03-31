package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *pgStore) IssueBan(ctx context.Context, params IssueBanParams) (Ban, error) {
	row := s.db.QueryRow(ctx, `
		INSERT INTO users.bans (player_id, banned_by, reason, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, player_id, banned_by, reason, expires_at, lifted_at, lifted_by, created_at
	`, params.PlayerID, params.BannedBy, params.Reason, params.ExpiresAt)

	return scanBan(row)
}

func (s *pgStore) LiftBan(ctx context.Context, banID, liftedBy uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE users.bans
		SET lifted_at = NOW(), lifted_by = $2
		WHERE id = $1 AND lifted_at IS NULL
	`, banID, liftedBy)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CheckActiveBan returns the active ban for a player, or nil if not banned.
// A ban is active when lifted_at IS NULL AND (expires_at IS NULL OR expires_at > NOW()).
func (s *pgStore) CheckActiveBan(ctx context.Context, playerID uuid.UUID) (*Ban, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, player_id, banned_by, reason, expires_at, lifted_at, lifted_by, created_at
		FROM users.bans
		WHERE player_id = $1
		  AND lifted_at IS NULL
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at DESC
		LIMIT 1
	`, playerID)

	ban, err := scanBan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ban, nil
}

func (s *pgStore) ListBans(ctx context.Context, playerID uuid.UUID) ([]Ban, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, player_id, banned_by, reason, expires_at, lifted_at, lifted_by, created_at
		FROM users.bans
		WHERE player_id = $1
		ORDER BY created_at DESC
	`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Ban
	for rows.Next() {
		var b Ban
		if err := rows.Scan(&b.ID, &b.PlayerID, &b.BannedBy, &b.Reason,
			&b.ExpiresAt, &b.LiftedAt, &b.LiftedBy, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// --- helpers -----------------------------------------------------------------

func scanBan(row pgx.Row) (Ban, error) {
	var b Ban
	err := row.Scan(&b.ID, &b.PlayerID, &b.BannedBy, &b.Reason,
		&b.ExpiresAt, &b.LiftedAt, &b.LiftedBy, &b.CreatedAt)
	return b, err
}
