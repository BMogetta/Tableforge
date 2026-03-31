-- 003_chat_ratings_moderation.sql
-- Adds: room chat, direct messages, player mutes, ratings, session mode,
-- and pause/resume vote columns.

-- ── Room chat ─────────────────────────────────────────────────────────────────
-- Auditable, never deleted. Managers may hide individual messages.

CREATE TABLE room_messages (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    room_id    UUID        NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    player_id  UUID        NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    content    TEXT        NOT NULL,
    reported   BOOLEAN     NOT NULL DEFAULT false,
    hidden     BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_room_messages_room_id    ON room_messages(room_id);
CREATE INDEX idx_room_messages_player_id  ON room_messages(player_id);
CREATE INDEX idx_room_messages_created_at ON room_messages(room_id, created_at);

-- ── Direct messages ───────────────────────────────────────────────────────────

CREATE TABLE direct_messages (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    sender_id   UUID        NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    receiver_id UUID        NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    content     TEXT        NOT NULL,
    read_at     TIMESTAMPTZ,
    reported    BOOLEAN     NOT NULL DEFAULT false,
    hidden      BOOLEAN     NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_direct_messages_sender    ON direct_messages(sender_id);
CREATE INDEX idx_direct_messages_receiver  ON direct_messages(receiver_id);
-- Supports GetDMHistory(playerA, playerB) efficiently in both directions.
CREATE INDEX idx_direct_messages_convo ON direct_messages(
    LEAST(sender_id::TEXT, receiver_id::TEXT),
    GREATEST(sender_id::TEXT, receiver_id::TEXT),
    created_at
);

-- ── Player mutes ──────────────────────────────────────────────────────────────
-- Cross-session, server-side. Survive reconnects and new sessions.

CREATE TABLE player_mutes (
    muter_id   UUID        NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    muted_id   UUID        NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (muter_id, muted_id)
);

CREATE INDEX idx_player_mutes_muter ON player_mutes(muter_id);

-- ── Ratings ───────────────────────────────────────────────────────────────────
-- Decoupled from players. Only updated in ranked sessions.

CREATE TABLE ratings (
    player_id      UUID        PRIMARY KEY REFERENCES players(id) ON DELETE CASCADE,
    mmr            FLOAT       NOT NULL DEFAULT 1500,
    display_rating FLOAT       NOT NULL DEFAULT 1500,
    games_played   INT         NOT NULL DEFAULT 0,
    win_streak     INT         NOT NULL DEFAULT 0,
    loss_streak    INT         NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ratings_mmr ON ratings(mmr DESC);

-- ── Session: mode + pause/resume votes ───────────────────────────────────────

ALTER TABLE game_sessions
    ADD COLUMN mode         TEXT     NOT NULL DEFAULT 'casual'
        CHECK (mode IN ('casual', 'ranked')),
    ADD COLUMN pause_votes  TEXT[]   NOT NULL DEFAULT '{}',
    ADD COLUMN resume_votes TEXT[]   NOT NULL DEFAULT '{}';