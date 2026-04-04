-- Partial index for fast lookup of active sessions per player.
CREATE INDEX IF NOT EXISTS idx_player_sessions_player_id
  ON player_sessions(player_id) WHERE revoked_at IS NULL;
