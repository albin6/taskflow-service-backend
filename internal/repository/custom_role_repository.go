package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/taskflow/backend/internal/models"
)

type CustomRoleRepository struct {
	db *sql.DB
}

func NewCustomRoleRepository(db *sql.DB) *CustomRoleRepository {
	return &CustomRoleRepository{db: db}
}

func (r *CustomRoleRepository) Create(role *models.CustomRole) error {
	query := `
		INSERT INTO custom_roles (team_id, role_name, can_manage_tasks, can_view_analytics, can_manage_members_limited, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(
		query,
		role.TeamID,
		role.RoleName,
		role.CanManageTasks,
		role.CanViewAnalytics,
		role.CanManageMembersLimited,
		role.CreatedBy,
	).Scan(&role.ID, &role.CreatedAt, &role.UpdatedAt)
}

func (r *CustomRoleRepository) GetByID(id uuid.UUID) (*models.CustomRole, error) {
	var role models.CustomRole
	query := `SELECT * FROM custom_roles WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&role.ID,
		&role.TeamID,
		&role.RoleName,
		&role.CanManageTasks,
		&role.CanViewAnalytics,
		&role.CanManageMembersLimited,
		&role.CreatedBy,
		&role.CreatedAt,
		&role.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &role, err
}

func (r *CustomRoleRepository) GetByTeamID(teamID uuid.UUID) ([]models.CustomRole, error) {
	var roles []models.CustomRole
	query := `SELECT * FROM custom_roles WHERE team_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(query, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var role models.CustomRole
		err := rows.Scan(
			&role.ID,
			&role.TeamID,
			&role.RoleName,
			&role.CanManageTasks,
			&role.CanViewAnalytics,
			&role.CanManageMembersLimited,
			&role.CreatedBy,
			&role.CreatedAt,
			&role.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *CustomRoleRepository) Update(id uuid.UUID, req *models.UpdateCustomRoleRequest) error {
	updates := []string{}
	args := []interface{}{}
	argCount := 1

	if req.RoleName != nil {
		updates = append(updates, fmt.Sprintf("role_name = $%d", argCount))
		args = append(args, *req.RoleName)
		argCount++
	}
	if req.CanManageTasks != nil {
		updates = append(updates, fmt.Sprintf("can_manage_tasks = $%d", argCount))
		args = append(args, *req.CanManageTasks)
		argCount++
	}
	if req.CanViewAnalytics != nil {
		updates = append(updates, fmt.Sprintf("can_view_analytics = $%d", argCount))
		args = append(args, *req.CanViewAnalytics)
		argCount++
	}
	if req.CanManageMembersLimited != nil {
		updates = append(updates, fmt.Sprintf("can_manage_members_limited = $%d", argCount))
		args = append(args, *req.CanManageMembersLimited)
		argCount++
	}

	if len(updates) == 0 {
		return nil
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE custom_roles SET %s WHERE id = $%d",
		joinStrings(updates, ", "), argCount)

	_, err := r.db.Exec(query, args...)
	return err
}

func (r *CustomRoleRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM custom_roles WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *CustomRoleRepository) AssignRoleToMember(teamID, userID uuid.UUID, customRoleID *uuid.UUID) error {
	query := `UPDATE team_members SET custom_role_id = $1 WHERE team_id = $2 AND user_id = $3`
	_, err := r.db.Exec(query, customRoleID, teamID, userID)
	return err
}

func (r *CustomRoleRepository) UpdateMemberPosition(teamID, userID uuid.UUID, position string) error {
	query := `UPDATE team_members SET position = $1 WHERE team_id = $2 AND user_id = $3`
	_, err := r.db.Exec(query, position, teamID, userID)
	return err
}

func (r *CustomRoleRepository) GetMemberRole(teamID, userID uuid.UUID) (*models.CustomRole, error) {
	var role models.CustomRole
	query := `
		SELECT cr.* FROM custom_roles cr
		JOIN team_members tm ON tm.custom_role_id = cr.id
		WHERE tm.team_id = $1 AND tm.user_id = $2
	`
	err := r.db.QueryRow(query, teamID, userID).Scan(
		&role.ID,
		&role.TeamID,
		&role.RoleName,
		&role.CanManageTasks,
		&role.CanViewAnalytics,
		&role.CanManageMembersLimited,
		&role.CreatedBy,
		&role.CreatedAt,
		&role.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &role, err
}

// Helper function to join strings
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
