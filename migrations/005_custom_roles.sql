-- Enable uuid-ossp extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create custom_roles table
CREATE TABLE IF NOT EXISTS custom_roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    role_name VARCHAR(100) NOT NULL,
    can_manage_tasks BOOLEAN DEFAULT false,
    can_view_analytics BOOLEAN DEFAULT false,
    can_manage_members_limited BOOLEAN DEFAULT false,
    created_by UUID REFERENCES profiles(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(team_id, role_name)
);

-- Add custom_role_id and position to team_members
ALTER TABLE team_members 
ADD COLUMN IF NOT EXISTS custom_role_id UUID REFERENCES custom_roles(id) ON DELETE SET NULL,
ADD COLUMN IF NOT EXISTS position VARCHAR(100);

-- Create index for faster lookups
CREATE INDEX IF NOT EXISTS idx_custom_roles_team_id ON custom_roles(team_id);
CREATE INDEX IF NOT EXISTS idx_team_members_custom_role ON team_members(custom_role_id);

-- Add updated_at trigger for custom_roles
CREATE OR REPLACE FUNCTION update_custom_roles_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER custom_roles_updated_at
    BEFORE UPDATE ON custom_roles
    FOR EACH ROW
    EXECUTE FUNCTION update_custom_roles_updated_at();
