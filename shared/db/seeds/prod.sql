-- =============================================================================
-- prod.sql — Production seed data
-- =============================================================================
-- Idempotent. Re-runnable. ON CONFLICT DO NOTHING on every INSERT so the
-- migrator Job can run on every ArgoCD sync without breaking.
--
-- This file is the SOURCE OF TRUTH for who can log in to recess.bmogetta.com.
-- Adding a new owner / player → edit this file → commit → next sync applies.
--
-- DO NOT add @hacken.io or other dev-only emails here — those go in dev.sql.

-- ── Game configurations (production) ─────────────────────────────────────────
INSERT INTO game_configs (game_id, default_timeout_secs, min_timeout_secs, max_timeout_secs, timeout_penalty)
VALUES ('tictactoe', 30, 10, 120, 'lose_game')
ON CONFLICT (game_id) DO NOTHING;

-- ── Owners ───────────────────────────────────────────────────────────────────
INSERT INTO allowed_emails (email, role, note)
VALUES
    ('brunomogetta@gmail.com', 'owner', 'project owner'),
    ('patriarcaleandro@gmail.com', 'owner', 'project collaborator')
ON CONFLICT (email) DO NOTHING;
