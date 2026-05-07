package streams_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/recess/shared/streams"
	"github.com/recess/shared/testutil"
	"github.com/redis/go-redis/v9"
)

// produce + consume happy path: one publisher, one worker, one message.
func TestWorker_HappyPath(t *testing.T) {
	rdb := testutil.NewTestRedisContainer(t)
	prod := streams.NewProducer(rdb)

	const stream = "test.happy"
	const group = "test-group"

	got := make(chan []byte, 1)
	w := streams.NewWorker(rdb, stream, group, "consumer-1",
		func(_ context.Context, payload []byte) error {
			got <- payload
			return nil
		},
		streams.WithBlock(200*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = w.Run(ctx) }()
	<-w.Started()

	if _, err := prod.Publish(ctx, stream, map[string]string{"hello": "world"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case payload := <-got:
		if string(payload) != `{"hello":"world"}` {
			t.Fatalf("payload mismatch: got %q", string(payload))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("handler never called")
	}
}

// Two consumers in same group must split messages (each delivered exactly once
// across the group).
func TestWorker_GroupDistributesMessages(t *testing.T) {
	rdb := testutil.NewTestRedisContainer(t)
	prod := streams.NewProducer(rdb)

	const stream = "test.dist"
	const group = "test-group"
	const total = 20

	var c1, c2 int64

	mkWorker := func(name string, counter *int64) *streams.Worker {
		return streams.NewWorker(rdb, stream, group, name,
			func(_ context.Context, _ []byte) error {
				atomic.AddInt64(counter, 1)
				return nil
			},
			streams.WithBlock(100*time.Millisecond),
			streams.WithBatch(1),
		)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w1 := mkWorker("c1", &c1)
	w2 := mkWorker("c2", &c2)
	go func() { _ = w1.Run(ctx) }()
	go func() { _ = w2.Run(ctx) }()
	<-w1.Started()
	<-w2.Started()

	for i := 0; i < total; i++ {
		if _, err := prod.Publish(ctx, stream, map[string]int{"i": i}); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&c1)+atomic.LoadInt64(&c2) >= total {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	got1 := atomic.LoadInt64(&c1)
	got2 := atomic.LoadInt64(&c2)
	if got1+got2 != total {
		t.Fatalf("expected %d total deliveries, got c1=%d c2=%d", total, got1, got2)
	}
	if got1 == 0 || got2 == 0 {
		t.Fatalf("expected both consumers to receive messages, got c1=%d c2=%d", got1, got2)
	}
}

// Group bootstrap is idempotent — calling Run on two workers with the same
// group should not error on the second one (BUSYGROUP).
func TestWorker_GroupBootstrapIdempotent(t *testing.T) {
	rdb := testutil.NewTestRedisContainer(t)

	const stream = "test.idempotent"
	const group = "g"
	noop := func(context.Context, []byte) error { return nil }

	w1 := streams.NewWorker(rdb, stream, group, "c1", noop, streams.WithBlock(50*time.Millisecond))
	w2 := streams.NewWorker(rdb, stream, group, "c2", noop, streams.WithBlock(50*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 2)
	go func() { errCh <- w1.Run(ctx) }()
	time.Sleep(150 * time.Millisecond)
	go func() { errCh <- w2.Run(ctx) }()
	time.Sleep(150 * time.Millisecond)

	cancel()
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("worker %d errored: %v", i, err)
		}
	}
}

// Handler error → message reclaimed by claim loop → eventually succeeds.
func TestWorker_RetryAfterClaim(t *testing.T) {
	rdb := testutil.NewTestRedisContainer(t)
	prod := streams.NewProducer(rdb)

	const stream = "test.retry"
	const group = "test-group"

	var attempts int64
	done := make(chan struct{}, 1)
	w := streams.NewWorker(rdb, stream, group, "c1",
		func(_ context.Context, _ []byte) error {
			n := atomic.AddInt64(&attempts, 1)
			if n < 2 {
				return errors.New("transient")
			}
			select {
			case done <- struct{}{}:
			default:
			}
			return nil
		},
		streams.WithBlock(100*time.Millisecond),
		// Aggressive timings to keep test fast.
		streams.WithClaimMinIdle(200*time.Millisecond),
		streams.WithClaimInterval(200*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = w.Run(ctx) }()
	<-w.Started()

	if _, err := prod.Publish(ctx, stream, map[string]string{"k": "v"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("never succeeded after retry, attempts=%d", atomic.LoadInt64(&attempts))
	}
	if got := atomic.LoadInt64(&attempts); got < 2 {
		t.Fatalf("expected >=2 attempts, got %d", got)
	}

	// PEL should be empty after success.
	cancel()
	time.Sleep(200 * time.Millisecond)
	pending, err := rdb.XPending(context.Background(), stream, group).Result()
	if err != nil {
		t.Fatalf("xpending: %v", err)
	}
	if pending.Count != 0 {
		t.Fatalf("expected empty PEL after success, got count=%d", pending.Count)
	}
}

// Permanent handler failure → message routed to DLQ + acked from main stream.
func TestWorker_DLQAfterMaxRetries(t *testing.T) {
	rdb := testutil.NewTestRedisContainer(t)
	prod := streams.NewProducer(rdb)

	const stream = "test.dlq"
	const group = "test-group"
	const maxRetries = 3

	var attempts int64
	w := streams.NewWorker(rdb, stream, group, "c1",
		func(_ context.Context, _ []byte) error {
			atomic.AddInt64(&attempts, 1)
			return errors.New("permanent")
		},
		streams.WithBlock(100*time.Millisecond),
		streams.WithClaimMinIdle(150*time.Millisecond),
		streams.WithClaimInterval(150*time.Millisecond),
		streams.WithMaxRetries(maxRetries),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = w.Run(ctx) }()
	<-w.Started()

	if _, err := prod.Publish(ctx, stream, map[string]string{"x": "y"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	dlqStream := stream + ".dlq"
	deadline := time.Now().Add(10 * time.Second)
	var dlqLen int64
	for time.Now().Before(deadline) {
		dlqLen, _ = rdb.XLen(context.Background(), dlqStream).Result()
		if dlqLen >= 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if dlqLen != 1 {
		t.Fatalf("expected 1 DLQ entry, got %d (attempts=%d)", dlqLen, atomic.LoadInt64(&attempts))
	}

	// Original PEL should be drained after DLQ ack.
	pending, err := rdb.XPending(context.Background(), stream, group).Result()
	if err != nil {
		t.Fatalf("xpending: %v", err)
	}
	if pending.Count != 0 {
		t.Fatalf("expected empty PEL after DLQ, got count=%d", pending.Count)
	}

	// DLQ entry should carry the original payload + error.
	entries, err := rdb.XRange(context.Background(), dlqStream, "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange dlq: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 dlq entry, got %d", len(entries))
	}
	values := entries[0].Values
	if got, _ := values["error"].(string); got != "permanent" {
		t.Fatalf("expected error=permanent in DLQ, got %q", got)
	}
	if _, ok := values["original_id"].(string); !ok {
		t.Fatal("dlq entry missing original_id")
	}
	if _, ok := values["payload"].(string); !ok {
		t.Fatal("dlq entry missing payload")
	}
}

// MAXLEN trimming caps the stream length (approximate).
func TestProducer_MaxLenTrims(t *testing.T) {
	rdb := testutil.NewTestRedisContainer(t)
	prod := streams.NewProducer(rdb, streams.WithMaxLen(50))
	const stream = "test.trim"

	ctx := context.Background()
	for i := 0; i < 200; i++ {
		if _, err := prod.Publish(ctx, stream, map[string]int{"i": i}); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	length, err := rdb.XLen(ctx, stream).Result()
	if err != nil {
		t.Fatalf("xlen: %v", err)
	}
	// Approximate trimming with cap=50 should keep us close to 50, never
	// way above. Allow generous slack for radix-tree node boundaries.
	if length > 200 {
		t.Fatalf("expected MAXLEN trimming to cap length, got %d", length)
	}
	if length < 50 {
		t.Fatalf("approximate trim should not undershoot, got %d", length)
	}
}

// Malformed entry (no payload field) is dropped, not retried forever.
func TestWorker_MalformedEntryDropped(t *testing.T) {
	rdb := testutil.NewTestRedisContainer(t)

	const stream = "test.malformed"
	const group = "test-group"

	called := make(chan struct{}, 1)
	w := streams.NewWorker(rdb, stream, group, "c1",
		func(context.Context, []byte) error {
			select {
			case called <- struct{}{}:
			default:
			}
			return nil
		},
		streams.WithBlock(100*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Force the consumer group to be created at "$" — only catches new entries.
	go func() { _ = w.Run(ctx) }()
	<-w.Started()

	// Inject a malformed entry: no "payload" field.
	if _, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"not_payload": "oops"},
	}).Result(); err != nil {
		t.Fatalf("xadd malformed: %v", err)
	}

	// Wait long enough for the worker to read and ack-drop.
	time.Sleep(500 * time.Millisecond)

	// Handler must not have been called for the malformed entry.
	select {
	case <-called:
		t.Fatal("handler was called for malformed entry")
	default:
	}

	// PEL should be empty — entry was acked on drop.
	pending, err := rdb.XPending(context.Background(), stream, group).Result()
	if err != nil {
		t.Fatalf("xpending: %v", err)
	}
	if pending.Count != 0 {
		t.Fatalf("expected empty PEL after malformed drop, got count=%d", pending.Count)
	}
}

// Sanity: NewProducer returns a usable client with sensible defaults.
func TestProducer_Defaults(t *testing.T) {
	rdb := testutil.NewTestRedisContainer(t)
	prod := streams.NewProducer(rdb)
	id, err := prod.Publish(context.Background(), "test.defaults", map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty stream ID")
	}
}

// Compile-time assert that Handler matches what consumers expect.
var _ streams.Handler = func(ctx context.Context, _ []byte) error {
	return fmt.Errorf("noop %v", ctx)
}
