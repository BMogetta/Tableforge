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
	// EventPresenceUpdated is broadcast to all room clients when a player
	// connects or disconnects.
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

// spectatorBlocklist lists events that are never delivered to spectator clients.
var spectatorBlocklist = map[EventType]bool{
	EventRematchVote:      true,
	EventPauseVoteUpdate:  true,
	EventSessionSuspended: true,
	EventResumeVoteUpdate: true,
	EventSessionResumed:   true,
}

// Event is the envelope sent to clients in a room.
type Event struct {
	Type    EventType `json:"type"`
	Payload any       `json:"payload"`
}

// filteredEnvelope is used internally when publishing via Redis to carry both
// the player-facing and spectator-facing payloads in a single pub/sub message.
// The s (spectator_only) flag tells listenRoom which fanout path to use.
type filteredEnvelope struct {
	S    bool            `json:"s,omitempty"` // true → deliver only to spectators
	Data json.RawMessage `json:"d"`
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

	rdb *redis.Client // nil → single-instance mode
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

// Broadcast publishes an event to all clients in a room (players + spectators),
// respecting the spectatorBlocklist. Use BroadcastFiltered when players and
// spectators need different state payloads (e.g. games with private hands).
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
		env, err := json.Marshal(filteredEnvelope{S: false, Data: data})
		if err != nil {
			log.Printf("ws: marshal broadcast envelope: %v", err)
			return
		}
		if err := h.rdb.Publish(context.Background(), channelKey(roomID), env).Err(); err != nil {
			log.Printf("ws: redis publish: %v", err)
		}
		return
	}

	h.fanout(roomID, event.Type, data)
}

// BroadcastFiltered sends playerEvent to non-spectator clients and
// spectatorEvent to spectator clients in the same room.
// Use this for games with private state (e.g. Love Letter hands).
// Events in spectatorBlocklist are still suppressed for spectators.
func (h *Hub) BroadcastFiltered(roomID uuid.UUID, playerEvent Event, spectatorEvent Event) {
	if h == nil {
		return
	}

	var playerData []byte
	if playerEvent.Payload != nil {
		var err error
		playerData, err = json.Marshal(playerEvent)
		if err != nil {
			log.Printf("ws: BroadcastFiltered marshal player event: %v", err)
			return
		}
	}
	spectatorData, err := json.Marshal(spectatorEvent)
	if err != nil {
		log.Printf("ws: BroadcastFiltered marshal spectator event: %v", err)
		return
	}

	if h.rdb != nil {
		// Publish player payload (s=false) and spectator payload (s=true) separately.
		// listenRoom routes each to the correct client set.
		playerEnv, err := json.Marshal(filteredEnvelope{S: false, Data: playerData})
		if err != nil {
			log.Printf("ws: BroadcastFiltered marshal player envelope: %v", err)
			return
		}
		spectatorEnv, err := json.Marshal(filteredEnvelope{S: true, Data: spectatorData})
		if err != nil {
			log.Printf("ws: BroadcastFiltered marshal spectator envelope: %v", err)
			return
		}
		if err := h.rdb.Publish(context.Background(), channelKey(roomID), playerEnv).Err(); err != nil {
			log.Printf("ws: redis publish player event: %v", err)
		}
		if err := h.rdb.Publish(context.Background(), channelKey(roomID), spectatorEnv).Err(); err != nil {
			log.Printf("ws: redis publish spectator event: %v", err)
		}
		return
	}

	h.fanoutFiltered(roomID, playerEvent.Type, playerData, spectatorData)
}

// listenRoom subscribes to the Redis channel for roomID and fans out messages
// to local clients. It exits when there are no more local clients for the room.
// Messages are wrapped in filteredEnvelope to support BroadcastFiltered routing.
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

		var env filteredEnvelope
		if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
			log.Printf("ws: listenRoom unmarshal envelope: %v", err)
			continue
		}

		// Parse the event type for spectatorBlocklist checks.
		var header struct {
			Type EventType `json:"type"`
		}
		_ = json.Unmarshal(env.Data, &header)

		if env.S {
			// Spectator-only payload from BroadcastFiltered.
			h.fanoutSpectatorsOnly(roomID, header.Type, env.Data)
		} else {
			// Player payload — or a plain Broadcast (s=false, goes to everyone).
			// For BroadcastFiltered the spectator payload arrives separately (s=true).
			// For plain Broadcast there is no separate spectator message, so
			// non-spectators get this and spectators also get it (unless blocklisted).
			h.fanout(roomID, header.Type, env.Data)
		}
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

// fanoutFiltered delivers playerData to non-spectators and spectatorData to
// spectators. Events in spectatorBlocklist are suppressed for spectators.
func (h *Hub) fanoutFiltered(roomID uuid.UUID, eventType EventType, playerData, spectatorData []byte) {
	h.mu.RLock()
	clients := h.rooms[roomID]
	h.mu.RUnlock()

	blocked := spectatorBlocklist[eventType]

	for c := range clients {
		if c.spectator {
			if blocked {
				continue
			}
			select {
			case c.send <- spectatorData:
			default:
				log.Printf("ws: dropping slow spectator in room %s", roomID)
				close(c.send)
			}
		} else {
			if playerData == nil {
				continue
			}
			select {
			case c.send <- playerData:
			default:
				log.Printf("ws: dropping slow client in room %s", roomID)
				close(c.send)
			}
		}
	}
}

// fanoutSpectatorsOnly delivers data only to spectator clients in a room.
// Used by listenRoom when processing a spectator-only envelope (s=true).
func (h *Hub) fanoutSpectatorsOnly(roomID uuid.UUID, eventType EventType, data []byte) {
	h.mu.RLock()
	clients := h.rooms[roomID]
	h.mu.RUnlock()

	if spectatorBlocklist[eventType] {
		return
	}

	for c := range clients {
		if !c.spectator {
			continue
		}
		select {
		case c.send <- data:
		default:
			log.Printf("ws: dropping slow spectator in room %s", roomID)
			close(c.send)
		}
	}
}
