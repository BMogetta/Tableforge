package api

import (
	"github.com/recess/game-server/internal/platform/store"
)

// SessionResponse is returned by GET /sessions/{sessionID}.
type SessionResponse struct {
	Session GameSessionDTO    `json:"session"`
	State   interface{}       `json:"state"`
	Result  *store.GameResult `json:"result"`
}

// MoveAckResponse is returned by POST /sessions/{sessionID}/move
// and POST /sessions/{sessionID}/surrender. It confirms the move was
// accepted without echoing state — clients receive the authoritative
// state update via the WebSocket broadcast.
type MoveAckResponse struct {
	MoveNumber int  `json:"move_number"`
	IsOver     bool `json:"is_over"`
}

// RematchResponse is returned by POST /sessions/{sessionID}/rematch.
type RematchResponse struct {
	Votes        int `json:"votes"`
	TotalPlayers int `json:"total_players"`
}
