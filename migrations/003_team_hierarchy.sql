-- Team Hierarchy Migration
-- Adds support for teams with Team Heads, Team Leads, and custom roles

-- Create teams table
CREATE TABLE IF NOT EXISTS teams (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  description TEXT,
  created_by UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create index on created_by for faster lookups
CREATE INDEX IF NOT EXISTS idx_teams_created_by ON teams(created_by);

-- Create team_roles table for custom roles within teams
CREATE TABLE IF NOT EXISTS team_roles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
  role_name TEXT NOT NULL,
  permissions JSONB DEFAULT '[]'::jsonb,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  UNIQUE (team_id, role_name)
);

-- Create index on team_id for faster role lookups
CREATE INDEX IF NOT EXISTS idx_team_roles_team_id ON team_roles(team_id);

-- Create team_members table linking users to teams with roles
CREATE TABLE IF NOT EXISTS team_members (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
  team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
  role_id UUID REFERENCES team_roles(id) ON DELETE SET NULL,
  is_head BOOLEAN NOT NULL DEFAULT false,
  is_lead BOOLEAN NOT NULL DEFAULT false,
  joined_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, team_id)
);

-- Create indexes for team_members
CREATE INDEX IF NOT EXISTS idx_team_members_user_id ON team_members(user_id);
CREATE INDEX IF NOT EXISTS idx_team_members_team_id ON team_members(team_id);
CREATE INDEX IF NOT EXISTS idx_team_members_role_id ON team_members(role_id);
CREATE INDEX IF NOT EXISTS idx_team_members_is_head ON team_members(is_head);
CREATE INDEX IF NOT EXISTS idx_team_members_is_lead ON team_members(is_lead);

-- Add team_id to tasks table to associate tasks with teams
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='tasks' AND column_name='team_id') THEN
        ALTER TABLE tasks ADD COLUMN team_id UUID REFERENCES teams(id) ON DELETE SET NULL;
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_tasks_team_id ON tasks(team_id);

-- Create trigger for teams updated_at
DROP TRIGGER IF EXISTS update_teams_updated_at ON teams;
CREATE TRIGGER update_teams_updated_at
  BEFORE UPDATE ON teams
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();

-- Function to check if user can assign task to another user based on team hierarchy
CREATE OR REPLACE FUNCTION can_assign_task(
  _assigner_id UUID,
  _assignee_id UUID,
  _team_id UUID
) RETURNS BOOLEAN AS $$
DECLARE
  assigner_is_admin BOOLEAN;
  assigner_is_head BOOLEAN;
  assigner_is_lead BOOLEAN;
  assignee_is_head BOOLEAN;
  assignee_is_lead BOOLEAN;
BEGIN
  -- Check if assigner is admin
  SELECT EXISTS (
    SELECT 1 FROM user_roles 
    WHERE user_id = _assigner_id AND role = 'admin'
  ) INTO assigner_is_admin;
  
  -- Admins can assign to anyone
  IF assigner_is_admin THEN
    RETURN TRUE;
  END IF;
  
  -- Check team membership and roles
  SELECT is_head, is_lead INTO assigner_is_head, assigner_is_lead
  FROM team_members
  WHERE user_id = _assigner_id AND team_id = _team_id;
  
  SELECT is_head, is_lead INTO assignee_is_head, assignee_is_lead
  FROM team_members
  WHERE user_id = _assignee_id AND team_id = _team_id;
  
  -- If assigner is not in the team, they cannot assign
  IF assigner_is_head IS NULL AND assigner_is_lead IS NULL THEN
    RETURN FALSE;
  END IF;
  
  -- If assignee is not in the team, cannot assign
  IF assignee_is_head IS NULL AND assignee_is_lead IS NULL THEN
    RETURN FALSE;
  END IF;
  
  -- Team Head can assign to anyone in the team
  IF assigner_is_head THEN
    RETURN TRUE;
  END IF;
  
  -- Team Lead can assign to members only (not to heads or other leads)
  IF assigner_is_lead THEN
    RETURN NOT (assignee_is_head OR assignee_is_lead);
  END IF;
  
  -- Regular members cannot assign tasks
  RETURN FALSE;
END;
$$ LANGUAGE plpgsql STABLE;

-- Function to get assignable users for a team member
CREATE OR REPLACE FUNCTION get_assignable_users(
  _user_id UUID,
  _team_id UUID
) RETURNS TABLE (
  user_id UUID,
  email TEXT,
  full_name TEXT,
  is_head BOOLEAN,
  is_lead BOOLEAN
) AS $$
DECLARE
  user_is_admin BOOLEAN;
  user_is_head BOOLEAN;
  user_is_lead BOOLEAN;
BEGIN
  -- Check if user is admin
  SELECT EXISTS (
    SELECT 1 FROM user_roles 
    WHERE user_roles.user_id = _user_id AND role = 'admin'
  ) INTO user_is_admin;
  
  -- Admins can assign to anyone in the team
  IF user_is_admin THEN
    RETURN QUERY
    SELECT 
      tm.user_id,
      p.email,
      p.full_name,
      tm.is_head,
      tm.is_lead
    FROM team_members tm
    JOIN profiles p ON p.id = tm.user_id
    WHERE tm.team_id = _team_id;
    RETURN;
  END IF;
  
  -- Check user's role in the team
  SELECT tm.is_head, tm.is_lead INTO user_is_head, user_is_lead
  FROM team_members tm
  WHERE tm.user_id = _user_id AND tm.team_id = _team_id;
  
  -- If user is not in the team, return empty
  IF user_is_head IS NULL AND user_is_lead IS NULL THEN
    RETURN;
  END IF;
  
  -- Team Head can assign to anyone in the team
  IF user_is_head THEN
    RETURN QUERY
    SELECT 
      tm.user_id,
      p.email,
      p.full_name,
      tm.is_head,
      tm.is_lead
    FROM team_members tm
    JOIN profiles p ON p.id = tm.user_id
    WHERE tm.team_id = _team_id;
    RETURN;
  END IF;
  
  -- Team Lead can assign to members only
  IF user_is_lead THEN
    RETURN QUERY
    SELECT 
      tm.user_id,
      p.email,
      p.full_name,
      tm.is_head,
      tm.is_lead
    FROM team_members tm
    JOIN profiles p ON p.id = tm.user_id
    WHERE tm.team_id = _team_id 
      AND NOT tm.is_head 
      AND NOT tm.is_lead;
    RETURN;
  END IF;
  
  -- Regular members can assign to other regular members in their team
  RETURN QUERY
  SELECT 
    tm.user_id,
    p.email,
    p.full_name,
    tm.is_head,
    tm.is_lead
  FROM team_members tm
  JOIN profiles p ON p.id = tm.user_id
  WHERE tm.team_id = _team_id
    AND NOT tm.is_head 
    AND NOT tm.is_lead;
  RETURN;
END;
$$ LANGUAGE plpgsql STABLE;

-- Add comments for documentation
COMMENT ON TABLE teams IS 'Teams for organizing users into groups';
COMMENT ON TABLE team_roles IS 'Custom roles within teams with specific permissions';
COMMENT ON TABLE team_members IS 'User membership in teams with role assignments';
COMMENT ON FUNCTION can_assign_task IS 'Validates if a user can assign a task to another user based on team hierarchy';
COMMENT ON FUNCTION get_assignable_users IS 'Returns list of users that a team member can assign tasks to';
