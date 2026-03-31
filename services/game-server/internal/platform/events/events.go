// Package events provides append-only event sourcing backed by Redis Streams.
//
// While a game session is active, events are appended to a Redis Stream
// at key "events:session:<session_id>". When the session ends, Persist
// writes all events to the session_events table in Postgres and deletes
// the stream.
//
// Events can be read from Postgres after persistence via Read.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tableforge/game-server/internal/platform/store"
)

// Type identifies the kind of event.
type Type string

const (
	TypeGameStarted        Type = "game_started"
	TypeMoveApplied        Type = "move_applied"
	TypeGameOver           Type = "game_over"
	TypeTurnTimeout        Type = "turn_timeout"
	TypePlayerConnected    Type = "player_connected"
	TypePlayerDisconnected Type = "player_disconnected"
	TypeSpectatorJoined    Type = "spectator_joined"
	TypeSpectatorLeft      Type = "spectator_left"
	TypePlayerSurrendered  Type = "player_surrendered"
	TypeRematchVoted       Type = "rematch_voted"
	TypeSessionSuspended   Type = "session_suspended"
	TypeSessionResumed     Type = "session_resumed"
)

// Event is a single entry in the event stream.
type Event struct {
	ID         string          `json:"id"` // Redis stream entry ID (e.g. "1700000000000-0")
	SessionID  uuid.UUID       `json:"session_id"`
	Type       Type            `json:"type"`
	PlayerID   *uuid.UUID      `json:"player_id,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	OccurredAt time.Time       `json:"occurred_at"`
}

// streamKey returns the Redis Stream key for a session.
func streamKey(sessionID uuid.UUID) string {
	return "events:session:" + sessionID.String()
}

// Store handles appending and persisting session events.
type Store struct {
	rdb *redis.Client
	st  store.Store
}

// New creates an event Store.
func New(rdb *redis.Client, st store.Store) *Store {
	return &Store{rdb: rdb, st: st}
}

// Append writes an event to the Redis Stream for the session.
// It is a fire-and-forget operation — errors are logged but not returned
// to avoid interrupting game flow.
func (s *Store) Append(ctx context.Context, sessionID uuid.UUID, eventType Type, playerID *uuid.UUID, payload any) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		slog.Error("events: marshal payload failed", "session_id", sessionID, "event_type", eventType, "error", err)
		return
	}

	fields := map[string]any{
		"session_id":  sessionID.String(),
		"type":        string(eventType),
		"payload":     string(payloadJSON),
		"occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	if playerID != nil {
		fields["player_id"] = playerID.String()
	}

	if err := s.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey(sessionID),
		Values: fields,
	}).Err(); err != nil {
		slog.Error("events: append failed", "session_id", sessionID, "event_type", eventType, "error", err)
	}
}

// Persist reads all events from the Redis Stream, writes them to Postgres,
// and deletes the stream. Call this when a session ends.
func (s *Store) Persist(ctx context.Context, sessionID uuid.UUID) {
	key := streamKey(sessionID)

	entries, err := s.rdb.XRange(ctx, key, "-", "+").Result()
	if err != nil {
		slog.Error("events: persist xrange failed", "session_id", sessionID, "error", err)
		return
	}
	if len(entries) == 0 {
		return
	}

	params := make([]store.CreateSessionEventParams, 0, len(entries))
	for _, entry := range entries {
		p, err := parseEntry(sessionID, entry)
		if err != nil {
			slog.Warn("events: persist parse entry failed", "entry_id", entry.ID, "error", err)
			continue
		}
		params = append(params, p)
	}

	if err := s.st.BulkCreateSessionEvents(ctx, params); err != nil {
		slog.Error("events: persist bulk insert failed", "session_id", sessionID, "error", err)
		return
	}

	// Stream is no longer needed after persistence.
	if err := s.rdb.Del(ctx, key).Err(); err != nil {
		slog.Error("events: persist delete stream failed", "session_id", sessionID, "error", err)
	}
}

// Read returns all persisted events for a session from Postgres.
// If the session is still active, it reads from the Redis Stream instead
// so callers always get a complete picture.
func (s *Store) Read(ctx context.Context, sessionID uuid.UUID) ([]Event, error) {
	// Check if the stream still exists (session is active).
	exists, err := s.rdb.Exists(ctx, streamKey(sessionID)).Result()
	if err != nil {
		slog.Warn("events: read exists check failed", "session_id", sessionID, "error", err)
		// Fall through to Postgres.
	}

	if exists > 0 {
		return s.readFromStream(ctx, sessionID)
	}

	return s.readFromDB(ctx, sessionID)
}

// readFromStream reads events from the Redis Stream for an active session.
func (s *Store) readFromStream(ctx context.Context, sessionID uuid.UUID) ([]Event, error) {
	entries, err := s.rdb.XRange(ctx, streamKey(sessionID), "-", "+").Result()
	if err != nil {
		return nil, fmt.Errorf("events.Read: XRange: %w", err)
	}

	events := make([]Event, 0, len(entries))
	for _, entry := range entries {
		p, err := parseEntry(sessionID, entry)
		if err != nil {
			slog.Warn("events: read parse entry failed", "entry_id", entry.ID, "error", err)
			continue
		}
		events = append(events, entryParamsToEvent(entry.ID, p))
	}
	return events, nil
}

// readFromDB reads persisted events from Postgres for a finished session.
func (s *Store) readFromDB(ctx context.Context, sessionID uuid.UUID) ([]Event, error) {
	rows, err := s.st.ListSessionEvents(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("events.Read: list from db: %w", err)
	}

	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		e := Event{
			ID:         row.StreamID,
			SessionID:  row.SessionID,
			Type:       Type(row.EventType),
			Payload:    row.Payload,
			OccurredAt: row.OccurredAt,
		}
		if row.PlayerID != nil {
			e.PlayerID = row.PlayerID
		}
		events = append(events, e)
	}
	return events, nil
}

// parseEntry converts a Redis Stream entry to store.CreateSessionEventParams.
func parseEntry(sessionID uuid.UUID, entry redis.XMessage) (store.CreateSessionEventParams, error) {
	p := store.CreateSessionEventParams{
		SessionID: sessionID,
		StreamID:  entry.ID,
		EventType: entry.Values["type"].(string),
	}

	if raw, ok := entry.Values["payload"].(string); ok {
		p.Payload = json.RawMessage(raw)
	}

	if raw, ok := entry.Values["player_id"].(string); ok && raw != "" {
		id, err := uuid.Parse(raw)
		if err == nil {
			p.PlayerID = &id
		}
	}

	if raw, ok := entry.Values["occurred_at"].(string); ok {
		t, err := time.Parse(time.RFC3339Nano, raw)
		if err == nil {
			p.OccurredAt = t
		}
	}
	if p.OccurredAt.IsZero() {
		p.OccurredAt = time.Now().UTC()
	}

	return p, nil
}

// entryParamsToEvent converts store params back to an Event for the Read path.
func entryParamsToEvent(streamID string, p store.CreateSessionEventParams) Event {
	return Event{
		ID:         streamID,
		SessionID:  p.SessionID,
		Type:       Type(p.EventType),
		PlayerID:   p.PlayerID,
		Payload:    p.Payload,
		OccurredAt: p.OccurredAt,
	}
}
