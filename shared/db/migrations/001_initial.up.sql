-- =============================================================================
-- CORE IDENTITY, LOBBY & GAME ENGINE (Public Schema)
-- =============================================================================
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE player_role AS ENUM ('player', 'manager', 'owner');

-- IDENTITY & AUTH
CREATE TABLE players (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    username    VARCHAR(32)  NOT NULL UNIQUE,
    role        player_role  NOT NULL DEFAULT 'player',
    avatar_url  TEXT,
    is_bot      BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE TABLE player_settings (
    player_id  UUID         PRIMARY KEY REFERENCES players(id) ON DELETE CASCADE,
    settings   JSONB        NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE player_sessions (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id    UUID         NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    device_info  JSONB        NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ  NOT NULL,
    revoked_at   TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE allowed_emails (
    email       TEXT         PRIMARY KEY,
    role        player_role  NOT NULL DEFAULT 'player',
    note        TEXT,
    invited_by  UUID         REFERENCES players(id),
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE oauth_identities (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    player_id    UUID NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    provider     TEXT NOT NULL,
    provider_id  TEXT NOT NULL,
    email        TEXT NOT NULL,
    avatar_url   TEXT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_id)
);

-- LOBBY & MATCHMAKING
CREATE TABLE rooms (
    id                UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    code              VARCHAR(8)   NOT NULL UNIQUE,
    game_id           VARCHAR(64)  NOT NULL,
    owner_id          UUID         NOT NULL REFERENCES players(id),
    status            VARCHAR(16)  NOT NULL DEFAULT 'waiting' 
                                   CHECK (status IN ('waiting', 'in_progress', 'finished')),
    max_players       INT          NOT NULL DEFAULT 2,
    turn_timeout_secs INT,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ
);

CREATE TABLE room_players (
    room_id   UUID         NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    player_id UUID         NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    seat      INT          NOT NULL,
    joined_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (room_id, player_id)
);

CREATE TABLE room_settings (
    room_id    UUID         NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    key        TEXT         NOT NULL,
    value      TEXT         NOT NULL,
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (room_id, key)
);

CREATE TABLE game_configs (
    game_id              TEXT PRIMARY KEY,
    default_timeout_secs INT  NOT NULL DEFAULT 60,
    min_timeout_secs     INT  NOT NULL DEFAULT 30,
    max_timeout_secs     INT  NOT NULL DEFAULT 600,
    timeout_penalty      TEXT NOT NULL DEFAULT 'lose_turn'
                              CHECK (timeout_penalty IN ('lose_turn', 'lose_game'))
);

CREATE TABLE room_invitations (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id      UUID         NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    invited_by   UUID         NOT NULL REFERENCES players(id),
    invited_id   UUID         NOT NULL REFERENCES players(id),
    token        TEXT         NOT NULL UNIQUE,
    status       TEXT         NOT NULL DEFAULT 'pending'
                                    CHECK (status IN ('pending', 'accepted', 'declined', 'expired')),
    expires_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW() + INTERVAL '48 hours',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    responded_at TIMESTAMPTZ
);

-- MESSAGING & NOTIFICATIONS
CREATE TABLE room_messages (
    id         UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    room_id    UUID         NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    player_id  UUID         NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    content    TEXT         NOT NULL,
    reported   BOOLEAN      NOT NULL DEFAULT FALSE,
    hidden     BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE direct_messages (
    id           UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    sender_id    UUID         NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    receiver_id  UUID         NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    content      TEXT         NOT NULL,
    read_at      TIMESTAMPTZ,
    reported     BOOLEAN      NOT NULL DEFAULT FALSE,
    hidden     BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE notifications (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id         UUID         NOT NULL REFERENCES players(id),
    type              TEXT         NOT NULL CHECK (type IN ('friend_request', 'room_invitation', 'ban_issued')),
    payload           JSONB        NOT NULL DEFAULT '{}',
    action_taken      TEXT         CHECK (action_taken IN ('accepted', 'declined')),
    action_expires_at TIMESTAMPTZ,
    read_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- GAME ENGINE
CREATE TABLE game_sessions (
    id                UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    room_id           UUID         NOT NULL REFERENCES rooms(id),
    game_id           VARCHAR(64)  NOT NULL,
    name              TEXT,
    mode              TEXT         NOT NULL DEFAULT 'casual' CHECK (mode IN ('casual', 'ranked')),
    state             JSONB        NOT NULL DEFAULT '{}',
    move_count        INT          NOT NULL DEFAULT 0,
    suspend_count     INT          NOT NULL DEFAULT 0,
    suspended_at      TIMESTAMPTZ,
    suspended_reason  TEXT,
    turn_timeout_secs INT,
    last_move_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    ready_players     TEXT[]       NOT NULL DEFAULT '{}',
    started_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    finished_at       TIMESTAMPTZ,
    deleted_at        TIMESTAMPTZ
);

CREATE TABLE moves (
    id           UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id   UUID         NOT NULL REFERENCES game_sessions(id) ON DELETE CASCADE,
    player_id    UUID         NOT NULL REFERENCES players(id),
    payload      JSONB        NOT NULL DEFAULT '{}',
    state_after  BYTEA,
    move_number  INT          NOT NULL,
    applied_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (session_id, move_number)
);

CREATE TABLE session_events (
    id           BIGSERIAL    PRIMARY KEY,
    session_id   UUID         NOT NULL REFERENCES game_sessions(id) ON DELETE CASCADE,
    stream_id    TEXT         NOT NULL UNIQUE,
    event_type   TEXT         NOT NULL,
    player_id    UUID         REFERENCES players(id) ON DELETE SET NULL,
    payload      JSONB,
    occurred_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE session_pause_votes (
    session_id UUID         NOT NULL REFERENCES game_sessions(id) ON DELETE CASCADE,
    player_id  UUID         NOT NULL REFERENCES players(id),
    vote_type  TEXT         NOT NULL CHECK (vote_type IN ('pause', 'resume')),
    voted_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (session_id, player_id, vote_type)
);

CREATE TABLE game_results (
    id            UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id    UUID         NOT NULL REFERENCES game_sessions(id),
    game_id       TEXT         NOT NULL,
    winner_id     UUID         REFERENCES players(id),
    is_draw       BOOLEAN      NOT NULL DEFAULT FALSE,
    ended_by      TEXT         NOT NULL CHECK (ended_by IN ('win', 'draw', 'forfeit', 'suspended', 'timeout', 'ready_timeout')),
    duration_secs INT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE game_result_players (
    result_id UUID NOT NULL REFERENCES game_results(id) ON DELETE CASCADE,
    player_id UUID NOT NULL REFERENCES players(id),
    seat      INT  NOT NULL,
    outcome   TEXT NOT NULL CHECK (outcome IN ('win', 'loss', 'draw', 'forfeit')),
    PRIMARY KEY (result_id, player_id)
);

CREATE TABLE rematch_votes (
    session_id UUID         NOT NULL REFERENCES game_sessions(id) ON DELETE CASCADE,
    player_id  UUID         NOT NULL REFERENCES players(id),
    voted_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (session_id, player_id)
);

-- TOURNAMENTS
CREATE TABLE tournaments (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT         NOT NULL,
    game_id           TEXT         NOT NULL,
    format            TEXT         NOT NULL CHECK (format IN ('elimination', 'round_robin')),
    status            TEXT         NOT NULL DEFAULT 'draft'
                                   CHECK (status IN ('draft', 'registration', 'in_progress', 'finished', 'cancelled')),
    max_participants  INT,
    created_by        UUID         NOT NULL REFERENCES players(id),
    winner_id         UUID         REFERENCES players(id),
    starts_at         TIMESTAMPTZ,
    ends_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE tournament_participants (
    tournament_id UUID         NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
    player_id     UUID         NOT NULL REFERENCES players(id),
    seed          INT,
    eliminated_at TIMESTAMPTZ,
    registered_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tournament_id, player_id)
);

CREATE TABLE tournament_rounds (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tournament_id  UUID         NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
    round_number   INT          NOT NULL,
    status         TEXT         NOT NULL DEFAULT 'pending'
                                 CHECK (status IN ('pending', 'in_progress', 'finished')),
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (tournament_id, round_number)
);

CREATE TABLE tournament_matches (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    round_id       UUID         NOT NULL REFERENCES tournament_rounds(id) ON DELETE CASCADE,
    player_one_id  UUID         NOT NULL REFERENCES players(id),
    player_two_id  UUID         NOT NULL REFERENCES players(id),
    session_id     UUID         REFERENCES game_sessions(id),
    winner_id      UUID         REFERENCES players(id),
    bye            BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- INDEXES
CREATE INDEX idx_players_deleted_at ON players(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_player_sessions_player_active ON player_sessions(player_id, created_at DESC) WHERE revoked_at IS NULL;
CREATE INDEX idx_rooms_code ON rooms(code);
CREATE INDEX idx_game_sessions_room ON game_sessions(room_id);
CREATE INDEX idx_room_messages_room_id ON room_messages(room_id);
CREATE INDEX idx_notifications_player_unread ON notifications(player_id, created_at DESC) WHERE read_at IS NULL;
CREATE INDEX idx_player_sessions_player_id ON player_sessions(player_id) WHERE revoked_at IS NULL;