package runtime

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// ReadyVoteResult carries the outcome of a VoteReady call.
type ReadyVoteResult struct {
	ReadyPlayers []string `json:"ready_players"`
	Required     int      `json:"required"`
	AllReady     bool     `json:"all_ready"`
}

// VoteReady registers a ready confirmation for playerID on the given session.
// When all human participants have confirmed, AllReady is true and the caller
// should broadcast game_ready and start the TurnTimer.
// Bots are auto-confirmed since they live on the server and need no asset loading.
func (svc *Service) VoteReady(ctx context.Context, sessionID, playerID uuid.UUID) (ReadyVoteResult, error) {
	session, err := svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return ReadyVoteResult{}, ErrSessionNotFound
	}
	if session.FinishedAt != nil {
		return ReadyVoteResult{}, ErrGameOver
	}
	players, err := svc.store.ListRoomPlayers(ctx, session.RoomID)
	if err != nil {
		return ReadyVoteResult{}, fmt.Errorf("VoteReady: list players: %w", err)
	}
	callerFound := false
	for _, p := range players {
		if p.PlayerID == playerID {
			callerFound = true
			break
		}
	}
	if !callerFound {
		return ReadyVoteResult{}, ErrNotParticipant
	}
	allReady, err := svc.store.VoteReady(ctx, sessionID, playerID)
	if err != nil {
		return ReadyVoteResult{}, fmt.Errorf("VoteReady: store vote: %w", err)
	}
	// Auto-confirm bots — they live on the server and need no asset loading.
	// Uses isBot() (store fallback) so bots are detected even after a server restart.
	for _, p := range players {
		if svc.isBot(ctx, p.PlayerID) {
			ready, _ := svc.store.VoteReady(ctx, sessionID, p.PlayerID)
			if ready {
				allReady = true
			}
		}
	}
	session, err = svc.store.GetGameSession(ctx, sessionID)
	if err != nil {
		return ReadyVoteResult{}, fmt.Errorf("VoteReady: reload: %w", err)
	}
	return ReadyVoteResult{
		ReadyPlayers: session.ReadyPlayers,
		Required:     len(players),
		AllReady:     allReady,
	}, nil
}
