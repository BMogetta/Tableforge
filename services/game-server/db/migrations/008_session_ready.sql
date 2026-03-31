-- Session ready handshake — tracks which players have confirmed asset load.
-- The game does not start (TurnTimer does not fire) until all human players
-- have sent player_ready or the 60-second timeout expires.

ALTER TABLE game_sessions
    ADD COLUMN ready_players TEXT[] NOT NULL DEFAULT '{}';