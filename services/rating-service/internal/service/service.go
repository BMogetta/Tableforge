package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/tableforge/rating-service/internal/store"
	"github.com/tableforge/shared/domain/rating"
	"github.com/tableforge/shared/events"
)

// Service orchestrates rating updates triggered by game.session.finished events.
type Service struct {
	store  store.Store
	engine *rating.Engine
	log    *slog.Logger
}

func New(st store.Store, engine *rating.Engine, log *slog.Logger) *Service {
	return &Service{store: st, engine: engine, log: log}
}

// ProcessGameFinished handles one game.session.finished event.
//
// Rank derivation from SessionPlayer.Outcome:
//
//	"win"           → rank 1
//	"draw"          → rank 1  (all share rank 1, engine delta ≈ 0 for equal MMRs)
//	"loss"          → rank 2
//	"forfeit"       → rank 2
//
// When GameSessionFinished gains explicit multi-team placement ranks, add a
// Rank field to SessionPlayer and use it directly instead of outcomeToRank.
func (s *Service) ProcessGameFinished(ctx context.Context, evt events.GameSessionFinished) error {
	if evt.Mode != "ranked" {
		s.log.Debug("skipping non-ranked session", "session_id", evt.SessionID)
		return nil
	}
	if len(evt.Players) < 2 {
		s.log.Warn("session has fewer than 2 players, skipping", "session_id", evt.SessionID)
		return nil
	}

	// Parse player UUIDs up front — fail fast on bad data.
	playerIDs := make([]uuid.UUID, len(evt.Players))
	for i, sp := range evt.Players {
		id, err := uuid.Parse(sp.PlayerID)
		if err != nil {
			return fmt.Errorf("invalid player_id %q: %w", sp.PlayerID, err)
		}
		playerIDs[i] = id
	}

	// Load current ratings — returns a map with defaults for missing rows.
	ratingsByID, err := s.store.GetRatings(ctx, playerIDs, evt.GameID)
	if err != nil {
		return fmt.Errorf("load ratings: %w", err)
	}

	// Build rating.Player values from store rows.
	// Keep mmrBefore snapshot before ProcessMatch mutates the structs.
	mmrBefore := make(map[uuid.UUID]float64, len(playerIDs))
	ratingPlayers := make(map[uuid.UUID]*rating.Player, len(playerIDs))
	for _, id := range playerIDs {
		row := ratingsByID[id]
		mmrBefore[id] = row.MMR
		ratingPlayers[id] = &rating.Player{
			ID:            id.String(),
			MMR:           row.MMR,
			DisplayRating: row.DisplayRating,
			GamesPlayed:   row.GamesPlayed,
			WinStreak:     row.WinStreak,
			LossStreak:    row.LossStreak,
		}
	}

	// Build MatchResult — each SessionPlayer is a solo team (1v1 / FFA individuals).
	placements := make([]rating.Placement, len(evt.Players))
	for i, sp := range evt.Players {
		id := playerIDs[i]
		placements[i] = rating.Placement{
			Team: &rating.Team{Players: []*rating.Player{ratingPlayers[id]}},
			Rank: outcomeToRank(sp.Outcome),
		}
	}

	result := &rating.MatchResult{Placements: placements}
	if err := result.Validate(); err != nil {
		return fmt.Errorf("invalid match result: %w", err)
	}

	// ProcessMatch mutates ratingPlayers values in-place.
	deltas, err := s.engine.ProcessMatch(result)
	if err != nil {
		return fmt.Errorf("process match: %w", err)
	}

	// Collect upsert + history rows.
	updates := make([]*store.PlayerRating, 0, len(playerIDs))
	history := make([]store.HistoryEntry, 0, len(playerIDs))

	resultID, _ := uuid.Parse(evt.SessionID) // session_id == game_results.id in current schema

	for _, id := range playerIDs {
		rp := ratingPlayers[id]
		updates = append(updates, &store.PlayerRating{
			PlayerID:      id,
			GameID:        evt.GameID,
			MMR:           rp.MMR,
			DisplayRating: rp.DisplayRating,
			GamesPlayed:   rp.GamesPlayed,
			WinStreak:     rp.WinStreak,
			LossStreak:    rp.LossStreak,
		})
		history = append(history, store.HistoryEntry{
			PlayerID:  id,
			GameID:    evt.GameID,
			ResultID:  resultID,
			MMRBefore: mmrBefore[id],
			MMRAfter:  rp.MMR,
			Delta:     deltas[rp.ID],
		})
	}

	if err := s.store.UpsertRatings(ctx, updates, history); err != nil {
		return fmt.Errorf("persist ratings: %w", err)
	}

	s.log.Info("ratings updated",
		"session_id", evt.SessionID,
		"game_id", evt.GameID,
		"players", len(evt.Players),
	)
	return nil
}

func outcomeToRank(outcome string) int {
	switch outcome {
	case "win", "draw":
		return 1
	default: // "loss", "forfeit"
		return 2
	}
}
