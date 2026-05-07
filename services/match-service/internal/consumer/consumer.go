// Package consumer reads player.banned from a Redis Stream and removes
// the banned player from the matchmaking queue.
//
// Stream: player.banned. Consumer group: "match-service".
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
	"github.com/recess/shared/streams"
)

const (
	streamPlayerBanned = "player.banned"
	consumerGroup      = "match-service"
	dedupeKeyPrefix    = "dedup:match-service:"
	dedupeTTL          = 24 * time.Hour
)

// Dequeuer is the minimal queue interface the consumer needs.
type Dequeuer interface {
	Dequeue(ctx context.Context, playerID uuid.UUID) (bool, error)
}

// Consumer reads player.banned events and removes the player from the queue.
type Consumer struct {
	rdb    *redis.Client
	worker *streams.Worker
	queue  Dequeuer
	log    *slog.Logger
}

func New(rdb *redis.Client, queue Dequeuer, log *slog.Logger, consumerName string) *Consumer {
	c := &Consumer{rdb: rdb, queue: queue, log: log}
	c.worker = streams.NewWorker(
		rdb,
		streamPlayerBanned,
		consumerGroup,
		consumerName,
		c.handle,
		streams.WithLogger(log),
	)
	return c
}

// Run blocks until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	c.log.Info("match consumer starting", "stream", streamPlayerBanned, "group", consumerGroup)
	return c.worker.Run(ctx)
}

func (c *Consumer) handle(ctx context.Context, payload []byte) error {
	var evt sharedevents.PlayerBanned
	if err := json.Unmarshal(payload, &evt); err != nil {
		return fmt.Errorf("unmarshal PlayerBanned: %w", err)
	}

	playerID, err := uuid.Parse(evt.PlayerID)
	if err != nil {
		return fmt.Errorf("invalid player_id: %w", err)
	}

	if seen, err := c.seenEvent(ctx, evt.EventID); err != nil {
		c.log.Warn("match: dedupe check failed, processing event", "event_id", evt.EventID, "error", err)
	} else if seen {
		c.log.Info("match: skipping duplicate player.banned event", "event_id", evt.EventID, "player_id", playerID)
		return nil
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

// seenEvent returns true if the event was already processed within the 24h
// dedupe window; otherwise records the event and returns false. Defense in
// depth on top of Streams' at-least-once semantics.
func (c *Consumer) seenEvent(ctx context.Context, eventID string) (bool, error) {
	if eventID == "" || c.rdb == nil {
		return false, nil
	}
	ok, err := c.rdb.SetNX(ctx, dedupeKeyPrefix+eventID, "1", dedupeTTL).Result() //nolint:staticcheck // SetNX is the idiomatic atomic check-and-set; SET ... NX requires building SetArgs which adds noise without functional benefit
	if err != nil {
		// Defensive: redis.Nil shouldn't happen on SetNX but handle it just in case.
		if errors.Is(err, redis.Nil) {
			return true, nil
		}
		return false, err
	}
	return !ok, nil
}
