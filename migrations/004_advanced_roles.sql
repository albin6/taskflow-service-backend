-- Migration 004: Advanced Roles and Team Registration
-- Adds requested_role and requested_team_id to profiles for hierarchical approval

ALTER TABLE profiles ADD COLUMN IF NOT EXISTS requested_role TEXT;
ALTER TABLE profiles ADD COLUMN IF NOT EXISTS requested_team_id UUID REFERENCES teams(id) ON DELETE SET NULL;

-- Create index for requested_team_id for faster lookups during approval
CREATE INDEX IF NOT EXISTS idx_profiles_requested_team_id ON profiles(requested_team_id);

COMMENT ON COLUMN profiles.requested_role IS 'The role the user requested during signup (admin, team_lead, team_member)';
COMMENT ON COLUMN profiles.requested_team_id IS 'The team the user requested to join during signup';
