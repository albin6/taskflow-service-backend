-- Migration 008: Audit Log
-- Creates the audit_logs table for tracking all significant system actions

CREATE TABLE IF NOT EXISTS audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id    UUID REFERENCES profiles(id) ON DELETE SET NULL,
    actor_email TEXT NOT NULL DEFAULT '',
    action      TEXT NOT NULL,
    entity_type TEXT NOT NULL DEFAULT '',
    entity_id   TEXT NOT NULL DEFAULT '',
    old_value   JSONB,
    new_value   JSONB,
    ip_address  TEXT,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id    ON audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action       ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity_type  ON audit_logs(entity_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity_id    ON audit_logs(entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at   ON audit_logs(created_at DESC);

COMMENT ON TABLE  audit_logs             IS 'Immutable audit trail of all significant system actions';
COMMENT ON COLUMN audit_logs.actor_id    IS 'UUID of the user who performed the action';
COMMENT ON COLUMN audit_logs.action      IS 'Action code e.g. USER_APPROVED, TASK_DELETED';
COMMENT ON COLUMN audit_logs.entity_type IS 'Resource type e.g. task, team, user';
COMMENT ON COLUMN audit_logs.entity_id   IS 'UUID of the affected resource';
COMMENT ON COLUMN audit_logs.old_value   IS 'Snapshot of the resource before the action';
COMMENT ON COLUMN audit_logs.new_value   IS 'Snapshot of the resource after the action';
