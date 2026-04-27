-- =============================================================================
-- USER-SERVICE (Social & Moderation)
-- =============================================================================
CREATE SCHEMA IF NOT EXISTS users;

CREATE TABLE users.player_profiles (
    player_id  UUID         PRIMARY KEY REFERENCES public.players(id) ON DELETE CASCADE,
    bio        TEXT,
    country    CHAR(2),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE users.friendships (
    requester_id UUID         NOT NULL REFERENCES public.players(id) ON DELETE CASCADE,
    addressee_id UUID         NOT NULL REFERENCES public.players(id) ON DELETE CASCADE,
    status       TEXT         NOT NULL CHECK (status IN ('pending', 'accepted', 'blocked')),
    note         TEXT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (requester_id, addressee_id)
);

CREATE TABLE users.player_mutes (
    muter_id   UUID         NOT NULL REFERENCES public.players(id) ON DELETE CASCADE,
    muted_id   UUID         NOT NULL REFERENCES public.players(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (muter_id, muted_id)
);

CREATE TABLE users.bans (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id   UUID         NOT NULL REFERENCES public.players(id),
    banned_by   UUID         NOT NULL REFERENCES public.players(id),
    reason      TEXT,
    expires_at  TIMESTAMPTZ,
    lifted_at   TIMESTAMPTZ,
    lifted_by   UUID         REFERENCES public.players(id),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE users.player_reports (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id  UUID         NOT NULL REFERENCES public.players(id),
    reported_id  UUID         NOT NULL REFERENCES public.players(id),
    reason       TEXT         NOT NULL,
    context      JSONB        NOT NULL DEFAULT '{}',
    status       TEXT         NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'reviewed')),
    reviewed_by  UUID         REFERENCES public.players(id),
    reviewed_at  TIMESTAMPTZ,
    resolution   TEXT,
    ban_id       UUID         REFERENCES users.bans(id),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE users.player_achievements (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id        UUID         NOT NULL REFERENCES public.players(id) ON DELETE CASCADE,
    achievement_key  TEXT         NOT NULL,
    unlocked_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (player_id, achievement_key)
);

-- INDEXES
CREATE INDEX idx_friendships_requester_status ON users.friendships(requester_id, status);
CREATE INDEX idx_bans_player_active ON users.bans(player_id, created_at DESC) WHERE lifted_at IS NULL;
CREATE INDEX idx_player_reports_pending ON users.player_reports(status, created_at ASC) WHERE status = 'pending';