-- =============================================================================
-- ADMIN SCHEMA
-- =============================================================================
CREATE SCHEMA IF NOT EXISTS admin;

CREATE TABLE admin.audit_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id UUID NOT NULL REFERENCES players(id),
  action TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL,
  details JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_logs_actor ON admin.audit_logs(actor_id);
CREATE INDEX idx_audit_logs_created ON admin.audit_logs(created_at DESC);
