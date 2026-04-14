-- =============================================================================
-- BOT PROFILE: labelled difficulty / flavor for bot accounts
-- =============================================================================
-- Adds a nullable bot_profile column to players. Only rows with is_bot=TRUE
-- should carry a non-null profile, and a CHECK enforces the enum values.
--
-- The profile string drives:
--   - MCTS params in game-server (iterations, exploration, risk aversion)
--   - UI badge copy ("BOT · EASY")
--   - future leaderboard filters
-- It intentionally lives on players (not in a separate bot table) so the
-- same JOIN-less read path covers humans and bots.

ALTER TABLE players
    ADD COLUMN bot_profile TEXT;

ALTER TABLE players
    ADD CONSTRAINT players_bot_profile_check
    CHECK (
        bot_profile IS NULL
        OR bot_profile IN ('easy', 'medium', 'hard', 'aggressive')
    );

-- Humans must never carry a profile. Keeps the invariant enforced at the DB.
ALTER TABLE players
    ADD CONSTRAINT players_bot_profile_only_for_bots
    CHECK (is_bot = TRUE OR bot_profile IS NULL);

-- Backfill the seed bots created by tools/seed-test. The usernames carry
-- the profile by convention (bot_<profile>_1). Any matching row gets the
-- corresponding label; non-matching bots stay NULL and will be labelled
-- explicitly by whoever creates them.
UPDATE players SET bot_profile = 'easy'       WHERE is_bot = TRUE AND username = 'bot_easy_1';
UPDATE players SET bot_profile = 'medium'     WHERE is_bot = TRUE AND username = 'bot_medium_1';
UPDATE players SET bot_profile = 'hard'       WHERE is_bot = TRUE AND username = 'bot_hard_1';
UPDATE players SET bot_profile = 'aggressive' WHERE is_bot = TRUE AND username = 'bot_aggressive_1';
