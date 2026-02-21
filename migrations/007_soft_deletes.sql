-- Migration 007: Soft Deletes
-- Adds deleted_at column to tasks, teams, and profiles for soft-delete support

-- Add deleted_at to tasks
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX IF NOT EXISTS idx_tasks_deleted_at ON tasks(deleted_at) WHERE deleted_at IS NULL;

-- Add deleted_at to teams
ALTER TABLE teams ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX IF NOT EXISTS idx_teams_deleted_at ON teams(deleted_at) WHERE deleted_at IS NULL;

-- Add deleted_at to profiles
ALTER TABLE profiles ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX IF NOT EXISTS idx_profiles_deleted_at ON profiles(deleted_at) WHERE deleted_at IS NULL;

COMMENT ON COLUMN tasks.deleted_at IS 'Soft delete timestamp; NULL means active record';
COMMENT ON COLUMN teams.deleted_at IS 'Soft delete timestamp; NULL means active record';
COMMENT ON COLUMN profiles.deleted_at IS 'Soft delete timestamp; NULL means active record';
