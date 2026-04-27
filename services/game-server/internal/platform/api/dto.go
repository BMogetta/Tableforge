package api

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/platform/events"
	"github.com/recess/game-server/internal/platform/store"
)

// ---------------------------------------------------------------------------
// GameSessionDTO
//
// Client-facing representation of a game session.
// Replaces store.GameSession in API responses to:
//   - Exclude internal fields (State []byte, DeletedAt)
//   - Control JSON serialization precisely
//   - Decouple the API contract from the internal store model
// ---------------------------------------------------------------------------

type GameSessionDTO struct {
	ID              uuid.UUID  `json:"id"`
	RoomID          uuid.UUID  `json:"room_id"`
	GameID          string     `json:"game_id"`
	Name            *string    `json:"name,omitempty"`
	Mode            string     `json:"mode"`
	MoveCount       int        `json:"move_count"`
	SuspendCount    int        `json:"suspend_count"`
	SuspendedAt     *time.Time `json:"suspended_at,omitempty"`
	SuspendedReason *string    `json:"suspended_reason,omitempty"`
	ReadyPlayers    []string   `json:"ready_players"`
	TurnTimeoutSecs *int       `json:"turn_timeout_secs,omitempty"`
	LastMoveAt      time.Time  `json:"last_move_at"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
}

// sessionToDTO converts a store.GameSession to a GameSessionDTO.
// State []byte and DeletedAt are intentionally excluded.
func sessionToDTO(s store.GameSession) GameSessionDTO {
	return GameSessionDTO{
		ID:              s.ID,
		RoomID:          s.RoomID,
		GameID:          s.GameID,
		Name:            s.Name,
		Mode:            string(s.Mode),
		MoveCount:       s.MoveCount,
		SuspendCount:    s.SuspendCount,
		SuspendedAt:     s.SuspendedAt,
		SuspendedReason: s.SuspendedReason,
		ReadyPlayers:    s.ReadyPlayers,
		TurnTimeoutSecs: s.TurnTimeoutSecs,
		LastMoveAt:      s.LastMoveAt,
		StartedAt:       s.StartedAt,
		FinishedAt:      s.FinishedAt,
	}
}

// MoveDTO is the client-facing representation of a session move.
// StateAfter is base64-encoded JSON of the game state after this move.
type MoveDTO struct {
	ID         uuid.UUID   `json:"id"`
	SessionID  uuid.UUID   `json:"session_id"`
	PlayerID   uuid.UUID   `json:"player_id"`
	Payload    interface{} `json:"payload"`
	StateAfter string      `json:"state_after,omitempty"`
	MoveNumber int         `json:"move_number"`
	AppliedAt  time.Time   `json:"applied_at"`
}

func moveToDTO(m store.Move) MoveDTO {
	var payload interface{}
	if len(m.Payload) > 0 {
		_ = json.Unmarshal(m.Payload, &payload)
	}
	var stateAfter string
	if len(m.StateAfter) > 0 {
		stateAfter = base64.StdEncoding.EncodeToString(m.StateAfter)
	}
	return MoveDTO{
		ID:         m.ID,
		SessionID:  m.SessionID,
		PlayerID:   m.PlayerID,
		Payload:    payload,
		StateAfter: stateAfter,
		MoveNumber: m.MoveNumber,
		AppliedAt:  m.AppliedAt,
	}
}

// SessionEventDTO is the client-facing representation of a session event.
type SessionEventDTO struct {
	ID         string      `json:"id"`
	SessionID  uuid.UUID   `json:"session_id"`
	EventType  string      `json:"event_type"`
	PlayerID   *uuid.UUID  `json:"player_id,omitempty"`
	Payload    interface{} `json:"payload,omitempty"`
	OccurredAt time.Time   `json:"occurred_at"`
}

func sessionEventToDTO(e events.Event) SessionEventDTO {
	var payload interface{}
	if len(e.Payload) > 0 {
		_ = json.Unmarshal(e.Payload, &payload)
	}
	return SessionEventDTO{
		ID:         e.ID,
		SessionID:  e.SessionID,
		EventType:  string(e.Type),
		PlayerID:   e.PlayerID,
		Payload:    payload,
		OccurredAt: e.OccurredAt,
	}
}
