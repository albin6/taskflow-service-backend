package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"context"
	"time"

	"github.com/google/uuid"
	"github.com/taskflow/backend/internal/cache"
	"github.com/taskflow/backend/internal/models"
)

type TeamRepository struct {
	db    *sql.DB
	cache *cache.RedisClient
}

func NewTeamRepository(db *sql.DB, cache *cache.RedisClient) *TeamRepository {
	return &TeamRepository{db: db, cache: cache}
}

func (r *TeamRepository) invalidateUserTeams(userID uuid.UUID) {
	if r.cache != nil {
		r.cache.Client.Del(context.Background(), fmt.Sprintf("user:%s:teams", userID.String()))
	}
}

// CreateTeam creates a new team
func (r *TeamRepository) CreateTeam(name string, description *string, createdBy uuid.UUID) (*models.Team, error) {
	team := &models.Team{}
	query := `
		INSERT INTO teams (name, description, created_by)
		VALUES ($1, $2, $3)
		RETURNING id, name, description, created_by, created_at, updated_at
	`
	err := r.db.QueryRow(query, name, description, createdBy).Scan(
		&team.ID, &team.Name, &team.Description, &team.CreatedBy, &team.CreatedAt, &team.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create team: %w", err)
	}
	return team, nil
}

// GetTeam retrieves a team by ID
func (r *TeamRepository) GetTeam(teamID uuid.UUID) (*models.Team, error) {
	team := &models.Team{}
	query := `SELECT id, name, description, created_by, created_at, updated_at FROM teams WHERE id = $1 AND deleted_at IS NULL`
	err := r.db.QueryRow(query, teamID).Scan(
		&team.ID, &team.Name, &team.Description, &team.CreatedBy, &team.CreatedAt, &team.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	return team, nil
}

// GetTeamWithMembers retrieves a team with all its members
func (r *TeamRepository) GetTeamWithMembers(teamID uuid.UUID) (*models.TeamWithMembers, error) {
	team, err := r.GetTeam(teamID)
	if err != nil || team == nil {
		return nil, err
	}

	members, err := r.GetTeamMembers(teamID)
	if err != nil {
		return nil, err
	}

	return &models.TeamWithMembers{
		Team:    *team,
		Members: members,
	}, nil
}

// ListTeams retrieves all teams
func (r *TeamRepository) ListTeams() ([]models.Team, error) {
	var teams []models.Team
	query := `SELECT id, name, description, created_by, created_at, updated_at FROM teams WHERE deleted_at IS NULL ORDER BY created_at DESC`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var team models.Team
		if err := rows.Scan(&team.ID, &team.Name, &team.Description, &team.CreatedBy, &team.CreatedAt, &team.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan team: %w", err)
		}
		teams = append(teams, team)
	}

	return teams, nil
}

func (r *TeamRepository) ListAll() ([]models.Team, error) {
	return r.ListTeams()
}

// ListUserTeams retrieves all teams a user belongs to
func (r *TeamRepository) ListUserTeams(userID uuid.UUID) ([]models.UserTeamMembership, error) {
	// Try cache first
	if r.cache != nil {
		ctx := context.Background()
		val, err := r.cache.Client.Get(ctx, fmt.Sprintf("user:%s:teams", userID.String())).Result()
		if err == nil {
			var memberships []models.UserTeamMembership
			if err := json.Unmarshal([]byte(val), &memberships); err == nil {
				return memberships, nil
			}
		}
	}

	var memberships []models.UserTeamMembership
	query := `
		SELECT 
			t.id as team_id,
			t.name as team_name,
			tm.is_head,
			tm.is_lead,
			COALESCE(cr.role_name, tr.role_name) as role_name,
			COALESCE(cr.can_manage_tasks, false) as can_manage_tasks,
			COALESCE(cr.can_view_analytics, false) as can_view_analytics,
			COALESCE(cr.can_manage_members_limited, false) as can_manage_members_limited
		FROM team_members tm
		JOIN teams t ON t.id = tm.team_id AND t.deleted_at IS NULL
		LEFT JOIN team_roles tr ON tr.id = tm.role_id
		LEFT JOIN custom_roles cr ON cr.id = tm.custom_role_id
		WHERE tm.user_id = $1
		ORDER BY t.name
	`
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user teams: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var m models.UserTeamMembership
		if err := rows.Scan(
			&m.TeamID, &m.TeamName, &m.IsHead, &m.IsLead, &m.RoleName,
			&m.CanManageTasks, &m.CanViewAnalytics, &m.CanManageMembersLimited,
		); err != nil {
			return nil, fmt.Errorf("failed to scan membership: %w", err)
		}
		memberships = append(memberships, m)
	}

	if r.cache != nil {
		data, _ := json.Marshal(memberships)
		r.cache.Client.Set(context.Background(), fmt.Sprintf("user:%s:teams", userID.String()), data, 30*time.Minute)
	}

	return memberships, nil
}

