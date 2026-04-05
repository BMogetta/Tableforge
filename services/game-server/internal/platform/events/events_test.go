package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/game-server/internal/testutil"
)

func newTestStore(t *testing.T) (*Store, *redis.Client, *miniredis.Miniredis, *testutil.FakeStore) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	fs := testutil.NewFakeStore()
	s := New(rdb, fs)
	return s, rdb, mr, fs
}

// --- Append tests ------------------------------------------------------------

func TestAppend_WritesToStream(t *testing.T) {
	s, rdb, _, _ := newTestStore(t)
	ctx := context.Background()
	sessionID := uuid.New()
	playerID := uuid.New()

	s.Append(ctx, sessionID, TypeMoveApplied, &playerID, map[string]any{"cell": 4})

	key := streamKey(sessionID)
	entries, err := rdb.XRange(ctx, key, "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Values["session_id"] != sessionID.String() {
		t.Errorf("session_id: expected %s, got %s", sessionID, e.Values["session_id"])
	}
	if e.Values["type"] != string(TypeMoveApplied) {
		t.Errorf("type: expected %s, got %s", TypeMoveApplied, e.Values["type"])
	}
	if e.Values["player_id"] != playerID.String() {
		t.Errorf("player_id: expected %s, got %s", playerID, e.Values["player_id"])
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(e.Values["payload"].(string)), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["cell"] != float64(4) {
		t.Errorf("payload.cell: expected 4, got %v", payload["cell"])
	}
}

func TestAppend_NilPlayerID(t *testing.T) {
	s, rdb, _, _ := newTestStore(t)
	ctx := context.Background()
	sessionID := uuid.New()

	s.Append(ctx, sessionID, TypeGameStarted, nil, map[string]any{"game_id": "tictactoe"})

	entries, err := rdb.XRange(ctx, streamKey(sessionID), "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if _, hasPlayer := entries[0].Values["player_id"]; hasPlayer {
		t.Error("expected no player_id field when nil")
	}
}

func TestAppend_MultipleEvents(t *testing.T) {
	s, rdb, _, _ := newTestStore(t)
	ctx := context.Background()
	sessionID := uuid.New()

	s.Append(ctx, sessionID, TypeGameStarted, nil, map[string]any{})
	s.Append(ctx, sessionID, TypeMoveApplied, nil, map[string]any{})
	s.Append(ctx, sessionID, TypeGameOver, nil, map[string]any{})

	entries, err := rdb.XRange(ctx, streamKey(sessionID), "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestAppend_UnmarshalablePayload_NoOp(t *testing.T) {
	s, rdb, _, _ := newTestStore(t)
	ctx := context.Background()
	sessionID := uuid.New()

	// channels can't be marshaled to JSON
	s.Append(ctx, sessionID, TypeMoveApplied, nil, make(chan int))

	entries, err := rdb.XRange(ctx, streamKey(sessionID), "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after marshal failure, got %d", len(entries))
	}
}

func TestAppend_OccurredAtIsRecent(t *testing.T) {
	s, rdb, _, _ := newTestStore(t)
	ctx := context.Background()
	sessionID := uuid.New()
	before := time.Now().UTC()

	s.Append(ctx, sessionID, TypeGameStarted, nil, map[string]any{})

	entries, _ := rdb.XRange(ctx, streamKey(sessionID), "-", "+").Result()
	if len(entries) != 1 {
		t.Fatal("expected 1 entry")
	}
	raw := entries[0].Values["occurred_at"].(string)
	ts, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		t.Fatalf("parse occurred_at: %v", err)
	}
	if ts.Before(before) {
		t.Errorf("occurred_at %v is before test start %v", ts, before)
	}
}

// --- Persist tests -----------------------------------------------------------

