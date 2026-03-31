-- 007_player_settings.sql
-- Adds per-player settings stored as a JSONB object.
-- A single row per player avoids EAV overhead and allows atomic updates.
-- The settings column defaults to an empty object; the application layer
-- merges defaults at read time so the schema never needs updating for new keys.

CREATE TABLE player_settings (
    player_id  UUID        PRIMARY KEY REFERENCES players(id) ON DELETE CASCADE,
    settings   JSONB       NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);