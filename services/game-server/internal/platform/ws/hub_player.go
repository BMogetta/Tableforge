package ws

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	sharedws "github.com/recess/shared/ws"
)

// BroadcastToPlayer publishes an event to a player's personal Redis channel.
// ws-gateway consumes it and delivers to the player's WebSocket connection.
func (h *Hub) BroadcastToPlayer(playerID uuid.UUID, event Event) {
	if h == nil || h.rdb == nil {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("ws: marshal player event", "player_id", playerID, "error", err)
		return
	}
	if err := h.rdb.Publish(context.Background(), sharedws.PlayerChannelKey(playerID), data).Err(); err != nil {
		slog.Error("ws: redis publish player", "player_id", playerID, "error", err)
	}
}
