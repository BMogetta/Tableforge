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
	EventMoveApplied    EventType = "move_applied"
	EventGameOver       EventType = "game_over"
	EventPlayerJoined   EventType = "player_joined"
	EventPlayerLeft     EventType = "player_left"
	EventOwnerChanged   EventType = "owner_changed"
	EventRoomClosed     EventType = "room_closed"
	EventGameStarted    EventType = "game_started"
	EventRematchVote    EventType = "rematch_vote"
	EventRematchStarted EventType = "rematch_started"
)

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

	h.fanout(roomID, data)
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
		h.fanout(roomID, []byte(msg.Payload))
	}
}

// fanout delivers data to all local clients in a room.
func (h *Hub) fanout(roomID uuid.UUID, data []byte) {
	h.mu.RLock()
	clients := h.rooms[roomID]
	h.mu.RUnlock()

	for c := range clients {
		select {
		case c.send <- data:
		default:
			log.Printf("ws: dropping slow client in room %s", roomID)
			close(c.send)
		}
	}
}
