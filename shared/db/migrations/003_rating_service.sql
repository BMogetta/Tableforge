-- =============================================================================
-- RATING-SERVICE (Elo & Rankings)
-- =============================================================================
CREATE SCHEMA IF NOT EXISTS ratings;

CREATE TABLE ratings.ratings (
    player_id      UUID         NOT NULL REFERENCES public.players(id) ON DELETE CASCADE,
    game_id        TEXT         NOT NULL,
    mmr            FLOAT        NOT NULL DEFAULT 1500,
    display_rating FLOAT        NOT NULL DEFAULT 1500,
    games_played   INT          NOT NULL DEFAULT 0,
    win_streak     INT          NOT NULL DEFAULT 0,
    loss_streak    INT          NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (player_id, game_id)
);

CREATE TABLE ratings.rating_history (
    id          BIGSERIAL    PRIMARY KEY,
    player_id   UUID         NOT NULL REFERENCES public.players(id),
    game_id     TEXT         NOT NULL,
    result_id   UUID         NOT NULL REFERENCES public.game_results(id),
    mmr_before  FLOAT        NOT NULL,
    mmr_after   FLOAT        NOT NULL,
    delta       FLOAT        NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- INDEXES
CREATE INDEX idx_ratings_mmr ON ratings.ratings(game_id, mmr DESC);
CREATE INDEX idx_ratings_display ON ratings.ratings(game_id, display_rating DESC);
CREATE INDEX idx_rating_history_player_game ON ratings.rating_history(player_id, game_id, created_at DESC);
CREATE INDEX idx_rating_history_result ON ratings.rating_history(result_id);