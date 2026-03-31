package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SessionEvent is a single persisted event for a game session.
type SessionEvent struct {
	ID         int64           `json:"id"`
	SessionID  uuid.UUID       `json:"session_id"`
	StreamID   string          `json:"stream_id"`
	EventType  string          `json:"event_type"`
	PlayerID   *uuid.UUID      `json:"player_id,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	OccurredAt time.Time       `json:"occurred_at"`
}

// CreateSessionEventParams holds the data for a single event to insert.
type CreateSessionEventParams struct {
	SessionID  uuid.UUID
	StreamID   string
	EventType  string
	PlayerID   *uuid.UUID
	Payload    json.RawMessage
	OccurredAt time.Time
}

// BulkCreateSessionEvents inserts all events for a session in a single transaction.
func (s *PGStore) BulkCreateSessionEvents(ctx context.Context, params []CreateSessionEventParams) error {
	if len(params) == 0 {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("BulkCreateSessionEvents: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, p := range params {
		_, err := tx.Exec(ctx,
			`INSERT INTO session_events
			   (session_id, stream_id, event_type, player_id, payload, occurred_at)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (stream_id) DO NOTHING`,
			p.SessionID, p.StreamID, p.EventType, p.PlayerID, p.Payload, p.OccurredAt,
		)
		if err != nil {
			return fmt.Errorf("BulkCreateSessionEvents: insert %s: %w", p.StreamID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("BulkCreateSessionEvents: commit: %w", err)
	}
	return nil
}

// ListSessionEvents returns all persisted events for a session, ordered by occurred_at.
func (s *PGStore) ListSessionEvents(ctx context.Context, sessionID uuid.UUID) ([]SessionEvent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, session_id, stream_id, event_type, player_id, payload, occurred_at
		 FROM session_events
		 WHERE session_id = $1
		 ORDER BY occurred_at ASC, id ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListSessionEvents: %w", err)
	}
	defer rows.Close()

	var events []SessionEvent
	for rows.Next() {
		var e SessionEvent
		if err := rows.Scan(
			&e.ID, &e.SessionID, &e.StreamID, &e.EventType,
			&e.PlayerID, &e.Payload, &e.OccurredAt,
		); err != nil {
			return nil, fmt.Errorf("ListSessionEvents scan: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
