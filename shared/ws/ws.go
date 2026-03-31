// Package ws defines the client-facing WebSocket event contract shared between
// the game-server (publisher) and ws-gateway (subscriber/fan-out).
//
// These types are distinct from shared/events, which defines inter-service
// async events that never go directly to the browser.
package ws

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// EventType identifies the kind of server-sent event delivered to clients.
type EventType string

const (
	EventMoveApplied          EventType = "move_applied"
	EventGameOver             EventType = "game_over"
	EventPlayerJoined         EventType = "player_joined"
	EventPlayerLeft           EventType = "player_left"
	EventOwnerChanged         EventType = "owner_changed"
	EventRoomClosed           EventType = "room_closed"
	EventGameStarted          EventType = "game_started"
	EventRematchVote          EventType = "rematch_vote"
	EventGameReady            EventType = "game_ready"
	EventPlayerReady          EventType = "player_ready"
	EventRematchReady         EventType = "rematch_ready"
	EventSettingUpdated       EventType = "setting_updated"
	EventSpectatorJoined      EventType = "spectator_joined"
	EventSpectatorLeft        EventType = "spectator_left"
	EventPresenceUpdate       EventType = "presence_update"
	EventChatMessage          EventType = "chat_message"
	EventChatMessageHidden    EventType = "chat_message_hidden"
	EventDMReceived           EventType = "dm_received"
	EventDMRead               EventType = "dm_read"
	EventPauseVoteUpdate      EventType = "pause_vote_update"
	EventSessionSuspended     EventType = "session_suspended"
	EventResumeVoteUpdate     EventType = "resume_vote_update"
	EventSessionResumed       EventType = "session_resumed"
	EventQueueJoined          EventType = "queue_joined"
	EventQueueLeft            EventType = "queue_left"
	EventMatchFound           EventType = "match_found"
	EventMatchCancelled       EventType = "match_cancelled"
	EventMatchReady           EventType = "match_ready"
	EventNotificationReceived EventType = "notification_received"
)

// SpectatorBlocklist lists events that must never be delivered to spectators.
var SpectatorBlocklist = map[EventType]bool{
	EventRematchVote:      true,
	EventPauseVoteUpdate:  true,
	EventSessionSuspended: true,
	EventResumeVoteUpdate: true,
	EventSessionResumed:   true,
}

// Event is the envelope sent to WebSocket clients.
type Event struct {
	Type    EventType `json:"type"`
	Payload any       `json:"payload"`
}

// FilteredEnvelope is the Redis message wrapper for room channels.
// Spectators is true when the message should be delivered to spectators only.
type FilteredEnvelope struct {
	Spectators bool            `json:"s,omitempty"`
	Data       json.RawMessage `json:"d"`
}

// RoomChannelKey returns the Redis pub/sub channel for room-scoped events.
func RoomChannelKey(roomID uuid.UUID) string {
	return fmt.Sprintf("ws:room:%s", roomID)
}

// PlayerChannelKey returns the Redis pub/sub channel for player-scoped events.
func PlayerChannelKey(playerID uuid.UUID) string {
	return fmt.Sprintf("ws:player:%s", playerID)
}
