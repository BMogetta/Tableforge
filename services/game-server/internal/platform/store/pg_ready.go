package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// VoteReady appends playerID to the ready_players array for the session.
// Returns allVoted=true when every participant in the room has confirmed.
// Idempotent — safe to call multiple times for the same player.
func (s *PGStore) VoteReady(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID) (bool, error) {
	playerStr := playerID.String()

	_, err := s.pool.Exec(ctx,
		`UPDATE game_sessions
		 SET ready_players = array_append(ready_players, $1::TEXT)
		 WHERE id = $2
		   AND NOT ($1::TEXT = ANY(ready_players))`,
		playerStr, sessionID,
	)
	if err != nil {
		return false, fmt.Errorf("VoteReady: %w", err)
	}

	// Compare ready_players array length against human player count.
	var allReady bool
	err = s.pool.QueryRow(ctx,
		`SELECT
		     COALESCE(array_length(gs.ready_players, 1), 0)
		     >=
		     (SELECT COUNT(*)
		      FROM room_players rp
		      JOIN players p ON p.id = rp.player_id
		      WHERE rp.room_id = gs.room_id AND p.is_bot = FALSE)
		 FROM game_sessions gs
		 WHERE gs.id = $1`,
		sessionID,
	).Scan(&allReady)
	if err != nil {
		return false, fmt.Errorf("VoteReady: check all ready: %w", err)
	}
	return allReady, nil
}

// ClearReadyVotes resets the ready_players array to empty for the session.
func (s *PGStore) ClearReadyVotes(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE game_sessions SET ready_players = '{}' WHERE id = $1`,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("ClearReadyVotes: %w", err)
	}
	return nil
}
