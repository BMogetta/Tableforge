-- 001_initial.sql
-- Creates the core tables for Tableforge.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Players registered in the system
CREATE TABLE players (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username    VARCHAR(32) NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Rooms are lobbies where players gather before a game starts
CREATE TABLE rooms (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code        VARCHAR(8) NOT NULL UNIQUE,    -- short human-readable join code
    game_id     VARCHAR(64) NOT NULL,          -- matches engine.Game.ID()
    owner_id    UUID NOT NULL REFERENCES players(id),
    status      VARCHAR(16) NOT NULL DEFAULT 'waiting', -- waiting | in_progress | finished
    max_players INT NOT NULL DEFAULT 2,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Junction table: which players are in which room
CREATE TABLE room_players (
    room_id     UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    player_id   UUID NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    seat        INT NOT NULL,                  -- turn order index (0-based)
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (room_id, player_id)
);

-- One game session per room (created when the room moves to in_progress)
CREATE TABLE game_sessions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    room_id     UUID NOT NULL REFERENCES rooms(id),
    game_id     VARCHAR(64) NOT NULL,
    state       JSONB NOT NULL DEFAULT '{}',   -- serialized engine.GameState
    move_count  INT NOT NULL DEFAULT 0,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

-- Full move history for a session
CREATE TABLE moves (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id  UUID NOT NULL REFERENCES game_sessions(id) ON DELETE CASCADE,
    player_id   UUID NOT NULL REFERENCES players(id),
    payload     JSONB NOT NULL DEFAULT '{}',   -- serialized engine.Move.Payload
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_rooms_code ON rooms(code);
CREATE INDEX idx_rooms_status ON rooms(status);
CREATE INDEX idx_room_players_room ON room_players(room_id);
CREATE INDEX idx_game_sessions_room ON game_sessions(room_id);
CREATE INDEX idx_moves_session ON moves(session_id);