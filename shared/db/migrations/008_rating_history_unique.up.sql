-- =============================================================================
-- 008_rating_history_unique.up.sql
--
-- Enforce one rating_history row per (session, player) so that re-delivered
-- game.session.finished events cannot double-apply deltas. The column is
-- backfilled via the game_results FK chain before being made NOT NULL.
-- =============================================================================

ALTER TABLE ratings.rating_history
    ADD COLUMN session_id UUID;

UPDATE ratings.rating_history h
SET session_id = gr.session_id
FROM public.game_results gr
WHERE gr.id = h.result_id
  AND h.session_id IS NULL;

ALTER TABLE ratings.rating_history
    ALTER COLUMN session_id SET NOT NULL;

ALTER TABLE ratings.rating_history
    ADD CONSTRAINT rating_history_session_player_uniq UNIQUE (session_id, player_id);
