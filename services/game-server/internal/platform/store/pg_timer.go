package store

import (
	"context"
	"fmt"
)

// ListSessionsNeedingTimer returns all active sessions that have a turn timeout
// configured. Used by TurnTimer.ReschedulePending on server startup to re-arm
// timers for sessions that were active when the server last shut down.
func (s *PGStore) ListSessionsNeedingTimer(ctx context.Context) ([]GameSession, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, room_id, game_id, name, state, move_count,
		        suspend_count, suspended_at, suspended_reason,
		        turn_timeout_secs, last_move_at, started_at, finished_at, deleted_at
		 FROM game_sessions
		 WHERE finished_at IS NULL
		   AND deleted_at IS NULL
		   AND suspended_at IS NULL
		   AND turn_timeout_secs IS NOT NULL
		   AND turn_timeout_secs > 0`,
	)
	if err != nil {
		return nil, fmt.Errorf("ListSessionsNeedingTimer: %w", err)
	}
	defer rows.Close()

	var sessions []GameSession
	for rows.Next() {
		gs, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("ListSessionsNeedingTimer scan: %w", err)
		}
		sessions = append(sessions, gs)
	}
	return sessions, rows.Err()
}
