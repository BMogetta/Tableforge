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
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	sharedevents "github.com/recess/shared/events"
	sharedws "github.com/recess/shared/ws"
)

const (
	channelSessionRevoked = "player.session.revoked"
	channelBroadcastSent  = "admin.broadcast.sent"

	dedupeKeyPrefix = "dedup:ws-gateway:"
	dedupeTTL       = 24 * time.Hour
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
		return c.handleBroadcastSent(ctx, msg.Payload)
	default:
		return fmt.Errorf("unknown channel: %s", msg.Channel)
	}
}

func (c *Consumer) handleSessionRevoked(ctx context.Context, payload string) error {
	var evt sharedevents.PlayerSessionRevoked
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		return fmt.Errorf("unmarshal PlayerSessionRevoked: %w", err)
	}

	playerID, err := uuid.Parse(evt.PlayerID)
	if err != nil {
		return fmt.Errorf("invalid player_id: %w", err)
	}

	if seen, err := c.seenEvent(ctx, evt.EventID); err != nil {
		c.log.Warn("ws: dedupe check failed, processing event", "event_id", evt.EventID, "error", err)
	} else if seen {
		c.log.Info("ws: skipping duplicate session.revoked", "event_id", evt.EventID, "player_id", playerID)
		return nil
	}

	c.hub.DisconnectPlayer(playerID)
	c.log.Info("ws: disconnected player on session revoked",
		"player_id", playerID,
		"reason", evt.Reason,
		"event_id", evt.EventID,
	)
	return nil
}

func (c *Consumer) handleBroadcastSent(ctx context.Context, payload string) error {
	var evt sharedevents.AdminBroadcastSent
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		return fmt.Errorf("unmarshal AdminBroadcastSent: %w", err)
	}

	if seen, err := c.seenEvent(ctx, evt.EventID); err != nil {
		c.log.Warn("ws: dedupe check failed, processing event", "event_id", evt.EventID, "error", err)
	} else if seen {
		c.log.Info("ws: skipping duplicate admin.broadcast.sent", "event_id", evt.EventID)
		return nil
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

// seenEvent returns true if the event was already processed within the 24h
// dedupe window; otherwise records the event and returns false. An empty
// event_id or nil Redis client is treated as unseen so invocations without a
// wired Redis (unit tests, bootstrap) still proceed. Redis errors fail open.
func (c *Consumer) seenEvent(ctx context.Context, eventID string) (bool, error) {
	if eventID == "" || c.rdb == nil {
		return false, nil
	}
	ok, err := c.rdb.SetNX(ctx, dedupeKeyPrefix+eventID, "1", dedupeTTL).Result() //nolint:staticcheck // SetNX is the idiomatic atomic primitive; SET ... NX would add noise without functional benefit
	if err != nil {
		return false, err
	}
	return !ok, nil
}
