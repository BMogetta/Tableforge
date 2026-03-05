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
)

// spectatorOnlyEvents are never sent to spectators.
// Currently only rematch_vote — spectators cannot vote and don't need the tally.
var spectatorBlocklist = map[EventType]bool{
	EventRematchVote: true,
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

	rdb *redis.Client // nil → single-instance mode (original behaviour)
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[uuid.UUID]map[*Client]struct{})}
}

// NewHubWithRedis returns a Hub that uses Redis pub/sub for cross-instance fanout.
// Call Run(ctx) to start the subscription listener.
func NewHubWithRedis(rdb *redis.Client) *Hub {
	return &Hub{
		rooms: make(map[uuid.UUID]map[*Client]struct{}),
		rdb:   rdb,
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
}

func (h *Hub) unsubscribe(roomID uuid.UUID, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms[roomID], c)
	if len(h.rooms[roomID]) == 0 {
		delete(h.rooms, roomID)
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
