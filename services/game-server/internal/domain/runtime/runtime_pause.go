package runtime

import (
	"context"
	"fmt"
	"time"

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
	// Votes is the number of players that have voted so far.
	Votes int `json:"votes"`
	// Required is the number of human participants who must vote.
	Required int `json:"required"`
	// AllVoted is true when consensus was reached and the session state changed.
	AllVoted bool `json:"all_voted"`
}

// VotePause registers a pause vote for playerID on the given session.
// When all human participants have voted the session is suspended and AllVoted is true.
// Bots are excluded from the vote count — they never block pause consensus.
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
	humanCount := 0
	for _, p := range players {
		if p.PlayerID == playerID {
			callerFound = true
		}
		if !svc.isBot(ctx, p.PlayerID) {
			humanCount++
		}
	}
	if !callerFound {
		return PauseVoteResult{}, ErrNotParticipant
	}

	allVoted, err := svc.store.VotePause(ctx, sessionID, playerID)
	if err != nil {
		return PauseVoteResult{}, fmt.Errorf("VotePause: store vote: %w", err)
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

	voteCount, err := svc.store.CountPauseVotes(ctx, sessionID)
	if err != nil {
		return PauseVoteResult{}, fmt.Errorf("VotePause: count votes: %w", err)
	}

	return PauseVoteResult{
		Votes:    voteCount,
		Required: humanCount,
		AllVoted: allVoted,
	}, nil
}

// VoteResume registers a resume vote for playerID on the given session.
// When all human participants have voted the session is resumed and AllVoted is true.
// Bots are excluded from the vote count — they never block resume consensus.
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
	humanCount := 0
	for _, p := range players {
		if p.PlayerID == playerID {
			callerFound = true
		}
		if !svc.isBot(ctx, p.PlayerID) {
			humanCount++
		}
	}
	if !callerFound {
		return PauseVoteResult{}, ErrNotParticipant
	}

	allVoted, err := svc.store.VoteResume(ctx, sessionID, playerID)
	if err != nil {
		return PauseVoteResult{}, fmt.Errorf("VoteResume: store vote: %w", err)
	}

	if allVoted {
		if err := svc.store.ResumeSession(ctx, sessionID); err != nil {
			return PauseVoteResult{}, fmt.Errorf("VoteResume: resume session: %w", err)
		}
		if err := svc.store.ClearResumeVotes(ctx, sessionID); err != nil {
			return PauseVoteResult{}, fmt.Errorf("VoteResume: clear votes: %w", err)
		}
		if svc.timer != nil {
			resumed, err := svc.store.GetGameSession(ctx, sessionID)
			if err == nil && resumed.TurnTimeoutSecs != nil && *resumed.TurnTimeoutSecs > 0 {
				penalty := time.Duration(float64(*resumed.TurnTimeoutSecs)*ResumePenalty) * time.Second
				svc.timer.ScheduleIn(resumed.ID, penalty)
			}
		}
	}

	voteCount, err := svc.store.CountResumeVotes(ctx, sessionID)
	if err != nil {
		return PauseVoteResult{}, fmt.Errorf("VoteResume: count votes: %w", err)
	}

	return PauseVoteResult{
		Votes:    voteCount,
		Required: humanCount,
		AllVoted: allVoted,
	}, nil
}
