-- Migration: session_events
-- Stores the persisted event log for each game session.
-- Events are first written to a Redis Stream while the session is active,
-- then bulk-inserted here when the session ends and the stream is deleted.

CREATE TABLE session_events (
    id          BIGSERIAL PRIMARY KEY,
    session_id  UUID        NOT NULL REFERENCES game_sessions(id) ON DELETE CASCADE,
    -- stream_id is the Redis Stream entry ID (e.g. "1700000000000-0").
    -- Used as a natural idempotency key during persistence.
    stream_id   TEXT        NOT NULL,
    event_type  TEXT        NOT NULL,
    player_id   UUID        REFERENCES players(id) ON DELETE SET NULL,
    payload     JSONB,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT session_events_stream_id_unique UNIQUE (stream_id)
);

CREATE INDEX idx_session_events_session_id ON session_events(session_id);
CREATE INDEX idx_session_events_occurred_at ON session_events(session_id, occurred_at);