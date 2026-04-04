// Package consumer subscribes to Redis events that require ws-gateway action.
//
// Subscribed channels:
//
//   - player.session.revoked  → disconnect the player's WebSocket connection
//   - admin.broadcast.sent    → fan out broadcast message to all connected clients
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	sharedevents "github.com/recess/shared/events"
	sharedws "github.com/recess/shared/ws"
)

const (
	channelSessionRevoked  = "player.session.revoked"
	channelBroadcastSent   = "admin.broadcast.sent"
)

// Disconnector is the interface used to close player WS connections.
type Disconnector interface {
	DisconnectPlayer(playerID uuid.UUID)
}

// Broadcaster is the interface used to send messages to all connected clients.
type Broadcaster interface {
	BroadcastAll(data []byte)
}

// Consumer subscribes to event channels and acts on auth-related events.
type Consumer struct {
	rdb         *redis.Client
	hub         Disconnector
	broadcaster Broadcaster
	log         *slog.Logger
}

func New(rdb *redis.Client, hub Disconnector, broadcaster Broadcaster, log *slog.Logger) *Consumer {
	return &Consumer{rdb: rdb, hub: hub, broadcaster: broadcaster, log: log}
}

// Run blocks until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	sub := c.rdb.Subscribe(ctx, channelSessionRevoked, channelBroadcastSent)
	defer sub.Close()

	ch := sub.Channel()
	c.log.Info("subscribed to Redis channels", "channels", []string{channelSessionRevoked, channelBroadcastSent})

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
	case channelBroadcastSent:
		return c.handleBroadcastSent(msg.Payload)
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

func (c *Consumer) handleBroadcastSent(payload string) error {
	var evt sharedevents.AdminBroadcastSent
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		return fmt.Errorf("unmarshal AdminBroadcastSent: %w", err)
	}

	wsEvent := sharedws.Event{
		Type: sharedws.EventBroadcast,
		Payload: map[string]string{
			"message":        evt.Message,
			"broadcast_type": evt.BroadcastType,
		},
	}
	data, err := json.Marshal(wsEvent)
	if err != nil {
		return fmt.Errorf("marshal broadcast ws event: %w", err)
	}

	c.broadcaster.BroadcastAll(data)
	c.log.Info("ws: broadcast sent to all clients",
		"broadcast_type", evt.BroadcastType,
		"sent_by", evt.SentBy,
		"event_id", evt.EventID,
	)
	return nil
}
