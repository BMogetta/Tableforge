package streams

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultBlock         = 5 * time.Second
	defaultBatch   int64 = 10
	defaultMaxRetries    = 5
	defaultClaimMinIdle  = 30 * time.Second
	defaultClaimInterval = 15 * time.Second
)

// Worker reads from a single stream within a single consumer group, runs
// Handler on each message, and acks on success. Failures are reclaimed
// and retried via XAUTOCLAIM; messages that hit MaxRetries deliveries
// are routed to the DLQ stream and acked.
//
// Run two goroutines concurrently: the consume loop (XREADGROUP for new
// messages) and the claim loop (XAUTOCLAIM for stuck messages from
// crashed peers). Both feed the same processMessage path.
type Worker struct {
	rdb       *redis.Client
	stream    string
	group     string
	consumer  string
	handler   Handler
	dlqStream string
	log       *slog.Logger

	block         time.Duration
	batch         int64
	maxRetries    int
	claimMinIdle  time.Duration
	claimInterval time.Duration

	started chan struct{}
}

// WorkerOption configures a Worker.
type WorkerOption func(*Worker)

// WithBlock sets the XREADGROUP block timeout (default 5s).
func WithBlock(d time.Duration) WorkerOption {
	return func(w *Worker) { w.block = d }
}

// WithBatch sets the XREADGROUP COUNT (default 10).
func WithBatch(n int64) WorkerOption {
	return func(w *Worker) { w.batch = n }
}

// WithMaxRetries sets the delivery-count threshold for DLQ routing
// (default 5). Once XPENDING reports delivery count >= this value after
// a handler error, the message is moved to the DLQ stream and acked.
func WithMaxRetries(n int) WorkerOption {
	return func(w *Worker) { w.maxRetries = n }
}

// WithClaimMinIdle sets how long a message must be unacked before the
// claim loop reclaims it (default 30s).
func WithClaimMinIdle(d time.Duration) WorkerOption {
	return func(w *Worker) { w.claimMinIdle = d }
}

// WithClaimInterval sets how often the claim loop runs (default 15s).
func WithClaimInterval(d time.Duration) WorkerOption {
	return func(w *Worker) { w.claimInterval = d }
}

// WithDLQStream overrides the DLQ stream name (default: "{stream}.dlq").
func WithDLQStream(s string) WorkerOption {
	return func(w *Worker) { w.dlqStream = s }
}

// WithLogger plugs in a structured logger (default: slog.Default).
func WithLogger(l *slog.Logger) WorkerOption {
	return func(w *Worker) { w.log = l }
}

// NewWorker wires a Worker. Use Run to start it.
//
// stream is the stream name (e.g. "game.session.finished"). group is the
// consumer group name (one per service, e.g. "rating-service"). consumer
// is the per-instance identity (typically pod hostname) so XAUTOCLAIM
// can tell consumers apart.
func NewWorker(rdb *redis.Client, stream, group, consumer string, h Handler, opts ...WorkerOption) *Worker {
	w := &Worker{
		rdb:           rdb,
		stream:        stream,
		group:         group,
		consumer:      consumer,
		handler:       h,
		dlqStream:     stream + ".dlq",
		log:           slog.Default(),
		block:         defaultBlock,
		batch:         defaultBatch,
		maxRetries:    defaultMaxRetries,
		claimMinIdle:  defaultClaimMinIdle,
		claimInterval: defaultClaimInterval,
		started:       make(chan struct{}),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Run blocks until ctx is cancelled. Idempotently creates the consumer
// group on first call (handles BUSYGROUP). Errors from the loops are
// logged but do not stop the worker — the loops self-heal on transient
// Redis failures.
//
// The group is created with last-delivered-id "$", meaning the consumer
// only sees messages published after the group exists. Use Started() to
// synchronize a publisher with worker startup if missed messages would
// be a problem.
func (w *Worker) Run(ctx context.Context) error {
	if err := w.ensureGroup(ctx); err != nil {
		return fmt.Errorf("ensure group: %w", err)
	}
	close(w.started)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); w.consumeLoop(ctx) }()
	go func() { defer wg.Done(); w.claimLoop(ctx) }()
	wg.Wait()
	return nil
}

// Started returns a channel that is closed once the consumer group has
// been created (or confirmed to exist) and the read loops are about to
// start. Useful for tests and bootstrap sequences that must avoid
// publishing before a fresh consumer is ready.
func (w *Worker) Started() <-chan struct{} { return w.started }

