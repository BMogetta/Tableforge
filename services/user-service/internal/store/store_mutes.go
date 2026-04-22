package store

import (
	"context"

	"github.com/google/uuid"
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

	out := make([]PlayerMute, 0)
	for rows.Next() {
		var m PlayerMute
		if err := rows.Scan(&m.MuterID, &m.MutedID, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