// UpdateTeam updates team information
func (r *TeamRepository) UpdateTeam(teamID uuid.UUID, name *string, description *string) error {
	query := `UPDATE teams SET `
	args := []interface{}{}
	argCount := 1

	if name != nil {
		query += fmt.Sprintf("name = $%d, ", argCount)
		args = append(args, *name)
		argCount++
	}

	if description != nil {
		query += fmt.Sprintf("description = $%d, ", argCount)
		args = append(args, *description)
		argCount++
	}

	if len(args) == 0 {
		return nil // Nothing to update
	}

	// Remove trailing comma and space, add WHERE clause
	query = query[:len(query)-2] + fmt.Sprintf(" WHERE id = $%d", argCount)
	args = append(args, teamID)

	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update team: %w", err)
	}
	return nil
}

// DeleteTeam deletes a team
func (r *TeamRepository) DeleteTeam(teamID uuid.UUID) error {
	query := `UPDATE teams SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.db.Exec(query, teamID)
	if err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}
	return nil
}

// AddMember adds a user to a team with a role
func (r *TeamRepository) AddMember(teamID, userID uuid.UUID, roleID *uuid.UUID, isHead, isLead bool) error {
	query := `
		INSERT INTO team_members (user_id, team_id, role_id, is_head, is_lead)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(query, userID, teamID, roleID, isHead, isLead)
	if err != nil {
		return fmt.Errorf("failed to add member to team: %w", err)
	}
	r.invalidateUserTeams(userID)
	return nil
}

// RemoveMember removes a user from a team
func (r *TeamRepository) RemoveMember(teamID, userID uuid.UUID) error {
	query := `DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`
	_, err := r.db.Exec(query, teamID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove member from team: %w", err)
	}
	r.invalidateUserTeams(userID)
	return nil
}

// UpdateMemberRole updates a team member's role
func (r *TeamRepository) UpdateMemberRole(teamID, userID uuid.UUID, roleID *uuid.UUID, isHead, isLead *bool) error {
	query := `UPDATE team_members SET `
	args := []interface{}{}
	argCount := 1

	if roleID != nil {
		query += fmt.Sprintf("role_id = $%d, ", argCount)
		args = append(args, *roleID)
		argCount++
	}

	if isHead != nil {
		query += fmt.Sprintf("is_head = $%d, ", argCount)
		args = append(args, *isHead)
		argCount++
	}

	if isLead != nil {
		query += fmt.Sprintf("is_lead = $%d, ", argCount)
		args = append(args, *isLead)
		argCount++
	}

	if len(args) == 0 {
		return nil // Nothing to update
	}

	// Remove trailing comma and space, add WHERE clause
	query = query[:len(query)-2] + fmt.Sprintf(" WHERE team_id = $%d AND user_id = $%d", argCount, argCount+1)
	args = append(args, teamID, userID)

	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}
	r.invalidateUserTeams(userID)
	return nil
}

