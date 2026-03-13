package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
)

// playerChannelKey returns the Redis pub/sub channel name for a player.
// Used for dm_received and dm_read events delivered to a specific player
// regardless of which room they are currently connected to.
func playerChannelKey(playerID uuid.UUID) string {
	return fmt.Sprintf("ws:player:%s", playerID)
}

// subscribePlayer registers c in the per-player index.
// Must be called after subscribe (room index) so the lock ordering is consistent.
func (h *Hub) subscribePlayer(playerID uuid.UUID, c *Client) {
	h.pmu.Lock()
	defer h.pmu.Unlock()
	if h.players[playerID] == nil {
		h.players[playerID] = make(map[*Client]struct{})
	}
	h.players[playerID][c] = struct{}{}

	if h.rdb != nil && len(h.players[playerID]) == 1 {
		go h.listenPlayer(playerID)
	}
}

// unsubscribePlayer removes c from the per-player index.
// Must be called alongside unsubscribe (room index).
func (h *Hub) unsubscribePlayer(playerID uuid.UUID, c *Client) {
	h.pmu.Lock()
	defer h.pmu.Unlock()
	delete(h.players[playerID], c)
	if len(h.players[playerID]) == 0 {
		delete(h.players, playerID)
	}
}

// BroadcastToPlayer delivers an event to all connections belonging to playerID.
// In Redis mode the message is published to the player's Redis channel so that
// any server instance holding that player's connection receives it.
// No-op if the player is not connected on this instance (or any instance in
// Redis mode, since publish handles fanout).
func (h *Hub) BroadcastToPlayer(playerID uuid.UUID, event Event) {
	if h == nil {
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("ws: marshal player event: %v", err)
		return
	}

	if h.rdb != nil {
		if err := h.rdb.Publish(context.Background(), playerChannelKey(playerID), data).Err(); err != nil {
			log.Printf("ws: redis publish player: %v", err)
		}
		return
	}

	h.fanoutPlayer(playerID, data)
}

// listenPlayer subscribes to the Redis channel for playerID and fans out
// messages to local clients. Exits when there are no more local connections
// for this player.
func (h *Hub) listenPlayer(playerID uuid.UUID) {
	sub := h.rdb.Subscribe(context.Background(), playerChannelKey(playerID))
	defer sub.Close()

	ch := sub.Channel()
	for msg := range ch {
		h.pmu.RLock()
		_, active := h.players[playerID]
		h.pmu.RUnlock()
		if !active {
			return
		}
		h.fanoutPlayer(playerID, []byte(msg.Payload))
	}
}

// fanoutPlayer delivers data to all local connections belonging to playerID.
func (h *Hub) fanoutPlayer(playerID uuid.UUID, data []byte) {
	h.pmu.RLock()
	clients := h.players[playerID]
	h.pmu.RUnlock()

	for c := range clients {
		select {
		case c.send <- data:
		default:
			log.Printf("ws: dropping slow player client %s", playerID)
			close(c.send)
		}
	}
}
