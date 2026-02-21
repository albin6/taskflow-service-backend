package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/taskflow/backend/internal/models"
)

type TaskRepository struct {
	db *sql.DB
}

func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(task *models.Task) error {
	task.ID = uuid.New()
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	query := `
		INSERT INTO tasks (id, task_name, assigned_user_id, status, deadline, created_by, pending_approval, team_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.Exec(query, task.ID, task.TaskName, task.AssignedUserID,
		task.Status, task.Deadline, task.CreatedBy, task.PendingApproval,
		task.TeamID, task.CreatedAt, task.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	return nil
}

func (r *TaskRepository) GetByID(id uuid.UUID) (*models.Task, error) {
	task := &models.Task{}
	query := `
		SELECT id, task_name, assigned_user_id, status, start_time, finish_time, 
			deadline, created_by, pending_approval, team_id, created_at, updated_at
		FROM tasks WHERE id = $1 AND deleted_at IS NULL
	`

	err := r.db.QueryRow(query, id).Scan(
		&task.ID, &task.TaskName, &task.AssignedUserID, &task.Status,
		&task.StartTime, &task.FinishTime, &task.Deadline, &task.CreatedBy,
		&task.PendingApproval, &task.TeamID, &task.CreatedAt, &task.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	return task, nil
}

func (r *TaskRepository) GetWithDetails(id uuid.UUID) (*models.TaskWithDetails, error) {
	task := &models.TaskWithDetails{}
	query := `
		SELECT t.id, t.task_name, t.assigned_user_id, t.status, t.start_time, t.finish_time,
			t.deadline, t.created_by, t.pending_approval, t.created_at, t.updated_at,
			au.full_name as assigned_user_name, au.email as assigned_user_email,
			cu.full_name as created_by_name, cu.email as created_by_email
		FROM tasks t
		JOIN profiles au ON t.assigned_user_id = au.id
		JOIN profiles cu ON t.created_by = cu.id
		WHERE t.id = $1 AND t.deleted_at IS NULL
	`

	err := r.db.QueryRow(query, id).Scan(
		&task.ID, &task.TaskName, &task.AssignedUserID, &task.Status,
		&task.StartTime, &task.FinishTime, &task.Deadline, &task.CreatedBy,
		&task.PendingApproval, &task.CreatedAt, &task.UpdatedAt,
		&task.AssignedUserName, &task.AssignedUserEmail,
		&task.CreatedByName, &task.CreatedByEmail,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task with details: %w", err)
	}

	return task, nil
}

func (r *TaskRepository) List(params models.TaskQueryParams) ([]models.TaskWithDetails, error) {
	query := `
		SELECT t.id, t.task_name, t.assigned_user_id, t.status, t.start_time, t.finish_time,
			t.deadline, t.created_by, t.pending_approval, t.team_id, t.created_at, t.updated_at,
			au.full_name as assigned_user_name, au.email as assigned_user_email,
			cu.full_name as created_by_name, cu.email as created_by_email
		FROM tasks t
		JOIN profiles au ON t.assigned_user_id = au.id
		JOIN profiles cu ON t.created_by = cu.id
	`

	var conditions []string
	var args []interface{}
	argCount := 1

	conditions = append(conditions, "t.deleted_at IS NULL")

	if params.AssignedUserID != nil {
		conditions = append(conditions, fmt.Sprintf("t.assigned_user_id = $%d", argCount))
		args = append(args, *params.AssignedUserID)
		argCount++
	}

	if params.Status != nil {
		conditions = append(conditions, fmt.Sprintf("t.status = $%d", argCount))
		args = append(args, *params.Status)
		argCount++
	}

	if params.CreatedBy != nil {
		conditions = append(conditions, fmt.Sprintf("t.created_by = $%d", argCount))
		args = append(args, *params.CreatedBy)
		argCount++
	}

	if params.TeamID != nil {
		conditions = append(conditions, fmt.Sprintf("t.team_id = $%d", argCount))
		args = append(args, *params.TeamID)
		argCount++
	}

	// Filter by multiple team IDs (for users with can_manage_tasks in multiple teams)
	if len(params.TeamIDs) > 0 {
		placeholders := make([]string, len(params.TeamIDs))
		for i, teamID := range params.TeamIDs {
			placeholders[i] = fmt.Sprintf("$%d", argCount)
			args = append(args, teamID)
			argCount++
		}
		conditions = append(conditions, fmt.Sprintf("t.team_id IN (%s)", strings.Join(placeholders, ", ")))
	}

	if params.Search != "" {
		// Fuzzy search: replace spaces with wildcards
		searchPattern := "%" + strings.ReplaceAll(params.Search, " ", "%") + "%"
		conditions = append(conditions, fmt.Sprintf("(t.task_name ILIKE $%d OR au.full_name ILIKE $%d)", argCount, argCount))
		args = append(args, searchPattern)
		argCount++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY t.created_at DESC"

	if params.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, params.Limit)
		argCount++
	}

	if params.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, params.Offset)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []models.TaskWithDetails
	for rows.Next() {
		var task models.TaskWithDetails
		err := rows.Scan(
			&task.ID, &task.TaskName, &task.AssignedUserID, &task.Status,
			&task.StartTime, &task.FinishTime, &task.Deadline, &task.CreatedBy,
			&task.PendingApproval, &task.TeamID, &task.CreatedAt, &task.UpdatedAt,
			&task.AssignedUserName, &task.AssignedUserEmail,
			&task.CreatedByName, &task.CreatedByEmail,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (r *TaskRepository) ListAll() ([]models.Task, error) {
	query := `SELECT id, task_name, assigned_user_id, status, deadline, created_by, pending_approval, team_id, created_at, updated_at FROM tasks WHERE deleted_at IS NULL`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		err := rows.Scan(&t.ID, &t.TaskName, &t.AssignedUserID, &t.Status, &t.Deadline, &t.CreatedBy, &t.PendingApproval, &t.TeamID, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (r *TaskRepository) Update(id uuid.UUID, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	var setClauses []string
	var args []interface{}
	argCount := 1

	for key, value := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, argCount))
		args = append(args, value)
		argCount++
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE tasks SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "), argCount)

	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	return nil
}

func (r *TaskRepository) UpdateStatus(id uuid.UUID, status models.TaskStatus) error {
	updates := map[string]interface{}{
		"status": status,
	}

	// Set timestamps based on status
	now := time.Now()
	if status == models.TaskStatusInProgress {
		updates["start_time"] = now
	} else if status == models.TaskStatusCompleted || status == models.TaskStatusDone || status == models.TaskStatusInReview {
		// Only set finish_time if it hasn't been set before
		query := `UPDATE tasks SET status = $1, finish_time = COALESCE(finish_time, $2) WHERE id = $3`
		_, err := r.db.Exec(query, status, now, id)
		return err
	}

	return r.Update(id, updates)
}

func (r *TaskRepository) Delete(id uuid.UUID) error {
	query := `UPDATE tasks SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to soft-delete task: %w", err)
	}
	return nil
}

