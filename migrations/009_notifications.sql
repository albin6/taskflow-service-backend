-- Migration 009: Persistent Notifications
-- Creates the notifications table to store user notifications durably

CREATE TABLE IF NOT EXISTS notifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    type        TEXT NOT NULL DEFAULT 'INFO',
    title       TEXT NOT NULL DEFAULT '',
    message     TEXT NOT NULL DEFAULT '',
    entity_id   TEXT,
    entity_type TEXT,
    is_read     BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id    ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_is_read    ON notifications(user_id, is_read);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC);

COMMENT ON TABLE  notifications            IS 'Persistent per-user notifications';
COMMENT ON COLUMN notifications.type       IS 'Notification type: INFO, SUCCESS, WARNING, ERROR';
COMMENT ON COLUMN notifications.entity_id  IS 'Optional ID of the related resource';
COMMENT ON COLUMN notifications.expires_at IS 'Optional expiry; NULL means notification never auto-expires';
