-- 004_notifications.sql
-- Persistent inbox notifications for friend requests, room invitations, and bans.
-- Notifications are never deleted. read_at tracks read/unread state.
-- action_taken and action_expires_at support inline accept/decline actions.

CREATE TABLE notifications (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id         UUID        NOT NULL REFERENCES players(id),
    type              TEXT        NOT NULL CHECK (type IN ('friend_request', 'room_invitation', 'ban_issued')),
    payload           JSONB       NOT NULL DEFAULT '{}',
    action_taken      TEXT        CHECK (action_taken IN ('accepted', 'declined')),
    action_expires_at TIMESTAMPTZ,
    read_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fast lookup for unread count and unread list per player.
CREATE INDEX notifications_player_unread
    ON notifications(player_id, created_at DESC)
    WHERE read_at IS NULL;

-- General listing (read + unread filtered by age in application layer).
CREATE INDEX notifications_player_created
    ON notifications(player_id, created_at DESC);