-- 001_initial.sql
-- Full schema for Tableforge.
-- Consolidated from the original 5 migrations (001–005).
-- Omits rooms.allow_spectators and spectator_links — both superseded by the
-- allow_spectators room_setting (see room_settings known keys below).

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ── Enums ────────────────────────────────────────────────────────────────────

-- Role enum: controls access to admin features.
CREATE TYPE player_role AS ENUM ('player', 'manager', 'owner');

-- ── Players ──────────────────────────────────────────────────────────────────

CREATE TABLE players (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    username    VARCHAR(32) NOT NULL UNIQUE,
    role        player_role NOT NULL DEFAULT 'player',
    avatar_url  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

-- ── Auth ─────────────────────────────────────────────────────────────────────

-- Allowlist of emails that may register. Role determines the player's initial
-- role on first login. invited_by tracks who sent the invite.
CREATE TABLE allowed_emails (
    email       TEXT        PRIMARY KEY,
    role        player_role NOT NULL DEFAULT 'player',
    note        TEXT,
    invited_by  UUID        REFERENCES players(id),
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- OAuth identities linked to a player. One player may have multiple providers.
CREATE TABLE oauth_identities (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    player_id   UUID NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    provider    TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    email       TEXT NOT NULL,
    avatar_url  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_id)
);

-- ── Rooms ────────────────────────────────────────────────────────────────────

CREATE TABLE rooms (
    id               UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    code             VARCHAR(8)  NOT NULL UNIQUE,   -- short human-readable join code
    game_id          VARCHAR(64) NOT NULL,           -- matches engine.Game.ID()
    owner_id         UUID        NOT NULL REFERENCES players(id),
    status           VARCHAR(16) NOT NULL DEFAULT 'waiting', -- waiting | in_progress | finished
    max_players      INT         NOT NULL DEFAULT 2,
    turn_timeout_secs INT,                           -- NULL means no timeout enforced
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ
);

-- Junction table: which players are currently in which room.
CREATE TABLE room_players (
    room_id   UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    player_id UUID NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    seat      INT  NOT NULL,                        -- turn order index (0-based)
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (room_id, player_id)
);

-- ── Room settings ─────────────────────────────────────────────────────────────
--
-- Generic key/value settings per room, scoped to a single room.
-- Settings may be changed while the room is in `waiting` state.
-- The application layer (lobby.validateSetting) enforces which keys are valid
-- and what values are allowed. Default rows are inserted by CreateRoom so
-- reads never need fallback logic.
--
-- Known keys (enforced by the application, documented here for reference):
--
--   room_visibility  TEXT  (default: 'public')
--     Controls whether the room appears in the public lobby list.
--     Values:
--       'public'   — room is listed in the lobby; code is visible to everyone
--       'private'  — room is hidden from the lobby list; code is redacted for
--                    non-participants; joinable only by entering the code directly
--
--   allow_spectators  TEXT  (default: 'no')
--     Controls whether non-participants may connect as spectators via WebSocket.
--     Spectators receive all game events except rematch_vote (participants only).
--     Values:
--       'no'   — spectator WS connections are rejected with 403
--       'yes'  — spectator connections are accepted; a spectator_joined /
--                spectator_left event is broadcast to the room on connect/disconnect
--
--   first_mover_policy  TEXT  (default: 'random')
--     Controls who takes the first turn on the very first game in the room.
--     Values:
--       'random'       — chosen at random (default)
--       'fixed'        — always the player in first_mover_seat
--       'game_default' — delegated to the game engine
--
--   first_mover_seat  TEXT  (default: '0')
--     Seat index (0-based) of the player who goes first when
--     first_mover_policy = 'fixed'. Stored as TEXT to fit the generic KV
--     schema. Ignored for all other policies.
--
--   rematch_first_mover_policy  TEXT  (default: 'random')
--     Controls who takes the first turn on rematches (second game onwards).
--     Values:
--       'random'         — chosen at random
--       'fixed'          — always the player in first_mover_seat
--       'game_default'   — delegated to the game engine
--       'winner_first'   — previous game winner goes first
--       'loser_first'    — previous game loser goes first
--       'winner_chooses' — winner picks who goes first
--       'loser_chooses'  — loser picks who goes first

CREATE TABLE room_settings (
    room_id    UUID        NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    key        TEXT        NOT NULL,
    value      TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (room_id, key)
);

-- ── Game configs ─────────────────────────────────────────────────────────────
--
-- Per-game configuration defaults. Populated at startup by each game plugin.

CREATE TABLE game_configs (
    game_id              TEXT PRIMARY KEY,
    default_timeout_secs INT  NOT NULL DEFAULT 60,
    min_timeout_secs     INT  NOT NULL DEFAULT 30,
    max_timeout_secs     INT  NOT NULL DEFAULT 600,
    -- 'lose_turn': current player forfeits their turn, game continues.
    -- 'lose_game': current player forfeits the entire game.
    timeout_penalty      TEXT NOT NULL DEFAULT 'lose_turn'
        CHECK (timeout_penalty IN ('lose_turn', 'lose_game'))
);

-- Seed default config for tictactoe.
INSERT INTO game_configs (game_id, default_timeout_secs, min_timeout_secs, max_timeout_secs, timeout_penalty)
VALUES ('tictactoe', 30, 10, 120, 'lose_game');

-- ── Game sessions ─────────────────────────────────────────────────────────────

-- One active session per room (created when the room moves to in_progress).
CREATE TABLE game_sessions (
    id               UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    room_id          UUID        NOT NULL REFERENCES rooms(id),
    game_id          VARCHAR(64) NOT NULL,
    name             TEXT,
    state            JSONB       NOT NULL DEFAULT '{}',  -- serialized engine.GameState
    move_count       INT         NOT NULL DEFAULT 0,
    suspend_count    INT         NOT NULL DEFAULT 0,
    suspended_at     TIMESTAMPTZ,
    suspended_reason TEXT,
    turn_timeout_secs INT,                               -- effective timeout for this session
    last_move_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- when the current turn started
    started_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at      TIMESTAMPTZ,
    deleted_at       TIMESTAMPTZ
);

-- Full move history for a session.
CREATE TABLE moves (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id  UUID        NOT NULL REFERENCES game_sessions(id) ON DELETE CASCADE,
    player_id   UUID        NOT NULL REFERENCES players(id),
    payload     JSONB       NOT NULL DEFAULT '{}',  -- serialized engine.Move.Payload
    state_after JSONB,                               -- game state snapshot after this move
    move_number INT,                                 -- 1-based position in the session
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Results & leaderboard ────────────────────────────────────────────────────

CREATE TABLE game_results (
    id            UUID    PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id    UUID    NOT NULL REFERENCES game_sessions(id),
    game_id       TEXT    NOT NULL,
    winner_id     UUID    REFERENCES players(id),
    is_draw       BOOLEAN NOT NULL DEFAULT false,
    ended_by      TEXT    NOT NULL,   -- 'win' | 'draw' | 'forfeit' | 'suspended'
    duration_secs INT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE game_result_players (
    result_id UUID NOT NULL REFERENCES game_results(id) ON DELETE CASCADE,
    player_id UUID NOT NULL REFERENCES players(id),
    seat      INT  NOT NULL,
    outcome   TEXT NOT NULL,          -- 'win' | 'loss' | 'draw' | 'forfeit'
    PRIMARY KEY (result_id, player_id)
);

-- ── Rematch ───────────────────────────────────────────────────────────────────

CREATE TABLE rematch_votes (
    session_id UUID        NOT NULL REFERENCES game_sessions(id) ON DELETE CASCADE,
    player_id  UUID        NOT NULL REFERENCES players(id),
    voted_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (session_id, player_id)
);

-- ── Indexes ───────────────────────────────────────────────────────────────────

CREATE INDEX idx_rooms_code                 ON rooms(code);
CREATE INDEX idx_rooms_status               ON rooms(status);
CREATE INDEX idx_rooms_deleted_at           ON rooms(deleted_at)         WHERE deleted_at IS NULL;
CREATE INDEX idx_room_players_room          ON room_players(room_id);
CREATE INDEX idx_room_settings_room         ON room_settings(room_id);
CREATE INDEX idx_oauth_identities_player    ON oauth_identities(player_id);
CREATE INDEX idx_oauth_identities_email     ON oauth_identities(email);
CREATE INDEX idx_game_sessions_room         ON game_sessions(room_id);
CREATE INDEX idx_game_sessions_deleted_at   ON game_sessions(deleted_at)  WHERE deleted_at IS NULL;
CREATE INDEX idx_game_sessions_suspended    ON game_sessions(suspended_at) WHERE suspended_at IS NOT NULL;
CREATE INDEX idx_moves_session              ON moves(session_id);
CREATE INDEX idx_moves_session_number       ON moves(session_id, move_number);
CREATE INDEX idx_game_results_game_id       ON game_results(game_id);
CREATE INDEX idx_game_results_winner        ON game_results(winner_id)    WHERE winner_id IS NOT NULL;
CREATE INDEX idx_game_result_players_player ON game_result_players(player_id);