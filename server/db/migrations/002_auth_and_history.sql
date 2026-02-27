-- 002_auth_and_history.sql

-- ── Players enhancements ──────────────────────────────────────────────────────

ALTER TABLE players
    ADD COLUMN avatar_url   TEXT,
    ADD COLUMN deleted_at   TIMESTAMPTZ;

-- ── OAuth & Auth ──────────────────────────────────────────────────────────────

CREATE TABLE allowed_emails (
    email           TEXT PRIMARY KEY,
    note            TEXT,
    invited_by      UUID REFERENCES players(id),
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE oauth_identities (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    player_id       UUID NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    provider        TEXT NOT NULL,
    provider_id     TEXT NOT NULL,
    email           TEXT NOT NULL,
    avatar_url      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_id)
);

-- ── Room enhancements ─────────────────────────────────────────────────────────

ALTER TABLE rooms
    ADD COLUMN turn_timeout_secs  INT,
    ADD COLUMN allow_spectators   BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN deleted_at         TIMESTAMPTZ;

-- ── Spectators ────────────────────────────────────────────────────────────────

CREATE TABLE spectator_links (
    token           TEXT PRIMARY KEY,
    room_id         UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    created_by      UUID NOT NULL REFERENCES players(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Session enhancements ──────────────────────────────────────────────────────

ALTER TABLE game_sessions
    ADD COLUMN name             TEXT,
    ADD COLUMN deleted_at       TIMESTAMPTZ,
    ADD COLUMN suspend_count    INT NOT NULL DEFAULT 0,
    ADD COLUMN suspended_at     TIMESTAMPTZ,
    ADD COLUMN suspended_reason TEXT;

-- ── Move enhancements ─────────────────────────────────────────────────────────

ALTER TABLE moves
    ADD COLUMN state_after  JSONB,
    ADD COLUMN move_number  INT;

-- ── Results & leaderboard ────────────────────────────────────────────────────

CREATE TABLE game_results (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id      UUID NOT NULL REFERENCES game_sessions(id),
    game_id         TEXT NOT NULL,
    winner_id       UUID REFERENCES players(id),
    is_draw         BOOLEAN NOT NULL DEFAULT false,
    ended_by        TEXT NOT NULL,              -- 'win', 'draw', 'forfeit', 'suspended'
    duration_secs   INT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE game_result_players (
    result_id       UUID NOT NULL REFERENCES game_results(id) ON DELETE CASCADE,
    player_id       UUID NOT NULL REFERENCES players(id),
    seat            INT NOT NULL,
    outcome         TEXT NOT NULL,              -- 'win', 'loss', 'draw', 'forfeit'
    PRIMARY KEY (result_id, player_id)
);

-- ── Rematch ───────────────────────────────────────────────────────────────────

CREATE TABLE rematch_votes (
    session_id      UUID NOT NULL REFERENCES game_sessions(id) ON DELETE CASCADE,
    player_id       UUID NOT NULL REFERENCES players(id),
    voted_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (session_id, player_id)
);

-- ── Indexes ───────────────────────────────────────────────────────────────────

CREATE INDEX idx_oauth_identities_player    ON oauth_identities(player_id);
CREATE INDEX idx_oauth_identities_email     ON oauth_identities(email);
CREATE INDEX idx_rooms_deleted_at           ON rooms(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_game_sessions_deleted_at   ON game_sessions(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_game_sessions_suspended    ON game_sessions(suspended_at) WHERE suspended_at IS NOT NULL;
CREATE INDEX idx_game_results_game_id       ON game_results(game_id);
CREATE INDEX idx_game_results_winner        ON game_results(winner_id) WHERE winner_id IS NOT NULL;
CREATE INDEX idx_game_result_players_player ON game_result_players(player_id);
CREATE INDEX idx_moves_session_number       ON moves(session_id, move_number);
CREATE INDEX idx_spectator_links_room       ON spectator_links(room_id);