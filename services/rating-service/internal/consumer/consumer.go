package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
	"github.com/recess/shared/events"
)

const channelGameSessionFinished = "game.session.finished"

// Processor handles game session finished events.
type Processor interface {
	ProcessGameFinished(ctx context.Context, evt events.GameSessionFinished) error
}

// Consumer subscribes to game.session.finished and forwards events to a Processor.
type Consumer struct {
	rdb *redis.Client
	svc Processor
	log *slog.Logger
}

func New(rdb *redis.Client, svc Processor, log *slog.Logger) *Consumer {
	return &Consumer{rdb: rdb, svc: svc, log: log}
}

// Run blocks until ctx is cancelled. It reconnects automatically on transient errors.
func (c *Consumer) Run(ctx context.Context) error {
	sub := c.rdb.Subscribe(ctx, channelGameSessionFinished)
	defer sub.Close()

	ch := sub.Channel()
	c.log.Info("subscribed to Redis channel", "channel", channelGameSessionFinished)

	for {
		select {
		case <-ctx.Done():
			return nil

		case msg, ok := <-ch:
			if !ok {
				// Channel closed — Redis connection dropped.
				return errors.New("redis subscription channel closed unexpectedly")
			}
			if err := c.handle(ctx, msg); err != nil {
				// Log and continue: a single bad message must not kill the consumer.
				c.log.Error("failed to handle event",
					"channel", msg.Channel,
					"error", err,
				)
			}
		}
	}
}

func (c *Consumer) handle(ctx context.Context, msg *redis.Message) error {
	switch msg.Channel {
	case channelGameSessionFinished:
		return c.handleGameSessionFinished(ctx, msg.Payload)
	default:
		return fmt.Errorf("unknown channel: %s", msg.Channel)
	}
}

func (c *Consumer) handleGameSessionFinished(ctx context.Context, payload string) error {
	var evt events.GameSessionFinished
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		return fmt.Errorf("unmarshal GameSessionFinished: %w", err)
	}

	c.log.Debug("received event",
		"event_id", evt.EventID,
		"session_id", evt.SessionID,
		"mode", evt.Mode,
	)

	return c.svc.ProcessGameFinished(ctx, evt)
}
