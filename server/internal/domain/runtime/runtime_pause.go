package runtime

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

var (
	// ErrAlreadyPaused is returned when a pause vote is cast on a session
	// that is already suspended.
	ErrAlreadyPaused = fmt.Errorf("session is already paused")
	// ErrNotSuspended is returned when a resume vote is cast on a session
	// that is not currently suspended.
	ErrNotSuspended = fmt.Errorf("session is not paused")
)

// PauseVoteResult carries the outcome of a VotePause or VoteResume call.
type PauseVoteResult struct {
	// Votes is the list of player IDs that have voted so far.
	Votes []string `json:"votes"`
	// Required is the total number of participants who must vote.
	Required int `json:"required"`
	// AllVoted is true when consensus was reached and the session state changed.
	AllVoted bool `json:"all_voted"`
}

// VotePause registers a pause vote for playerID on the given session.
// When all participants have voted the session is suspended and AllVoted is true.
// Returns ErrAlreadyPaused if the session is already suspended.
// Returns ErrNotParticipant if the caller is not in the session's room.
// Returns ErrGameOver if the session is already finished.
func (svc *Service) VotePause(ctx context.Context, sessionID, playerID uuid.UUID) (PauseVoteResult, error) {
	session, err := svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return PauseVoteResult{}, ErrSessionNotFound
	}
	if session.FinishedAt != nil {
		return PauseVoteResult{}, ErrGameOver
	}
	if session.SuspendedAt != nil {
		return PauseVoteResult{}, ErrAlreadyPaused
	}

	players, err := svc.store.ListRoomPlayers(ctx, session.RoomID)
	if err != nil {
		return PauseVoteResult{}, fmt.Errorf("VotePause: list players: %w", err)
	}

	callerFound := false
	for _, p := range players {
		if p.PlayerID == playerID {
			callerFound = true
			break
		}
	}
	if !callerFound {
		return PauseVoteResult{}, ErrNotParticipant
	}

	if _, err := svc.store.VotePause(ctx, sessionID, playerID); err != nil {
		return PauseVoteResult{}, fmt.Errorf("VotePause: store vote: %w", err)
	}

	// Auto-vote for any registered bots so they never block pause consensus.
	for _, p := range players {
		if _, isBot := svc.bots.get(p.PlayerID); isBot {
			_, _ = svc.store.VotePause(ctx, sessionID, p.PlayerID)
		}
	}

	// Re-check consensus after bot votes — re-use playerID (idempotent).
	allVoted, err := svc.store.VotePause(ctx, sessionID, playerID)
	if err != nil {
		return PauseVoteResult{}, fmt.Errorf("VotePause: recheck: %w", err)
	}

	if allVoted {
		if svc.timer != nil {
			svc.timer.Cancel(sessionID)
		}
		if err := svc.store.SuspendSession(ctx, sessionID, "pause_vote"); err != nil {
			return PauseVoteResult{}, fmt.Errorf("VotePause: suspend session: %w", err)
		}
		if err := svc.store.ClearPauseVotes(ctx, sessionID); err != nil {
			return PauseVoteResult{}, fmt.Errorf("VotePause: clear votes: %w", err)
		}
	}

	// Reload to get the current pause_votes after the store write.
	session, err = svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return PauseVoteResult{}, fmt.Errorf("VotePause: reload session: %w", err)
	}

	return PauseVoteResult{
		Votes:    session.PauseVotes,
		Required: len(players),
		AllVoted: allVoted,
	}, nil
}

// VoteResume registers a resume vote for playerID on the given session.
// When all participants have voted the session is resumed and AllVoted is true.
// Returns ErrNotSuspended if the session is not currently suspended.
// Returns ErrNotParticipant if the caller is not in the session's room.
// Returns ErrGameOver if the session is already finished.
func (svc *Service) VoteResume(ctx context.Context, sessionID, playerID uuid.UUID) (PauseVoteResult, error) {
	session, err := svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return PauseVoteResult{}, ErrSessionNotFound
	}
	if session.FinishedAt != nil {
		return PauseVoteResult{}, ErrGameOver
	}
	if session.SuspendedAt == nil {
		return PauseVoteResult{}, ErrNotSuspended
	}

	players, err := svc.store.ListRoomPlayers(ctx, session.RoomID)
	if err != nil {
		return PauseVoteResult{}, fmt.Errorf("VoteResume: list players: %w", err)
	}

	callerFound := false
	for _, p := range players {
		if p.PlayerID == playerID {
			callerFound = true
			break
		}
	}
	if !callerFound {
		return PauseVoteResult{}, ErrNotParticipant
	}

	if _, err := svc.store.VoteResume(ctx, sessionID, playerID); err != nil {
		return PauseVoteResult{}, fmt.Errorf("VoteResume: store vote: %w", err)
	}

	// Auto-vote for any registered bots so they never block resume consensus.
	for _, p := range players {
		if _, isBot := svc.bots.get(p.PlayerID); isBot {
			_, _ = svc.store.VoteResume(ctx, sessionID, p.PlayerID)
		}
	}

	// Re-check consensus after bot votes — re-use playerID (idempotent).
	allVoted, err := svc.store.VoteResume(ctx, sessionID, playerID)
	if err != nil {
		return PauseVoteResult{}, fmt.Errorf("VoteResume: recheck: %w", err)
	}

	if allVoted {
		if err := svc.store.ResumeSession(ctx, sessionID); err != nil {
			return PauseVoteResult{}, fmt.Errorf("VoteResume: resume session: %w", err)
		}
		if err := svc.store.ClearResumeVotes(ctx, sessionID); err != nil {
			return PauseVoteResult{}, fmt.Errorf("VoteResume: clear votes: %w", err)
		}
		if svc.timer != nil {
			// Reschedule the turn timer after resuming — the session now has
			// an active turn that needs a timeout.
			resumed, err := svc.store.GetGameSession(ctx, sessionID)
			if err == nil {
				svc.timer.Schedule(resumed)
			}
		}
	}

	// Reload to get the current resume_votes after the store write.
	session, err = svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return PauseVoteResult{}, fmt.Errorf("VoteResume: reload session: %w", err)
	}

	return PauseVoteResult{
		Votes:    session.ResumeVotes,
		Required: len(players),
		AllVoted: allVoted,
	}, nil
}
