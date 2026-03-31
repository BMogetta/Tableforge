-- =============================================================================
-- 999_seed.sql — Development seed data
-- =============================================================================

-- ── Game Configurations ──────────────────────────────────────────────────────

INSERT INTO game_configs (game_id, default_timeout_secs, min_timeout_secs, max_timeout_secs, timeout_penalty)
VALUES ('tictactoe', 30, 10, 120, 'lose_game')
ON CONFLICT (game_id) DO NOTHING;

-- ── Owners ────────────────────────────────────────────────────────────────────

INSERT INTO allowed_emails (email, role, note)
VALUES ('brunomogetta@gmail.com', 'owner', 'project owner')
ON CONFLICT (email) DO NOTHING;

-- ── Players ───────────────────────────────────────────────────────────────────

INSERT INTO allowed_emails (email, role, note)
VALUES ('b.mogetta@hacken.io', 'player', 'dev player')
ON CONFLICT (email) DO NOTHING;