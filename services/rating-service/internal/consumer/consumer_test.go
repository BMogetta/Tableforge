package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/recess/shared/events"
	"github.com/recess/shared/streams"
	"github.com/recess/shared/testutil"
)

type mockProcessor struct {
	calls atomic.Int64
	last  events.GameSessionFinished
	err   error
}

func (m *mockProcessor) ProcessGameFinished(_ context.Context, evt events.GameSessionFinished) error {
	m.calls.Add(1)
	m.last = evt
	return m.err
}

func newHandlerOnly(p *mockProcessor) *Consumer {
	return &Consumer{svc: p, log: slog.Default()}
}

func sessionFinished(sessionID, gameID, mode string) events.GameSessionFinished {
	return events.GameSessionFinished{
		SessionID: sessionID,
		GameID:    gameID,
		Mode:      mode,
		EndedBy:   "win",
		WinnerID:  uuid.NewString(),
		Players: []events.SessionPlayer{
			{PlayerID: uuid.NewString(), Seat: 0, Outcome: "win"},
			{PlayerID: uuid.NewString(), Seat: 1, Outcome: "loss"},
		},
	}
}

// Unit: handle() decodes JSON and forwards to the Processor.
func TestHandle_ValidGameSessionFinished(t *testing.T) {
	mock := &mockProcessor{}
	c := newHandlerOnly(mock)

	sessionID := uuid.NewString()
	payload, _ := json.Marshal(sessionFinished(sessionID, "tictactoe", "ranked"))

	if err := c.handle(context.Background(), payload); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if mock.calls.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", mock.calls.Load())
	}
	if mock.last.SessionID != sessionID {
		t.Errorf("expected session_id %s, got %s", sessionID, mock.last.SessionID)
	}
	if mock.last.GameID != "tictactoe" {
		t.Errorf("expected game_id tictactoe, got %s", mock.last.GameID)
	}
	if mock.last.Mode != "ranked" {
		t.Errorf("expected mode ranked, got %s", mock.last.Mode)
	}
}

// Unit: malformed JSON returns an error (worker will retry/DLQ as configured).
func TestHandle_InvalidJSON(t *testing.T) {
	c := newHandlerOnly(&mockProcessor{})
	if err := c.handle(context.Background(), []byte("not json")); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// Unit: processor errors propagate so the worker can retry.
func TestHandle_ProcessError(t *testing.T) {
	mock := &mockProcessor{err: errors.New("db error")}
	c := newHandlerOnly(mock)

	payload, _ := json.Marshal(sessionFinished(uuid.NewString(), "tictactoe", "ranked"))
	err := c.handle(context.Background(), payload)
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if !errors.Is(err, mock.err) {
		t.Errorf("expected wrapped db error, got %v", err)
	}
}

// Integration: end-to-end through the streams.Worker — Producer publishes,
// Consumer's worker reads via XREADGROUP, and the Processor receives the event.
func TestConsumer_StreamsIntegration(t *testing.T) {
	rdb := testutil.NewTestRedisContainer(t)
	prod := streams.NewProducer(rdb)

	mock := &mockProcessor{}
	c := New(rdb, mock, slog.Default(), "test-consumer")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Run(ctx) }()
	<-c.worker.Started()

	sessionID := uuid.NewString()
	if _, err := prod.Publish(ctx, streamGameSessionFinished, sessionFinished(sessionID, "tictactoe", "ranked")); err != nil {
		t.Fatalf("publish: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if mock.calls.Load() == 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if mock.calls.Load() != 1 {
		t.Fatalf("expected exactly 1 delivery, got %d", mock.calls.Load())
	}
	if mock.last.SessionID != sessionID {
		t.Errorf("session_id mismatch: got %s want %s", mock.last.SessionID, sessionID)
	}
}

// Integration: two replicas in the same group split the load — each event is
// delivered to exactly one consumer in the group.
func TestConsumer_GroupDistributesAcrossReplicas(t *testing.T) {
	rdb := testutil.NewTestRedisContainer(t)
	prod := streams.NewProducer(rdb)

	m1 := &mockProcessor{}
	m2 := &mockProcessor{}
	c1 := New(rdb, m1, slog.Default(), "replica-1")
	c2 := New(rdb, m2, slog.Default(), "replica-2")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c1.Run(ctx) }()
	go func() { _ = c2.Run(ctx) }()
	<-c1.worker.Started()
	<-c2.worker.Started()

	const total = 20
	for i := 0; i < total; i++ {
		if _, err := prod.Publish(ctx, streamGameSessionFinished, sessionFinished(uuid.NewString(), "tictactoe", "ranked")); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if m1.calls.Load()+m2.calls.Load() >= total {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	got1, got2 := m1.calls.Load(), m2.calls.Load()
	if got1+got2 != total {
		t.Fatalf("expected %d total calls, got c1=%d c2=%d (sum=%d)", total, got1, got2, got1+got2)
	}
	if got1 == 0 || got2 == 0 {
		t.Fatalf("expected both replicas to receive messages, got c1=%d c2=%d", got1, got2)
	}
}
