-- =============================================================================
-- 009_notifications_source_event_id.up.sql
--
-- Dedupe notifications by the Redis event that spawned them so re-delivered
-- friendship.requested / friendship.accepted / player.banned /
-- achievement.unlocked events cannot produce duplicate inbox rows.
--
-- The unique index is partial: historical rows and any internal-API creations
-- without an event origin leave source_event_id NULL and are unaffected.
-- =============================================================================

ALTER TABLE notifications
    ADD COLUMN source_event_id UUID;

CREATE UNIQUE INDEX notifications_player_source_event_uniq
    ON notifications (player_id, source_event_id)
    WHERE source_event_id IS NOT NULL;
