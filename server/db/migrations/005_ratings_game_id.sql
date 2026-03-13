-- Adds game_id to the ratings table so leaderboards are scoped per game.
-- Drops the player_id-only primary key and replaces it with a composite
-- (player_id, game_id) so a player can hold one rating entry per game.

ALTER TABLE ratings ADD COLUMN game_id TEXT NOT NULL DEFAULT 'tictactoe';

ALTER TABLE ratings DROP CONSTRAINT ratings_pkey;
ALTER TABLE ratings ADD PRIMARY KEY (player_id, game_id);

DROP INDEX idx_ratings_mmr;
CREATE INDEX idx_ratings_mmr     ON ratings(game_id, mmr DESC);
CREATE INDEX idx_ratings_display ON ratings(game_id, display_rating DESC);