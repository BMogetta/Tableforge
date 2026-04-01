// Package consumer subscribes to Redis events that require match-service action.
//
// Subscribed channels:
//
//   - player.banned → remove the player from the matchmaking queue if present
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
)

const channelPlayerBanned = "player.banned"

// Dequeuer is the minimal queue interface the consumer needs.
type Dequeuer interface {
	Dequeue(ctx context.Context, playerID uuid.UUID) (bool, error)
}

// Consumer subscribes to event channels and acts on match-relevant events.
type Consumer struct {
	rdb   *redis.Client
	queue Dequeuer
	log   *slog.Logger
}

func New(rdb *redis.Client, queue Dequeuer, log *slog.Logger) *Consumer {
	return &Consumer{rdb: rdb, queue: queue, log: log}
}

// Run blocks until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	sub := c.rdb.Subscribe(ctx, channelPlayerBanned)
	defer sub.Close()

	ch := sub.Channel()
	c.log.Info("subscribed to Redis channels", "channels", []string{channelPlayerBanned})

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
	case channelPlayerBanned:
		return c.handlePlayerBanned(ctx, msg.Payload)
	default:
		return fmt.Errorf("unknown channel: %s", msg.Channel)
	}
}

func (c *Consumer) handlePlayerBanned(ctx context.Context, payload string) error {
	var evt sharedevents.PlayerBanned
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		return fmt.Errorf("unmarshal PlayerBanned: %w", err)
	}

	playerID, err := uuid.Parse(evt.PlayerID)
	if err != nil {
		return fmt.Errorf("invalid player_id: %w", err)
	}

	removed, err := c.queue.Dequeue(ctx, playerID)
	if err != nil {
		return fmt.Errorf("dequeue banned player: %w", err)
	}

	if removed {
		c.log.Info("match: dequeued banned player", "player_id", playerID, "event_id", evt.EventID)
	}
	return nil
}
