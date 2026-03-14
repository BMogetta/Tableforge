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
	// clients when a spectator connects or disconnects.
	EventSpectatorJoined EventType = "spectator_joined"
	EventSpectatorLeft   EventType = "spectator_left"
	// EventPresenceUpdated is broadcast to all room clients when a player
	// connects or disconnects.
	EventPresenceUpdated EventType = "presence_update"
	// EventChatMessage is broadcast to all room clients (including spectators)
	// when a player sends a chat message.
	EventChatMessage EventType = "chat_message"
	// EventChatMessageHidden is broadcast to all room clients when a manager
	// hides a chat message.
	EventChatMessageHidden EventType = "chat_message_hidden"
	// EventDMReceived is delivered to the receiver's player channel.
	EventDMReceived EventType = "dm_received"
	// EventDMRead is delivered to the sender's player channel.
	EventDMRead EventType = "dm_read"
	// EventPauseVoteUpdate is broadcast when a player casts a pause vote.
	EventPauseVoteUpdate EventType = "pause_vote_update"
	// EventSessionSuspended is broadcast when all players have voted to pause.
	EventSessionSuspended EventType = "session_suspended"
	// EventResumeVoteUpdate is broadcast when a player casts a resume vote.
	EventResumeVoteUpdate EventType = "resume_vote_update"
	// EventSessionResumed is broadcast when all players have voted to resume.
	EventSessionResumed EventType = "session_resumed"
	// EventQueueJoined is delivered to a player when they join the ranked queue.
	EventQueueJoined EventType = "queue_joined"
	// EventQueueLeft is delivered to a player when they leave the ranked queue.
	EventQueueLeft EventType = "queue_left"
	// EventMatchFound is delivered to both players when a match is proposed.
	EventMatchFound EventType = "match_found"
	// EventMatchCancelled is delivered when a proposed match expires or is declined.
	EventMatchCancelled EventType = "match_cancelled"
	// EventMatchReady is delivered to both players when both have accepted.
	EventMatchReady EventType = "match_ready"

	EventNotificationReceived EventType = "notification_received"
)

// spectatorBlocklist lists events never delivered to spectator clients.
var spectatorBlocklist = map[EventType]bool{
	EventRematchVote:      true,
	EventPauseVoteUpdate:  true,
	EventSessionSuspended: true,
	EventResumeVoteUpdate: true,
	EventSessionResumed:   true,
}

// Event is the envelope sent to clients.
type Event struct {
	Type    EventType `json:"type"`
	Payload any       `json:"payload"`
}

// filteredEnvelope is used internally when publishing via Redis to carry
// payloads in a single pub/sub message.
// S=true means the payload is intended for spectators only (from BroadcastFiltered).
type filteredEnvelope struct {
	S    bool            `json:"s,omitempty"`
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
// respecting the spectatorBlocklist.
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
// Use this for uniform player payloads with a different spectator payload.
// For per-player filtered payloads (e.g. private hands), use BroadcastToRoom.
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
		if playerData != nil {
			playerEnv, err := json.Marshal(filteredEnvelope{S: false, Data: playerData})
			if err != nil {
				log.Printf("ws: BroadcastFiltered marshal player envelope: %v", err)
				return
			}
			if err := h.rdb.Publish(context.Background(), channelKey(roomID), playerEnv).Err(); err != nil {
				log.Printf("ws: redis publish player event: %v", err)
			}
		}
		spectatorEnv, err := json.Marshal(filteredEnvelope{S: true, Data: spectatorData})
		if err != nil {
			log.Printf("ws: BroadcastFiltered marshal spectator envelope: %v", err)
			return
		}
		if err := h.rdb.Publish(context.Background(), channelKey(roomID), spectatorEnv).Err(); err != nil {
			log.Printf("ws: redis publish spectator event: %v", err)
		}
		return
	}

	h.fanoutFiltered(roomID, playerEvent.Type, playerData, spectatorData)
}

// BroadcastToRoom delivers a potentially different event to each non-spectator
// client in the room by calling eventFn(playerID) per player, and delivers
// spectatorEvent to spectator clients.
//
// playerIDs must be the full list of player UUIDs in the room. This is needed
// in Redis mode where the hub cannot enumerate remote clients — each player's
// filtered payload is published to their personal player channel.
//
// In single-instance mode, playerIDs is ignored and the hub iterates local
// clients directly, so no store call is needed beyond what the caller already has.
func (h *Hub) BroadcastToRoom(
	roomID uuid.UUID,
	playerIDs []uuid.UUID,
	eventFn func(playerID uuid.UUID) Event,
	spectatorEvent Event,
) {
	if h == nil {
		return
	}

	if h.rdb != nil {
		// Redis mode: publish per-player payloads to each player's channel,
		// and the spectator payload to the room channel.
		for _, playerID := range playerIDs {
			event := eventFn(playerID)
			data, err := json.Marshal(event)
			if err != nil {
				log.Printf("ws: BroadcastToRoom marshal player event for %s: %v", playerID, err)
				continue
			}
			if err := h.rdb.Publish(context.Background(), playerChannelKey(playerID), data).Err(); err != nil {
				log.Printf("ws: BroadcastToRoom redis publish player %s: %v", playerID, err)
			}
		}
		spectatorData, err := json.Marshal(spectatorEvent)
		if err != nil {
			log.Printf("ws: BroadcastToRoom marshal spectator event: %v", err)
			return
		}
		spectatorEnv, err := json.Marshal(filteredEnvelope{S: true, Data: spectatorData})
		if err != nil {
			log.Printf("ws: BroadcastToRoom marshal spectator envelope: %v", err)
			return
		}
		if err := h.rdb.Publish(context.Background(), channelKey(roomID), spectatorEnv).Err(); err != nil {
			log.Printf("ws: BroadcastToRoom redis publish spectator: %v", err)
		}
		return
	}

	// Single-instance mode: iterate local clients directly.
	h.mu.RLock()
	clients := h.rooms[roomID]
	h.mu.RUnlock()

	spectatorData, err := json.Marshal(spectatorEvent)
	if err != nil {
		log.Printf("ws: BroadcastToRoom marshal spectator event: %v", err)
		return
	}

	blocked := spectatorBlocklist[spectatorEvent.Type]

	for c := range clients {
		if c.spectator {
			if blocked {
				continue
			}
			select {
			case c.send <- spectatorData:
			default:
				log.Printf("ws: BroadcastToRoom dropping slow spectator in room %s", roomID)
				close(c.send)
			}
		} else {
			event := eventFn(c.playerID)
			data, err := json.Marshal(event)
			if err != nil {
				log.Printf("ws: BroadcastToRoom marshal event for player %s: %v", c.playerID, err)
				continue
			}
			select {
			case c.send <- data:
			default:
				log.Printf("ws: BroadcastToRoom dropping slow client in room %s", roomID)
				close(c.send)
			}
		}
	}
}

// listenRoom subscribes to the Redis channel for roomID and fans out messages
// to local clients. Messages are wrapped in filteredEnvelope to support routing.
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

		var header struct {
			Type EventType `json:"type"`
		}
		_ = json.Unmarshal(env.Data, &header)

		if env.S {
			h.fanoutSpectatorsOnly(roomID, header.Type, env.Data)
		} else {
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
// If playerData is nil, non-spectator clients are skipped.
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