// GetTeamMembers retrieves all members of a team with their details
func (r *TeamRepository) GetTeamMembers(teamID uuid.UUID) ([]models.TeamMemberDetail, error) {
	var members []models.TeamMemberDetail
	query := `
		SELECT 
			tm.id, tm.user_id, tm.team_id, tm.custom_role_id, tm.is_head, tm.is_lead, tm.joined_at,
			p.email, p.full_name, p.avatar_url,
			cr.role_name,
			cr.id as custom_role_pk, cr.team_id as custom_role_team_id, cr.role_name as custom_role_name, cr.can_manage_tasks, cr.can_manage_members_limited
		FROM team_members tm
		JOIN profiles p ON p.id = tm.user_id
		LEFT JOIN custom_roles cr ON cr.id = tm.custom_role_id
		WHERE tm.team_id = $1
		ORDER BY tm.is_head DESC, tm.is_lead DESC, p.full_name
	`
	rows, err := r.db.Query(query, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var m models.TeamMemberDetail
		var customRolePK, customRoleTeamID *uuid.UUID
		var customRoleName *string
		var canManageTasks, canManageMembersLimited *bool

		if err := rows.Scan(
			&m.ID, &m.UserID, &m.TeamID, &m.CustomRoleID, &m.IsHead, &m.IsLead, &m.JoinedAt,
			&m.Email, &m.FullName, &m.AvatarURL, &m.RoleName,
			&customRolePK, &customRoleTeamID, &customRoleName, &canManageTasks, &canManageMembersLimited,
		); err != nil {
			return nil, fmt.Errorf("failed to scan member: %w", err)
		}

		// Populate CustomRole object if custom role is assigned
		if customRolePK != nil && customRoleName != nil && customRoleTeamID != nil {
			m.CustomRole = &models.CustomRole{
				ID:                      *customRolePK,
				TeamID:                  *customRoleTeamID,
				RoleName:                *customRoleName,
				CanManageTasks:          canManageTasks != nil && *canManageTasks,
				CanManageMembersLimited: canManageMembersLimited != nil && *canManageMembersLimited,
			}
		}

		members = append(members, m)
	}

	return members, nil
}

// CreateCustomRole creates a custom role for a team
func (r *TeamRepository) CreateCustomRole(teamID uuid.UUID, roleName string, permissions []string) (*models.TeamRole, error) {
	role := &models.TeamRole{}

	// Convert permissions to JSON
	permissionsJSON, err := json.Marshal(permissions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal permissions: %w", err)
	}

	query := `
		INSERT INTO team_roles (team_id, role_name, permissions)
		VALUES ($1, $2, $3)
		RETURNING id, team_id, role_name, permissions, created_at
	`

	var permissionsStr string
	err = r.db.QueryRow(query, teamID, roleName, permissionsJSON).Scan(
		&role.ID, &role.TeamID, &role.RoleName, &permissionsStr, &role.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create custom role: %w", err)
	}

	// Parse permissions back
	if err := json.Unmarshal([]byte(permissionsStr), &role.Permissions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
	}

	return role, nil
}

// GetTeamRoles retrieves all custom roles for a team
func (r *TeamRepository) GetTeamRoles(teamID uuid.UUID) ([]models.TeamRole, error) {
	var roles []models.TeamRole
	query := `SELECT id, team_id, role_name, permissions, created_at FROM team_roles WHERE team_id = $1`

	rows, err := r.db.Query(query, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team roles: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var role models.TeamRole
		var permissionsStr string

		if err := rows.Scan(&role.ID, &role.TeamID, &role.RoleName, &permissionsStr, &role.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}

		if err := json.Unmarshal([]byte(permissionsStr), &role.Permissions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
		}

		roles = append(roles, role)
	}

	return roles, nil
}

// GetAssignableUsers returns users that a team member can assign tasks to
func (r *TeamRepository) GetAssignableUsers(userID, teamID uuid.UUID) ([]models.AssignableUser, error) {
	var users []models.AssignableUser
	query := `SELECT user_id, email, full_name, is_head, is_lead FROM get_assignable_users($1, $2)`
	rows, err := r.db.Query(query, userID, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assignable users: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var u models.AssignableUser
		if err := rows.Scan(&u.UserID, &u.Email, &u.FullName, &u.IsHead, &u.IsLead); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}

	return users, nil
}