func (r *TaskRepository) Count(params models.TaskQueryParams) (int, error) {
	query := "SELECT COUNT(*) FROM tasks t JOIN profiles au ON t.assigned_user_id = au.id"

	var conditions []string
	var args []interface{}
	argCount := 1

	conditions = append(conditions, "t.deleted_at IS NULL")

	if params.AssignedUserID != nil {
		conditions = append(conditions, fmt.Sprintf("t.assigned_user_id = $%d", argCount))
		args = append(args, *params.AssignedUserID)
		argCount++
	}

	if params.Status != nil {
		conditions = append(conditions, fmt.Sprintf("t.status = $%d", argCount))
		args = append(args, *params.Status)
		argCount++
	}

	if params.CreatedBy != nil {
		conditions = append(conditions, fmt.Sprintf("t.created_by = $%d", argCount))
		args = append(args, *params.CreatedBy)
		argCount++
	}

	if params.TeamID != nil {
		conditions = append(conditions, fmt.Sprintf("t.team_id = $%d", argCount))
		args = append(args, *params.TeamID)
		argCount++
	}

	if params.Search != "" {
		// Fuzzy search: replace spaces with wildcards
		searchPattern := "%" + strings.ReplaceAll(params.Search, " ", "%") + "%"
		query = "SELECT COUNT(*) FROM tasks t JOIN profiles au ON t.assigned_user_id = au.id"
		conditions = append(conditions, fmt.Sprintf("(t.task_name ILIKE $%d OR au.full_name ILIKE $%d)", argCount, argCount))
		args = append(args, searchPattern)
		argCount++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	var count int
	err := r.db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count tasks: %w", err)
	}

	return count, nil
}

func (r *TaskRepository) GetDB() *sql.DB {
	return r.db
}

func (r *TaskRepository) IsManagerOf(managerID, subordinateID uuid.UUID) (bool, error) {
	// Check if managerID is a head, lead, or has custom management permissions of a team that subordinateID is a member of
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM team_members m
			JOIN team_members s ON m.team_id = s.team_id
			LEFT JOIN custom_roles cr ON m.custom_role_id = cr.id
			WHERE m.user_id = $1 
			AND s.user_id = $2
			AND (
				m.is_head = TRUE 
				OR m.is_lead = TRUE 
				OR cr.can_manage_tasks = TRUE 
				OR cr.can_manage_members_limited = TRUE
			)
		)
	`

	var isManager bool
	err := r.db.QueryRow(query, managerID, subordinateID).Scan(&isManager)
	if err != nil {
		return false, fmt.Errorf("failed to check manager status: %w", err)
	}
	return isManager, nil
}

func (r *TaskRepository) IsManagerOfTeam(userID, teamID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM team_members m
			LEFT JOIN custom_roles cr ON m.custom_role_id = cr.id
			WHERE m.user_id = $1 AND m.team_id = $2
			AND (
				m.is_head = TRUE 
				OR m.is_lead = TRUE 
				OR cr.can_manage_tasks = TRUE
				OR cr.can_manage_members_limited = TRUE
			)
		)
	`

	var isManager bool
	err := r.db.QueryRow(query, userID, teamID).Scan(&isManager)
	if err != nil {
		return false, fmt.Errorf("failed to check team manager status: %w", err)
	}

	return isManager, nil
}
