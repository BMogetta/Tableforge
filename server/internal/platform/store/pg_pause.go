package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// VotePause appends playerID to the pause_votes array for the session.
// Returns allVoted=true when every participant in the room has voted to pause.
// The caller (runtime) is responsible for calling SuspendSession and ClearPauseVotes
// when allVoted is true.
func (s *PGStore) VotePause(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID) (bool, error) {
	playerStr := playerID.String()

	// Append the vote (idempotent: array_append only if not already present).
	_, err := s.pool.Exec(ctx,
		`UPDATE game_sessions
		 SET pause_votes = array_append(pause_votes, $1::TEXT)
		 WHERE id = $2
		   AND NOT ($1::TEXT = ANY(pause_votes))`,
		playerStr, sessionID,
	)
	if err != nil {
		return false, fmt.Errorf("VotePause: %w", err)
	}

	return s.allPlayersVoted(ctx, sessionID, "pause_votes")
}

// ClearPauseVotes resets the pause_votes array to empty for the session.
func (s *PGStore) ClearPauseVotes(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE game_sessions SET pause_votes = '{}' WHERE id = $1`,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("ClearPauseVotes: %w", err)
	}
	return nil
}

// VoteResume appends playerID to the resume_votes array for the session.
// Returns allVoted=true when every participant in the room has voted to resume.
// The caller (runtime) is responsible for calling ResumeSession and ClearResumeVotes
// when allVoted is true.
func (s *PGStore) VoteResume(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID) (bool, error) {
	playerStr := playerID.String()

	_, err := s.pool.Exec(ctx,
		`UPDATE game_sessions
		 SET resume_votes = array_append(resume_votes, $1::TEXT)
		 WHERE id = $2
		   AND NOT ($1::TEXT = ANY(resume_votes))`,
		playerStr, sessionID,
	)
	if err != nil {
		return false, fmt.Errorf("VoteResume: %w", err)
	}

	return s.allPlayersVoted(ctx, sessionID, "resume_votes")
}

// ClearResumeVotes resets the resume_votes array to empty for the session.
func (s *PGStore) ClearResumeVotes(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE game_sessions SET resume_votes = '{}' WHERE id = $1`,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("ClearResumeVotes: %w", err)
	}
	return nil
}

// ForceCloseSession immediately finishes a session without a result.
// Manager-only action for suspended sessions.
func (s *PGStore) ForceCloseSession(ctx context.Context, sessionID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE game_sessions
		 SET finished_at = NOW(), suspended_at = NULL, suspended_reason = NULL
		 WHERE id = $1 AND finished_at IS NULL`,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("ForceCloseSession: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("ForceCloseSession: session not found or already finished")
	}
	return nil
}

// allPlayersVoted returns true when the length of the named vote column equals
// the number of participants in the session's room.
func (s *PGStore) allPlayersVoted(ctx context.Context, sessionID uuid.UUID, voteColumn string) (bool, error) {
	// NOTE: voteColumn is a trusted internal value (only "pause_votes" or
	// "resume_votes") — never user-supplied. The format verb is intentional.
	query := fmt.Sprintf(
		`SELECT
		     (SELECT COUNT(*) FROM room_players rp
		      JOIN game_sessions gs ON gs.room_id = rp.room_id
		      WHERE gs.id = $1)
		     =
		     array_length(gs.%s, 1)
		 FROM game_sessions gs
		 WHERE gs.id = $1`,
		voteColumn,
	)

	var allVoted bool
	err := s.pool.QueryRow(ctx, query, sessionID).Scan(&allVoted)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, fmt.Errorf("allPlayersVoted: session not found")
		}
		return false, fmt.Errorf("allPlayersVoted (%s): %w", voteColumn, err)
	}
	return allVoted, nil
}
