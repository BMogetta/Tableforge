-- 004_turn_timeout.sql
-- Adds per-game configuration and turn timeout tracking on sessions.

-- Per-game configuration defaults.
-- Populated at startup by each game plugin via the seeder or init script.
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

-- Add turn_timeout_secs to game_sessions so each session knows its effective timeout.
-- NULL means no timeout enforced.
ALTER TABLE game_sessions
    ADD COLUMN turn_timeout_secs INT;

-- Add last_move_at to track when the current turn started.
ALTER TABLE game_sessions
    ADD COLUMN last_move_at TIMESTAMPTZ NOT NULL DEFAULT NOW();