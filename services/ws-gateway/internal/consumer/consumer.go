// Package consumer subscribes to Redis events that require ws-gateway action.
//
// Subscribed channels:
//
//   - player.session.revoked → disconnect the player's WebSocket connection
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	sharedevents "github.com/tableforge/shared/events"
)

const channelSessionRevoked = "player.session.revoked"

// Disconnector is the interface used to close player WS connections.
type Disconnector interface {
	DisconnectPlayer(playerID uuid.UUID)
}

// Consumer subscribes to event channels and acts on auth-related events.
type Consumer struct {
	rdb  *redis.Client
	hub  Disconnector
	log  *slog.Logger
}

func New(rdb *redis.Client, hub Disconnector, log *slog.Logger) *Consumer {
	return &Consumer{rdb: rdb, hub: hub, log: log}
}

// Run blocks until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	sub := c.rdb.Subscribe(ctx, channelSessionRevoked)
	defer sub.Close()

	ch := sub.Channel()
	c.log.Info("subscribed to Redis channels", "channels", []string{channelSessionRevoked})

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return errors.New("redis subscription channel closed unexpectedly")
			}
			if err := c.handle(ctx, msg); err != nil {
				c.log.Error("failed to handle event", "channel", msg.Channel, "error", err)
			}
		}
	}
}

func (c *Consumer) handle(ctx context.Context, msg *redis.Message) error {
	switch msg.Channel {
	case channelSessionRevoked:
		return c.handleSessionRevoked(ctx, msg.Payload)
	default:
		return fmt.Errorf("unknown channel: %s", msg.Channel)
	}
}

func (c *Consumer) handleSessionRevoked(_ context.Context, payload string) error {
	var evt sharedevents.PlayerSessionRevoked
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		return fmt.Errorf("unmarshal PlayerSessionRevoked: %w", err)
	}

	playerID, err := uuid.Parse(evt.PlayerID)
	if err != nil {
		return fmt.Errorf("invalid player_id: %w", err)
	}

	c.hub.DisconnectPlayer(playerID)
	c.log.Info("ws: disconnected player on session revoked",
		"player_id", playerID,
		"reason", evt.Reason,
		"event_id", evt.EventID,
	)
	return nil
}
