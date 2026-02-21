-- Update can_assign_task function to support custom roles
CREATE OR REPLACE FUNCTION can_assign_task(
  _assigner_id UUID,
  _assignee_id UUID,
  _team_id UUID
) RETURNS BOOLEAN AS $$
DECLARE
  assigner_is_admin BOOLEAN;
  assigner_is_head BOOLEAN;
  assigner_is_lead BOOLEAN;
  assigner_can_manage BOOLEAN;
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
  
  -- Check team membership and roles for assigner
  SELECT 
    tm.is_head, 
    tm.is_lead,
    (cr.can_manage_tasks OR cr.can_manage_members_limited)
  INTO assigner_is_head, assigner_is_lead, assigner_can_manage
  FROM team_members tm
  LEFT JOIN custom_roles cr ON tm.custom_role_id = cr.id
  WHERE tm.user_id = _assigner_id AND tm.team_id = _team_id;
  
  -- Check team membership and roles for assignee
  SELECT is_head, is_lead INTO assignee_is_head, assignee_is_lead
  FROM team_members
  WHERE user_id = _assignee_id AND team_id = _team_id;
  
  -- If assigner is not in the team, they cannot assign
  IF assigner_is_head IS NULL AND assigner_is_lead IS NULL AND assigner_can_manage IS NULL THEN
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
  
  -- Team Lead OR Custom Role with permission can assign to members only (not to heads or other leads)
  IF assigner_is_lead OR assigner_can_manage THEN
    RETURN NOT (assignee_is_head OR assignee_is_lead);
  END IF;
  
  -- Regular members cannot assign tasks
  RETURN FALSE;
END;
$$ LANGUAGE plpgsql STABLE;

-- Update get_assignable_users function to support custom roles
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
  user_can_manage BOOLEAN;
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
  SELECT 
    tm.is_head, 
    tm.is_lead,
    (cr.can_manage_tasks OR cr.can_manage_members_limited)
  INTO user_is_head, user_is_lead, user_can_manage
  FROM team_members tm
  LEFT JOIN custom_roles cr ON tm.custom_role_id = cr.id
  WHERE tm.user_id = _user_id AND tm.team_id = _team_id;
  
  -- If user is not in the team, return empty
  IF user_is_head IS NULL AND user_is_lead IS NULL AND user_can_manage IS NULL THEN
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
  
  -- Team Lead OR Custom Role with permission can assign to members only
  IF user_is_lead OR user_can_manage THEN
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
  
  -- Regular members cannot assign
  RETURN;
END;
$$ LANGUAGE plpgsql STABLE;
