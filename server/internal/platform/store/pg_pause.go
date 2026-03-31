package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// VotePause registers a pause vote for playerID on the given session.
// Returns allVoted=true when every human participant has voted to pause.
// Idempotent — voting twice has no effect.
func (s *PGStore) VotePause(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID) (bool, error) {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO session_pause_votes (session_id, player_id, vote_type)
		 VALUES ($1, $2, 'pause')
		 ON CONFLICT DO NOTHING`,
		sessionID, playerID,
	)
	if err != nil {
		return false, fmt.Errorf("VotePause: %w", err)
	}

	return s.allPlayersVoted(ctx, sessionID, "pause")
}

// ClearPauseVotes removes all pause votes for the session.
func (s *PGStore) ClearPauseVotes(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM session_pause_votes
		 WHERE session_id = $1 AND vote_type = 'pause'`,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("ClearPauseVotes: %w", err)
	}
	return nil
}

// VoteResume registers a resume vote for playerID on the given session.
// Returns allVoted=true when every human participant has voted to resume.
// Idempotent — voting twice has no effect.
func (s *PGStore) VoteResume(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID) (bool, error) {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO session_pause_votes (session_id, player_id, vote_type)
		 VALUES ($1, $2, 'resume')
		 ON CONFLICT DO NOTHING`,
		sessionID, playerID,
	)
	if err != nil {
		return false, fmt.Errorf("VoteResume: %w", err)
	}

	return s.allPlayersVoted(ctx, sessionID, "resume")
}

// ClearResumeVotes removes all resume votes for the session.
func (s *PGStore) ClearResumeVotes(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM session_pause_votes
		 WHERE session_id = $1 AND vote_type = 'resume'`,
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

// allPlayersVoted returns true when the number of votes of the given type
// equals the number of human participants in the session's room.
// Bots are excluded — they never vote.
func (s *PGStore) allPlayersVoted(ctx context.Context, sessionID uuid.UUID, voteType string) (bool, error) {
	var allVoted bool
	err := s.pool.QueryRow(ctx,
		`SELECT
		     (SELECT COUNT(*)
		      FROM session_pause_votes
		      WHERE session_id = $1 AND vote_type = $2)
		     >=
		     (SELECT COUNT(*)
		      FROM room_players rp
		      JOIN game_sessions gs ON gs.room_id = rp.room_id
		      JOIN players p ON p.id = rp.player_id
		      WHERE gs.id = $1 AND p.is_bot = FALSE)`,
		sessionID, voteType,
	).Scan(&allVoted)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, fmt.Errorf("allPlayersVoted: session not found")
		}
		return false, fmt.Errorf("allPlayersVoted (%s): %w", voteType, err)
	}
	return allVoted, nil
}

func (s *PGStore) CountPauseVotes(ctx context.Context, sessionID uuid.UUID) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM session_pause_votes
         WHERE session_id = $1 AND vote_type = 'pause'`,
		sessionID,
	).Scan(&count)
	return count, err
}

func (s *PGStore) CountResumeVotes(ctx context.Context, sessionID uuid.UUID) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM session_pause_votes
         WHERE session_id = $1 AND vote_type = 'resume'`,
		sessionID,
	).Scan(&count)
	return count, err
}
