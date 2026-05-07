// Package consumer reads game.session.finished from a Redis Stream and
// forwards each event to a Processor.
//
// Stream: game.session.finished. Consumer group: "rating-service" — one
// per service, so multiple replicas split the stream cleanly.
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"

	"github.com/recess/shared/events"
	"github.com/recess/shared/streams"
)

const (
	streamGameSessionFinished = "game.session.finished"
	consumerGroup             = "rating-service"
)

// Processor handles game session finished events.
type Processor interface {
	ProcessGameFinished(ctx context.Context, evt events.GameSessionFinished) error
}

// Consumer wraps a streams.Worker scoped to game.session.finished + the
// rating-service consumer group.
type Consumer struct {
	worker *streams.Worker
	svc    Processor
	log    *slog.Logger
}

// New constructs a Consumer. consumerName is the per-instance identity
// (typically the pod hostname) used by XAUTOCLAIM to tell replicas apart.
func New(rdb *redis.Client, svc Processor, log *slog.Logger, consumerName string) *Consumer {
	c := &Consumer{svc: svc, log: log}
	c.worker = streams.NewWorker(
		rdb,
		streamGameSessionFinished,
		consumerGroup,
		consumerName,
		c.handle,
		streams.WithLogger(log),
	)
	return c
}

// Run blocks until ctx is cancelled. The underlying worker bootstraps the
// consumer group idempotently and runs both consume + claim loops.
func (c *Consumer) Run(ctx context.Context) error {
	c.log.Info("rating consumer starting", "stream", streamGameSessionFinished, "group", consumerGroup)
	return c.worker.Run(ctx)
}

func (c *Consumer) handle(ctx context.Context, payload []byte) error {
	var evt events.GameSessionFinished
	if err := json.Unmarshal(payload, &evt); err != nil {
		return fmt.Errorf("unmarshal GameSessionFinished: %w", err)
	}

	c.log.Debug("received event",
		"event_id", evt.EventID,
		"session_id", evt.SessionID,
		"mode", evt.Mode,
	)

	return c.svc.ProcessGameFinished(ctx, evt)
}
