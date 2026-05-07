package streams

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const defaultMaxLen int64 = 10000

// Producer publishes JSON-marshaled payloads to a Redis Stream.
//
// MaxLen uses approximate trimming (~) — Redis is allowed to keep slightly
// more entries than the cap when it lets the radix-tree node split happen
// at a natural boundary. Performance gain is significant; the overshoot
// is bounded.
type Producer struct {
	rdb    *redis.Client
	maxLen int64
}

// ProducerOption configures a Producer.
type ProducerOption func(*Producer)

// WithMaxLen overrides the per-stream MAXLEN cap (default 10000).
func WithMaxLen(n int64) ProducerOption {
	return func(p *Producer) { p.maxLen = n }
}

// NewProducer wires a Producer to the given Redis client.
func NewProducer(rdb *redis.Client, opts ...ProducerOption) *Producer {
	p := &Producer{rdb: rdb, maxLen: defaultMaxLen}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Publish JSON-marshals payload and XADDs it to stream. Returns the
// assigned stream ID (e.g. "1717254000000-0").
func (p *Producer) Publish(ctx context.Context, stream string, payload any) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}
	id, err := p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		MaxLen: p.maxLen,
		Approx: true,
		Values: map[string]any{"payload": data},
	}).Result()
	if err != nil {
		return "", fmt.Errorf("xadd %s: %w", stream, err)
	}
	return id, nil
}
