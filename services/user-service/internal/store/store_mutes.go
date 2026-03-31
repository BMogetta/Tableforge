package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *pgStore) MutePlayer(ctx context.Context, muterID, mutedID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO users.player_mutes (muter_id, muted_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, muterID, mutedID)
	return err
}

func (s *pgStore) UnmutePlayer(ctx context.Context, muterID, mutedID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `
		DELETE FROM users.player_mutes
		WHERE muter_id = $1 AND muted_id = $2
	`, muterID, mutedID)
	return err
}

func (s *pgStore) GetMutedPlayers(ctx context.Context, playerID uuid.UUID) ([]PlayerMute, error) {
	rows, err := s.db.Query(ctx, `
		SELECT muter_id, muted_id, created_at
		FROM users.player_mutes
		WHERE muter_id = $1
		ORDER BY created_at DESC
	`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PlayerMute
	for rows.Next() {
		var m PlayerMute
		if err := rows.Scan(&m.MuterID, &m.MutedID, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// scanMutes is used internally by the gRPC server to get just the muted IDs.
func (s *pgStore) getMutedIDs(ctx context.Context, playerID uuid.UUID) ([]string, error) {
	rows, err := s.db.Query(ctx, `
		SELECT muted_id FROM users.player_mutes WHERE muter_id = $1
	`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id.String())
	}
	return ids, rows.Err()
}

// scanRows helper for single-column UUID scans.
func scanUUIDRows(rows pgx.Rows) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
