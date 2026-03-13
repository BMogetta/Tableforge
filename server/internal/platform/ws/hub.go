package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// EventType identifies the kind of server-sent event.
type EventType string

const (
	EventMoveApplied  EventType = "move_applied"
	EventGameOver     EventType = "game_over"
	EventPlayerJoined EventType = "player_joined"
	EventPlayerLeft   EventType = "player_left"
	EventOwnerChanged EventType = "owner_changed"
	EventRoomClosed   EventType = "room_closed"
	EventGameStarted  EventType = "game_started"
	EventRematchVote  EventType = "rematch_vote"
	// EventRematchReady is broadcast to all clients in the room when every
	// player has voted for a rematch. The payload contains the room_id so
	// clients can navigate back to the lobby without a full page reload.
	EventRematchReady   EventType = "rematch_ready"
	EventSettingUpdated EventType = "setting_updated"
	// EventSpectatorJoined / EventSpectatorLeft are broadcast to all room
	// clients when a spectator connects or disconnects, so the lobby can
	// update the spectator count in real time.
	EventSpectatorJoined EventType = "spectator_joined"
	EventSpectatorLeft   EventType = "spectator_left"
	// EventPresenceUpdated is broadcast to all room clients when a player connects or disconnects, so clients can update the player list in real time. The payload is a list of currently connected player IDs.
	EventPresenceUpdated EventType = "presence_update"
	// EventChatMessage is broadcast to all room clients (including spectators)
	// when a player sends a chat message.
	EventChatMessage EventType = "chat_message"
	// EventChatMessageHidden is broadcast to all room clients when a manager
	// hides a chat message. Clients should remove or redact the message by ID.
	EventChatMessageHidden EventType = "chat_message_hidden"
	// EventDMReceived is delivered to the receiver's player channel when a
	// direct message arrives. Payload: message_id, from, content, timestamp.
	EventDMReceived EventType = "dm_received"
	// EventDMRead is delivered to the sender's player channel when the receiver
	// reads their message. Payload: message_id.
	EventDMRead EventType = "dm_read"
	// EventPauseVoteUpdate is broadcast when a player casts a pause vote.
	// Payload: votes []player_id, required int
	EventPauseVoteUpdate EventType = "pause_vote_update"
	// EventSessionSuspended is broadcast when all players have voted to pause.
	// Payload: suspended_at time.Time
	EventSessionSuspended EventType = "session_suspended"
	// EventResumeVoteUpdate is broadcast when a player casts a resume vote.
	// Payload: votes []player_id, required int
	EventResumeVoteUpdate EventType = "resume_vote_update"
	// EventSessionResumed is broadcast when all players have voted to resume.
	// Payload: resumed_at time.Time
	EventSessionResumed EventType = "session_resumed"
	// EventQueueJoined is delivered to a player when they successfully join the
	// ranked queue. Payload: position, estimated_wait_secs.
	EventQueueJoined EventType = "queue_joined"
	// EventQueueLeft is delivered to a player when they leave the ranked queue.
	// Payload: reason ("declined", "manual").
	EventQueueLeft EventType = "queue_left"
	// EventMatchFound is delivered to both players when a match is proposed.
	// Payload: match_id, quality, timeout.
	EventMatchFound EventType = "match_found"
	// EventMatchCancelled is delivered when a proposed match expires or is
	// declined. Payload: match_id, reason ("timeout", "declined").
	EventMatchCancelled EventType = "match_cancelled"
	// EventMatchReady is delivered to both players when both have accepted and
	// the session has been created. Payload: room_id, session_id.
	EventMatchReady EventType = "match_ready"

	EventNotificationReceived EventType = "notification_received"
)

// spectatorOnlyEvents are never sent to spectators.
// Currently only rematch_vote — spectators cannot vote and don't need the tally.
var spectatorBlocklist = map[EventType]bool{
	EventRematchVote:      true,
	EventPauseVoteUpdate:  true,
	EventSessionSuspended: true,
	EventResumeVoteUpdate: true,
	EventSessionResumed:   true,
}

// Event is the envelope sent to all clients in a room.
type Event struct {
	Type    EventType `json:"type"`
	Payload any       `json:"payload"`
}