// CanAssignTask checks if a user can assign a task to another user
func (r *TeamRepository) CanAssignTask(assignerID, assigneeID, teamID uuid.UUID) (bool, error) {
	var canAssign bool
	query := `SELECT can_assign_task($1, $2, $3)`
	err := r.db.QueryRow(query, assignerID, assigneeID, teamID).Scan(&canAssign)
	if err != nil {
		return false, fmt.Errorf("failed to check assignment permission: %w", err)
	}
	return canAssign, nil
}

// IsMemberOfTeam checks if a user is a member of a team
func (r *TeamRepository) IsMemberOfTeam(userID, teamID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM team_members WHERE user_id = $1 AND team_id = $2)`
	err := r.db.QueryRow(query, userID, teamID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check team membership: %w", err)
	}
	return exists, nil
}

// AreBothRegularMembers checks if both users are regular members (not heads or leads) of the same team
func (r *TeamRepository) AreBothRegularMembers(userID1, userID2, teamID uuid.UUID) (bool, error) {
	var areBothRegular bool
	query := `
		SELECT EXISTS (
			SELECT 1 
			FROM team_members tm1
			JOIN team_members tm2 ON tm1.team_id = tm2.team_id
			WHERE tm1.user_id = $1 
				AND tm2.user_id = $2 
				AND tm1.team_id = $3
				AND tm2.team_id = $3
				AND NOT tm1.is_head 
				AND NOT tm1.is_lead
				AND NOT tm2.is_head 
				AND NOT tm2.is_lead
		)
	`
	err := r.db.QueryRow(query, userID1, userID2, teamID).Scan(&areBothRegular)
	if err != nil {
		return false, fmt.Errorf("failed to check if both are regular members: %w", err)
	}
	return areBothRegular, nil
}

// GetMemberRole retrieves a user's role in a team
func (r *TeamRepository) GetMemberRole(userID, teamID uuid.UUID) (*models.TeamMember, error) {
	member := &models.TeamMember{}
	query := `SELECT id, user_id, team_id, role_id, is_head, is_lead, joined_at 
	          FROM team_members WHERE user_id = $1 AND team_id = $2`
	err := r.db.QueryRow(query, userID, teamID).Scan(
		&member.ID, &member.UserID, &member.TeamID, &member.RoleID, &member.IsHead, &member.IsLead, &member.JoinedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get member role: %w", err)
	}
	return member, nil
}

// GetTeamApprovers returns user IDs who can approve team member requests for a team
// Priority: Team Lead > Team Head
// Returns empty slice if neither exists (admin will handle)
func (r *TeamRepository) GetTeamApprovers(teamID uuid.UUID) ([]uuid.UUID, error) {
	var approvers []uuid.UUID

	// First, try to find team leads
	leadQuery := `SELECT user_id FROM team_members WHERE team_id = $1 AND is_lead = true`
	rows, err := r.db.Query(leadQuery, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to query team leads: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("failed to scan team lead: %w", err)
		}
		approvers = append(approvers, userID)
	}

	// If we found team leads, return them
	if len(approvers) > 0 {
		return approvers, nil
	}

	// No team leads found, try to find team heads
	headQuery := `SELECT user_id FROM team_members WHERE team_id = $1 AND is_head = true`
	rows, err = r.db.Query(headQuery, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to query team heads: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("failed to scan team head: %w", err)
		}
		approvers = append(approvers, userID)
	}

	// Return team heads if found, or empty slice if neither lead nor head exists
	return approvers, nil
}

// GetUserManagedTeams returns all teams where the user has task management permissions
func (r *TeamRepository) GetUserManagedTeams(userID uuid.UUID) ([]uuid.UUID, error) {
	query := `
		SELECT DISTINCT tm.team_id
		FROM team_members tm
		LEFT JOIN custom_roles cr ON cr.id = tm.custom_role_id
		WHERE tm.user_id = $1
		AND (tm.is_head = true OR tm.is_lead = true OR cr.can_manage_tasks = true)
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query managed teams: %w", err)
	}
	defer rows.Close()

	var teamIDs []uuid.UUID
	for rows.Next() {
		var teamID uuid.UUID
		if err := rows.Scan(&teamID); err != nil {
			return nil, fmt.Errorf("failed to scan team ID: %w", err)
		}
		teamIDs = append(teamIDs, teamID)
	}

	return teamIDs, nil
}
