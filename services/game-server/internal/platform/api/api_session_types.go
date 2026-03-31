package api

import (
	"github.com/tableforge/game-server/internal/domain/engine"
	"github.com/tableforge/game-server/internal/platform/store"
)

// SessionResponse is returned by GET /sessions/{sessionID}.
type SessionResponse struct {
	Session GameSessionDTO    `json:"session"`
	State   interface{}       `json:"state"`
	Result  *store.GameResult `json:"result"`
}

// MoveResponse is returned by POST /sessions/{sessionID}/move
// and POST /sessions/{sessionID}/surrender.
// Mirrors runtime.MoveResult with JSON-friendly field names.
type MoveResponse struct {
	Session GameSessionDTO `json:"session"`
	State   interface{}    `json:"state"`
	IsOver  bool           `json:"is_over"`
	Result  *engine.Result `json:"result,omitempty"`
}

// RematchResponse is returned by POST /sessions/{sessionID}/rematch.
type RematchResponse struct {
	Votes        int `json:"votes"`
	TotalPlayers int `json:"total_players"`
}
