// shared/events/events.go
//
// Async event contracts for inter-service communication via Redis Pub/Sub.
//
// Conventions:
//   - Channel names follow the pattern:  {domain}.{entity}.{verb}
//   - All events carry a metadata header: event_id, occurred_at, version
//   - Payloads are JSON. Each event type has a dedicated Go struct.
//   - Publishers own the channel. Consumers must not write to it.
//   - Breaking changes require a new channel version: game.session.finished.v2
//
// Channel ownership:
//   game-service    → game.session.finished, game.move.applied
//   match-service   → match.found, match.cancelled, match.ready
//   user-service    → player.banned, player.unbanned, friendship.requested, friendship.accepted, admin.broadcast.sent
//   auth-service    → player.session.revoked
//
// These are DIFFERENT from the ws hub event types (those are client-facing).
// Inter-service events are internal — they never go directly to the browser.

package events

import "time"

// ─── Metadata ────────────────────────────────────────────────────────────────

// Meta is embedded in every event for tracing and idempotency.
type Meta struct {
	EventID    string    `json:"event_id"`    // UUID — idempotency key for consumers
	OccurredAt time.Time `json:"occurred_at"` // when the event happened
	Version    int       `json:"version"`     // schema version, starts at 1
}

// ─── game-service events ─────────────────────────────────────────────────────

// Channel: game.session.finished
// Published by: game-service when a session ends (any reason).
// Consumers:
//   user-service   → update ratings (ranked only), update win/loss streaks
//   replay-service → close the replay record, mark as complete
//   match-service  → unblock the room for potential rematch queue re-entry
type GameSessionFinished struct {
	Meta
	SessionID    string         `json:"session_id"`
	RoomID       string         `json:"room_id"`
	GameID       string         `json:"game_id"`
	Mode         string         `json:"mode"`          // "casual" | "ranked"
	EndedBy      string         `json:"ended_by"`      // "win" | "draw" | "forfeit" | "suspended"
	WinnerID     string         `json:"winner_id"`     // empty string if draw or suspended
	IsDraw       bool           `json:"is_draw"`
	DurationSecs int            `json:"duration_secs"`
	Players      []SessionPlayer `json:"players"`
}

type SessionPlayer struct {
	PlayerID string `json:"player_id"`
	Seat     int    `json:"seat"`
	Outcome  string `json:"outcome"` // "win" | "loss" | "draw" | "forfeit"
}

// Channel: game.move.applied
// Published by: game-service after each valid move.
// Consumers:
//   replay-service → append to live replay buffer (optional — stream already covers this)
// Note: replay-service can consume session_events stream directly on flush.
// This event exists for consumers that need real-time move notifications
// outside the WS layer (e.g. a future spectator analytics service).
type GameMoveApplied struct {
	Meta
	SessionID  string `json:"session_id"`
	GameID     string `json:"game_id"`
	PlayerID   string `json:"player_id"`
	MoveNumber int    `json:"move_number"`
	Payload    string `json:"payload"`    // JSON-encoded engine.Move.Payload
}

// ─── match-service events ─────────────────────────────────────────────────────

// Channel: match.found
// Published by: match-service after pairing two players and creating the room.
// Consumers:
//   ws/hub (player channel) → deliver match_found WS event to both players
// Note: this triggers the accept/decline UI on the client side.
type MatchFound struct {
	Meta
	MatchID   string `json:"match_id"`   // queue:pending:{match_id} key
	RoomID    string `json:"room_id"`
	RoomCode  string `json:"room_code"`
	GameID    string `json:"game_id"`
	PlayerAID string `json:"player_a_id"`
	PlayerBID string `json:"player_b_id"`
	MmrA      float64 `json:"mmr_a"`
	MmrB      float64 `json:"mmr_b"`
}

// Channel: match.cancelled
// Published by: match-service when accept window expires or a player declines.
// Consumers:
//   ws/hub (player channel) → deliver match_cancelled WS event
type MatchCancelled struct {
	Meta
	MatchID  string `json:"match_id"`
	PlayerAID string `json:"player_a_id"`
	PlayerBID string `json:"player_b_id"`
	Reason   string `json:"reason"` // "declined" | "timeout"
}

// Channel: match.ready
// Published by: match-service when both players accept and session is started.
// Consumers:
//   ws/hub (player channel) → deliver match_ready WS event with session_id
type MatchReady struct {
	Meta
	MatchID   string `json:"match_id"`
	SessionID string `json:"session_id"`
	RoomID    string `json:"room_id"`
	RoomCode  string `json:"room_code"`
	PlayerAID string `json:"player_a_id"`
	PlayerBID string `json:"player_b_id"`
}

// ─── user-service events ──────────────────────────────────────────────────────

// Channel: player.banned
// Published by: user-service when a ban is issued.
// Consumers:
//   auth-service   → revoke active session (force logout)
//   match-service  → remove from queue if present
//   game-service   → forfeit any active ranked sessions
type PlayerBanned struct {
	Meta
	PlayerID  string  `json:"player_id"`
	BanID     string  `json:"ban_id"`
	BannedBy  string  `json:"banned_by"`
	Reason    string  `json:"reason"`
	ExpiresAt *string `json:"expires_at"` // nil = permanent, RFC3339 if temporary
}

// Channel: player.unbanned
// Published by: user-service when a ban is lifted.
// Consumers:
//   auth-service → no action needed (player can log in again naturally)
type PlayerUnbanned struct {
	Meta
	PlayerID string `json:"player_id"`
	BanID    string `json:"ban_id"`
	LiftedBy string `json:"lifted_by"`
}

// Channel: friendship.requested
// Published by: user-service when a friend request is sent.
// Consumers:
//   notification-service → send in-app notification to addressee
type FriendshipRequested struct {
	Meta
	RequesterID      string `json:"requester_id"`
	RequesterUsername string `json:"requester_username"`
	AddresseeID      string `json:"addressee_id"`
}

// Channel: friendship.accepted
// Published by: user-service when a friend request is accepted.
// Consumers:
//   notification-service → send in-app notification to requester
// Note: the notification row is created by user-service directly.
// This event exists for future consumers (e.g. presence service showing
// "your friend X just came online").
type FriendshipAccepted struct {
	Meta
	RequesterID       string `json:"requester_id"`
	AddresseeID       string `json:"addressee_id"`
	AddresseeUsername string `json:"addressee_username"` // username of who accepted
}

// ─── admin events ────────────────────────────────────────────────────────────

// Channel: admin.broadcast.sent
// Published by: user-service when an admin sends a broadcast message.
// Consumers:
//   ws-gateway → fan out to ALL connected WebSocket clients
type AdminBroadcastSent struct {
	Meta
	Message       string `json:"message"`
	BroadcastType string `json:"broadcast_type"` // "info" | "warning"
	SentBy        string `json:"sent_by"`        // admin player_id
}

// ─── auth-service events ──────────────────────────────────────────────────────

// Channel: player.session.revoked
// Published by: auth-service when a session is explicitly revoked
// (logout, new login on another device, or triggered by player.banned).
// Consumers:
//   ws/hub (player channel) → close the WebSocket connection for that session
type PlayerSessionRevoked struct {
	Meta
	PlayerID  string `json:"player_id"`
	SessionID string `json:"session_id"` // the session_id that was revoked
	Reason    string `json:"reason"`     // "logout" | "superseded" | "banned"
}
