-- 006_bot_players.sql
-- Adds is_bot flag to players table.
-- Fill bots (added via POST /rooms/{id}/bots) are created with is_bot = TRUE.
-- Named bots (future) will also use this flag.
-- Human players always have is_bot = FALSE (the default).

ALTER TABLE players ADD COLUMN is_bot BOOLEAN NOT NULL DEFAULT FALSE;