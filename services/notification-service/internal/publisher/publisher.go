// Package publisher delivers real-time WebSocket events to players via Redis pub/sub.
// The ws-gateway is the sole Redis subscriber — it fans out to connected clients.
package publisher

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	sharedws "github.com/recess/shared/ws"
)

// Publisher publishes WS events to the ws:player:{id} Redis channel.
type Publisher struct {
	rdb *redis.Client
	log *slog.Logger
}

func New(rdb *redis.Client, log *slog.Logger) *Publisher {
	return &Publisher{rdb: rdb, log: log}
}

func (p *Publisher) PublishToPlayer(ctx context.Context, playerID uuid.UUID, event sharedws.Event) {
	data, err := json.Marshal(event)
	if err != nil {
		p.log.Error("publisher: marshal event", "player_id", playerID, "error", err)
		return
	}
	if err := p.rdb.Publish(ctx, sharedws.PlayerChannelKey(playerID), data).Err(); err != nil {
		p.log.Error("publisher: redis publish", "player_id", playerID, "error", err)
	}
}
