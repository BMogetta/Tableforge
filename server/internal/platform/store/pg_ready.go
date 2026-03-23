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

	return s.allPlayersVoted(ctx, sessionID, "ready_players")
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