// Hub manages per-room client sets and broadcasts events.
// When a Redis client is provided it pub/subs through Redis so that
// multiple server instances can broadcast to each other's local clients.
type Hub struct {
	mu    sync.RWMutex
	rooms map[uuid.UUID]map[*Client]struct{}

	// pmu guards the per-player index used by BroadcastToPlayer.
	pmu     sync.RWMutex
	players map[uuid.UUID]map[*Client]struct{}

	rdb *redis.Client // nil → single-instance mode (original behaviour)
}

func NewHub() *Hub {
	return &Hub{
		rooms:   make(map[uuid.UUID]map[*Client]struct{}),
		players: make(map[uuid.UUID]map[*Client]struct{}),
	}
}

// NewHubWithRedis returns a Hub that uses Redis pub/sub for cross-instance fanout.
// Call Run(ctx) to start the subscription listener.
func NewHubWithRedis(rdb *redis.Client) *Hub {
	return &Hub{
		rooms:   make(map[uuid.UUID]map[*Client]struct{}),
		players: make(map[uuid.UUID]map[*Client]struct{}),
		rdb:     rdb,
	}
}

// channelKey returns the Redis pub/sub channel name for a room.
func channelKey(roomID uuid.UUID) string {
	return fmt.Sprintf("ws:room:%s", roomID)
}

func (h *Hub) subscribe(roomID uuid.UUID, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Client]struct{})
	}
	h.rooms[roomID][c] = struct{}{}

	// If this is the first local client for the room, start a Redis subscription.
	if h.rdb != nil && len(h.rooms[roomID]) == 1 {
		go h.listenRoom(roomID)
	}

	if !c.spectator {
		h.subscribePlayer(c.playerID, c)
	}
}

func (h *Hub) unsubscribe(roomID uuid.UUID, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms[roomID], c)
	if len(h.rooms[roomID]) == 0 {
		delete(h.rooms, roomID)
	}
	if !c.spectator {
		h.unsubscribePlayer(c.playerID, c)
	}
}

// SpectatorCount returns the number of spectators currently connected to a room.
func (h *Hub) SpectatorCount(roomID uuid.UUID) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for c := range h.rooms[roomID] {
		if c.spectator {
			count++
		}
	}
	return count
}

// Broadcast publishes an event. In Redis mode the message is published to
// the Redis channel; listenRoom delivers it to local clients.
// In single-instance mode it fans out directly.
func (h *Hub) Broadcast(roomID uuid.UUID, event Event) {
	if h == nil {
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("ws: marshal event: %v", err)
		return
	}

	if h.rdb != nil {
		if err := h.rdb.Publish(context.Background(), channelKey(roomID), data).Err(); err != nil {
			log.Printf("ws: redis publish: %v", err)
		}
		return
	}

	h.fanout(roomID, event.Type, data)
}

// listenRoom subscribes to the Redis channel for roomID and fans out messages
// to local clients. It exits when there are no more local clients for the room.
func (h *Hub) listenRoom(roomID uuid.UUID) {
	sub := h.rdb.Subscribe(context.Background(), channelKey(roomID))
	defer sub.Close()

	ch := sub.Channel()
	for msg := range ch {
		h.mu.RLock()
		_, active := h.rooms[roomID]
		h.mu.RUnlock()
		if !active {
			return
		}
		// In Redis mode we don't have the EventType at fanout time, so we
		// re-parse just the type field for spectator filtering.
		var envelope struct {
			Type EventType `json:"type"`
		}
		_ = json.Unmarshal([]byte(msg.Payload), &envelope)
		h.fanout(roomID, envelope.Type, []byte(msg.Payload))
	}
}

// fanout delivers data to all local clients in a room.
// Events in spectatorBlocklist are skipped for spectator clients.
func (h *Hub) fanout(roomID uuid.UUID, eventType EventType, data []byte) {
	h.mu.RLock()
	clients := h.rooms[roomID]
	h.mu.RUnlock()

	blocked := spectatorBlocklist[eventType]

	for c := range clients {
		if blocked && c.spectator {
			continue
		}
		select {
		case c.send <- data:
		default:
			log.Printf("ws: dropping slow client in room %s", roomID)
			close(c.send)
		}
	}
}