func TestPersist_MovesToStoreAndDeletesStream(t *testing.T) {
	s, rdb, _, fs := newTestStore(t)
	ctx := context.Background()
	sessionID := uuid.New()
	pid := uuid.New()

	s.Append(ctx, sessionID, TypeGameStarted, nil, map[string]any{"game_id": "ttt"})
	s.Append(ctx, sessionID, TypeMoveApplied, &pid, map[string]any{"cell": 0})

	// Track what BulkCreateSessionEvents receives.
	var bulkParams []store.CreateSessionEventParams
	fs.OnBulkCreateSessionEvents = func(_ context.Context, params []store.CreateSessionEventParams) error {
		bulkParams = params
		return nil
	}

	s.Persist(ctx, sessionID)

	// Stream should be deleted.
	exists, _ := rdb.Exists(ctx, streamKey(sessionID)).Result()
	if exists != 0 {
		t.Error("expected stream to be deleted after persist")
	}

	// Events should have been passed to BulkCreateSessionEvents.
	if len(bulkParams) != 2 {
		t.Fatalf("expected 2 params, got %d", len(bulkParams))
	}
	if bulkParams[0].EventType != string(TypeGameStarted) {
		t.Errorf("first event type: expected game_started, got %s", bulkParams[0].EventType)
	}
	if bulkParams[1].EventType != string(TypeMoveApplied) {
		t.Errorf("second event type: expected move_applied, got %s", bulkParams[1].EventType)
	}
	if bulkParams[1].PlayerID == nil || *bulkParams[1].PlayerID != pid {
		t.Error("second event should have player_id set")
	}
}

func TestPersist_EmptyStream_NoOp(t *testing.T) {
	s, _, _, _ := newTestStore(t)
	ctx := context.Background()

	// Should not panic or error on non-existent stream.
	s.Persist(ctx, uuid.New())
}

// --- Read tests --------------------------------------------------------------

func TestRead_FromActiveStream(t *testing.T) {
	s, _, _, _ := newTestStore(t)
	ctx := context.Background()
	sessionID := uuid.New()
	pid := uuid.New()

	s.Append(ctx, sessionID, TypeGameStarted, nil, map[string]any{})
	s.Append(ctx, sessionID, TypeMoveApplied, &pid, map[string]any{"cell": 1})

	events, err := s.Read(ctx, sessionID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != TypeGameStarted {
		t.Errorf("first event: expected game_started, got %s", events[0].Type)
	}
	if events[1].Type != TypeMoveApplied {
		t.Errorf("second event: expected move_applied, got %s", events[1].Type)
	}
	if events[1].PlayerID == nil || *events[1].PlayerID != pid {
		t.Error("second event should have player_id")
	}
	if events[0].SessionID != sessionID {
		t.Error("event session_id should match")
	}
}

func TestRead_FromDB_AfterPersist(t *testing.T) {
	s, _, _, fs := newTestStore(t)
	ctx := context.Background()
	sessionID := uuid.New()

	// Simulate persisted events in the DB (stream doesn't exist).
	fs.OnListSessionEvents = func(_ context.Context, sid uuid.UUID) ([]store.SessionEvent, error) {
		if sid != sessionID {
			return nil, nil
		}
		return []store.SessionEvent{
			{
				StreamID:   "1-0",
				SessionID:  sid,
				EventType:  string(TypeGameStarted),
				OccurredAt: time.Now(),
			},
		}, nil
	}

	events, err := s.Read(ctx, sessionID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event from DB, got %d", len(events))
	}
	if events[0].Type != TypeGameStarted {
		t.Errorf("expected game_started, got %s", events[0].Type)
	}
}

// --- streamKey test ----------------------------------------------------------

func TestStreamKey_Format(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	got := streamKey(id)
	want := "events:session:11111111-1111-1111-1111-111111111111"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

// --- parseEntry test ---------------------------------------------------------

func TestParseEntry_MissingOccurredAt_DefaultsToNow(t *testing.T) {
	before := time.Now().UTC()
	entry := redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"type":    "game_started",
			"payload": `{"game_id":"ttt"}`,
		},
	}
	p, err := parseEntry(uuid.New(), entry)
	if err != nil {
		t.Fatalf("parseEntry: %v", err)
	}
	if p.OccurredAt.Before(before) {
		t.Error("expected occurred_at to default to ~now")
	}
}
