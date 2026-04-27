-- =============================================================================
-- ACHIEVEMENTS: tier/progress support + notification type
-- =============================================================================

-- Add tier and progress columns to player_achievements.
-- Existing rows (flat unlocks) keep tier=1, progress=1.
ALTER TABLE users.player_achievements
    ADD COLUMN tier     INT NOT NULL DEFAULT 0,
    ADD COLUMN progress INT NOT NULL DEFAULT 0;

-- Backfill existing rows: they represent fully unlocked flat achievements.
UPDATE users.player_achievements SET tier = 1, progress = 1 WHERE tier = 0;

-- Add achievement_unlocked + friend_request_accepted to notification type CHECK.
-- Drop the old constraint and recreate with the full set.
ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_type_check;

ALTER TABLE notifications
    ADD CONSTRAINT notifications_type_check
    CHECK (type IN (
        'friend_request',
        'friend_request_accepted',
        'room_invitation',
        'ban_issued',
        'achievement_unlocked'
    ));
