// Package consumer subscribes to Redis events that require ws-gateway action.
//
// Subscribed channels:
//
//   - player.session.revoked  → disconnect the player's WebSocket connection
//   - admin.broadcast.sent    → fan out broadcast message to all connected clients
//
// # Why Pub/Sub here (and not Streams or Asynq)
//
// ws-gateway is a stateful service: each replica owns the WebSocket
// connections it accepted. Both events above need to reach EVERY replica:
//
//   - session.revoked: only the replica hosting the target connection can
//     close it. With one Asynq queue, only one replica sees the task — and
//     it might not be the one hosting the connection. Pub/Sub fans out to
//     all replicas; the right one acts, the rest no-op (DisconnectPlayer
//     of an absent player is a no-op).
//
//   - broadcast.sent: each replica must broadcast to ITS clients. With one
//     Asynq queue, only one replica fires → half the user base never sees
//     the message. Pub/Sub fan-out is the correct semantic.
//
// This is the inverse of the rdb.Subscribe-blocks-scaling problem the rest
// of P3.6 fixed: there, fan-out duplicated DB writes; here, fan-out is
// the goal.
//
// # No global dedupe
//
// A previous version of this consumer used a Redis SETNX dedupe keyed by
// event_id. With multiple replicas, that broke fan-out: the first replica
// to receive any event marked it "seen" and the others skipped — so the
// pod hosting the target connection might silently drop the disconnect,
// and only one pod would broadcast (instead of all). The dedupe was
// removed in P3.6 Phase 3c. The handlers below are naturally idempotent
// (DisconnectPlayer is a no-op for an absent player; duplicate broadcasts
// are noisy but not destructive), and Pub/Sub itself is at-most-once, so
// duplicates require an unusual publisher-side bug to even occur.
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
	channelSessionRevoked = "player.session.revoked"
	channelBroadcastSent  = "admin.broadcast.sent"
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

func (c *Consumer) handleBroadcastSent(_ context.Context, payload string) error {
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
