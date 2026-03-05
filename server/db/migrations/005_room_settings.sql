-- 005_room_settings.sql
-- Generic key/value settings per room.
-- Settings are scoped to a room and can be changed while the room is in `waiting` state.
-- The application layer enforces which keys are valid and what values are allowed.
-- Default rows are inserted by CreateRoom so reads never need fallback logic.

CREATE TABLE room_settings (
    room_id     UUID        NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    key         TEXT        NOT NULL,
    value       TEXT        NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (room_id, key)
);

CREATE INDEX idx_room_settings_room ON room_settings(room_id);

-- Known keys (enforced by the application, documented here for reference):
--
--   first_mover_policy  TEXT  (default: 'random')
--     Controls who takes the first turn on the very first game in the room.
--     Values:
--       'random'          — chosen at random (default)
--       'fixed'           — always the player in first_mover_seat
--       'game_default'    — delegated to the game engine
--
--   first_mover_seat  TEXT  (default: '0')
--     Seat index (0-based) of the player who goes first when first_mover_policy = 'fixed'.
--     Stored as TEXT to fit the generic KV schema. Ignored for all other policies.
--
--   rematch_first_mover_policy  TEXT  (default: 'random')
--     Controls who takes the first turn on rematches (second game onwards).
--     Values:
--       'random'          — chosen at random
--       'fixed'           — always the player in first_mover_seat
--       'game_default'    — delegated to the game engine
--       'winner_first'    — previous game winner goes first
--       'loser_first'     — previous game loser goes first
--       'winner_chooses'  — winner picks who goes first
--       'loser_chooses'   — loser picks who goes first