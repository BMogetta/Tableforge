package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// MutePlayer records that muterID has muted mutedID.
// Idempotent: inserting a duplicate mute is a no-op.
func (s *PGStore) MutePlayer(ctx context.Context, muterID, mutedID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO player_mutes (muter_id, muted_id)
		 VALUES ($1, $2)
		 ON CONFLICT (muter_id, muted_id) DO NOTHING`,
		muterID, mutedID,
	)
	if err != nil {
		return fmt.Errorf("MutePlayer: %w", err)
	}
	return nil
}

// UnmutePlayer removes the mute record. No-op if the mute does not exist.
func (s *PGStore) UnmutePlayer(ctx context.Context, muterID, mutedID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM player_mutes WHERE muter_id = $1 AND muted_id = $2`,
		muterID, mutedID,
	)
	if err != nil {
		return fmt.Errorf("UnmutePlayer: %w", err)
	}
	return nil
}

// GetMutedPlayers returns all mute records where playerID is the muter.
func (s *PGStore) GetMutedPlayers(ctx context.Context, playerID uuid.UUID) ([]PlayerMute, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT muter_id, muted_id, created_at
		 FROM player_mutes
		 WHERE muter_id = $1
		 ORDER BY created_at DESC`,
		playerID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetMutedPlayers: %w", err)
	}
	defer rows.Close()

	mutes := []PlayerMute{}
	for rows.Next() {
		var m PlayerMute
		if err := rows.Scan(&m.MuterID, &m.MutedID, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("GetMutedPlayers scan: %w", err)
		}
		mutes = append(mutes, m)
	}
	return mutes, rows.Err()
}
