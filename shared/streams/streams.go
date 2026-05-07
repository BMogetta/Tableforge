// Package streams is a thin layer over Redis Streams (XADD / XREADGROUP)
// for cross-service fan-out events.
//
// # Why not Pub/Sub
//
// Redis Pub/Sub is at-most-once and fans out to every subscriber, so a
// service running >1 replica receives every event N times. Streams keep
// messages around (with MAXLEN trimming), and a consumer group splits the
// stream across the group's consumers — each message is delivered to
// exactly one consumer in the group, with at-least-once semantics
// (re-delivered if not acked before the idle threshold).
//
// # Why not Asynq
//
// Asynq is a task queue: one queue, one consuming pool. Fan-out across N
// services means the publisher must enqueue N tasks (one per consumer),
// which couples publishers to the consumer set. Streams give native
// fan-out: each consumer group reads independently from the same stream.
//
// Asynq is the right tool for point-to-point + scheduled tasks; Streams
// is the right tool for fan-out events. The codebase uses both.
//
// # Delivery model
//
// At-least-once. Handlers MUST be idempotent — same message may be
// delivered more than once if the consumer crashed mid-handler or the
// network dropped before XACK. Defense in depth: keep the event_id UUID
// in payloads and persist a (consumer, event_id) row on first success.
//
// # Retries and DLQ
//
// The Worker doesn't ack on handler error. After the idle threshold, the
// claim loop reclaims the message via XAUTOCLAIM and re-runs the handler.
// Once XPENDING reports a delivery count >= MaxRetries, the message is
// XADDed to {stream}.dlq and the original is acked (so the group moves on).
//
// # Stream payload format
//
// XADD entry has a single field "payload" containing JSON-marshaled bytes
// of whatever the publisher passed to Producer.Publish. Consumers receive
// raw []byte and unmarshal into the expected event struct themselves.
package streams

import (
	"context"
	"errors"
)

// Handler processes one message from the stream. Returning a non-nil error
// causes the Worker to skip ack; the message will be reclaimed and retried
// by the claim loop. Permanent failures are routed to the DLQ after
// MaxRetries delivery attempts.
type Handler func(ctx context.Context, payload []byte) error

// ErrSkipAck can be returned by a Handler when the message should not be
// acked AND should not count toward the retry budget. Useful for transient
// shutdown scenarios — most handlers should just return a normal error
// and let the retry budget protect against runaway loops.
var ErrSkipAck = errors.New("streams: skip ack without retry charge")