func (w *Worker) ensureGroup(ctx context.Context) error {
	err := w.rdb.XGroupCreateMkStream(ctx, w.stream, w.group, "$").Err()
	if err == nil {
		return nil
	}
	// BUSYGROUP means the group already exists — fine.
	if strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return err
}

func (w *Worker) consumeLoop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		streams, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    w.group,
			Consumer: w.consumer,
			Streams:  []string{w.stream, ">"},
			Count:    w.batch,
			Block:    w.block,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			w.log.Error("streams: xreadgroup",
				"stream", w.stream, "group", w.group, "error", err)
			// Backoff briefly so we don't spin on persistent Redis errors.
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}
		for _, s := range streams {
			for _, m := range s.Messages {
				w.processMessage(ctx, m)
			}
		}
	}
}

func (w *Worker) claimLoop(ctx context.Context) {
	ticker := time.NewTicker(w.claimInterval)
	defer ticker.Stop()
	nextID := "0-0"
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		msgs, newNext, err := w.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   w.stream,
			Group:    w.group,
			Consumer: w.consumer,
			MinIdle:  w.claimMinIdle,
			Start:    nextID,
			Count:    w.batch,
		}).Result()
		if err != nil {
			w.log.Error("streams: xautoclaim",
				"stream", w.stream, "group", w.group, "error", err)
			continue
		}
		nextID = newNext
		for _, m := range msgs {
			w.processMessage(ctx, m)
		}
	}
}

func (w *Worker) processMessage(ctx context.Context, m redis.XMessage) {
	payload, ok := readPayload(m)
	if !ok {
		// Malformed entry (no payload field) — ack and drop. Cannot
		// retry usefully; would loop forever otherwise.
		w.log.Warn("streams: malformed entry, dropping",
			"stream", w.stream, "id", m.ID)
		_ = w.rdb.XAck(ctx, w.stream, w.group, m.ID).Err()
		return
	}

	err := w.handler(ctx, payload)
	if err == nil {
		if ackErr := w.rdb.XAck(ctx, w.stream, w.group, m.ID).Err(); ackErr != nil {
			w.log.Error("streams: xack",
				"stream", w.stream, "id", m.ID, "error", ackErr)
		}
		return
	}

	if errors.Is(err, ErrSkipAck) {
		// Don't ack, don't count this attempt against the retry budget
		// either. Caller signals shutdown / transient skip.
		return
	}

	count, perr := w.deliveryCount(ctx, m.ID)
	if perr != nil {
		w.log.Error("streams: xpending lookup",
			"stream", w.stream, "id", m.ID, "error", perr)
		// Don't ack — let claim loop pick it up again later.
		return
	}

	if count >= int64(w.maxRetries) {
		w.toDLQ(ctx, m.ID, payload, err)
		return
	}

	w.log.Warn("streams: handler failed, will retry",
		"stream", w.stream, "id", m.ID, "delivery", count, "error", err)
}

func (w *Worker) deliveryCount(ctx context.Context, id string) (int64, error) {
	pending, err := w.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: w.stream,
		Group:  w.group,
		Start:  id,
		End:    id,
		Count:  1,
	}).Result()
	if err != nil {
		return 0, err
	}
	if len(pending) == 0 {
		return 0, nil
	}
	return pending[0].RetryCount, nil
}

func (w *Worker) toDLQ(ctx context.Context, originalID string, payload []byte, handlerErr error) {
	_, err := w.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: w.dlqStream,
		Approx: true,
		MaxLen: defaultMaxLen,
		Values: map[string]any{
			"payload":     payload,
			"original_id": originalID,
			"error":       handlerErr.Error(),
			"failed_at":   time.Now().UTC().Format(time.RFC3339Nano),
		},
	}).Result()
	if err != nil {
		w.log.Error("streams: xadd dlq",
			"dlq", w.dlqStream, "id", originalID, "error", err)
		// Don't ack — try again on next reclaim. Better duplicate DLQ
		// entries than losing a message entirely.
		return
	}
	if ackErr := w.rdb.XAck(ctx, w.stream, w.group, originalID).Err(); ackErr != nil {
		w.log.Error("streams: xack after dlq",
			"stream", w.stream, "id", originalID, "error", ackErr)
		return
	}
	w.log.Error("streams: routed to DLQ after max retries",
		"stream", w.stream, "dlq", w.dlqStream, "id", originalID, "error", handlerErr)
}

func readPayload(m redis.XMessage) ([]byte, bool) {
	v, ok := m.Values["payload"]
	if !ok {
		return nil, false
	}
	switch p := v.(type) {
	case string:
		return []byte(p), true
	case []byte:
		return p, true
	default:
		return nil, false
	}
}
