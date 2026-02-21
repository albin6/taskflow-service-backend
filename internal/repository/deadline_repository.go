package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/taskflow/backend/internal/models"
)

type DeadlineRepository struct {
	db *sql.DB
}

func NewDeadlineRepository(db *sql.DB) *DeadlineRepository {
	return &DeadlineRepository{db: db}
}

func (r *DeadlineRepository) Create(req *models.DeadlineRequest) error {
	req.ID = uuid.New()
	req.CreatedAt = time.Now()
	req.Status = models.RequestStatusPending

	query := `
		INSERT INTO deadline_requests (id, task_id, requested_by, current_deadline, 
			requested_deadline, reason, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Exec(query, req.ID, req.TaskID, req.RequestedBy,
		req.CurrentDeadline, req.RequestedDeadline, req.Reason, req.Status, req.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create deadline request: %w", err)
	}

	return nil
}

func (r *DeadlineRepository) GetByID(id uuid.UUID) (*models.DeadlineRequest, error) {
	req := &models.DeadlineRequest{}
	query := `
		SELECT id, task_id, requested_by, current_deadline, requested_deadline, 
			reason, status, reviewed_by, reviewed_at, created_at
		FROM deadline_requests WHERE id = $1
	`

	err := r.db.QueryRow(query, id).Scan(
		&req.ID, &req.TaskID, &req.RequestedBy, &req.CurrentDeadline,
		&req.RequestedDeadline, &req.Reason, &req.Status, &req.ReviewedBy,
		&req.ReviewedAt, &req.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get deadline request: %w", err)
	}

	return req, nil
}

func (r *DeadlineRepository) GetWithDetails(id uuid.UUID) (*models.DeadlineRequestWithDetails, error) {
	req := &models.DeadlineRequestWithDetails{}
	query := `
		SELECT dr.id, dr.task_id, dr.requested_by, dr.current_deadline, dr.requested_deadline,
			dr.reason, dr.status, dr.reviewed_by, dr.reviewed_at, dr.created_at,
			t.task_name,
			rb.full_name as requested_by_name, rb.email as requested_by_email,
			rv.full_name as reviewed_by_name, rv.email as reviewed_by_email
		FROM deadline_requests dr
		JOIN tasks t ON dr.task_id = t.id
		JOIN profiles rb ON dr.requested_by = rb.id
		LEFT JOIN profiles rv ON dr.reviewed_by = rv.id
		WHERE dr.id = $1
	`

	err := r.db.QueryRow(query, id).Scan(
		&req.ID, &req.TaskID, &req.RequestedBy, &req.CurrentDeadline,
		&req.RequestedDeadline, &req.Reason, &req.Status, &req.ReviewedBy,
		&req.ReviewedAt, &req.CreatedAt,
		&req.TaskName, &req.RequestedByName, &req.RequestedByEmail,
		&req.ReviewedByName, &req.ReviewedByEmail,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get deadline request with details: %w", err)
	}

	return req, nil
}

func (r *DeadlineRepository) ListByUser(userID uuid.UUID) ([]models.DeadlineRequestWithDetails, error) {
	query := `
		SELECT dr.id, dr.task_id, dr.requested_by, dr.current_deadline, dr.requested_deadline,
			dr.reason, dr.status, dr.reviewed_by, dr.reviewed_at, dr.created_at,
			t.task_name,
			rb.full_name as requested_by_name, rb.email as requested_by_email,
			rv.full_name as reviewed_by_name, rv.email as reviewed_by_email
		FROM deadline_requests dr
		JOIN tasks t ON dr.task_id = t.id
		JOIN profiles rb ON dr.requested_by = rb.id
		LEFT JOIN profiles rv ON dr.reviewed_by = rv.id
		WHERE dr.requested_by = $1
		ORDER BY dr.created_at DESC
	`

	return r.queryList(query, userID)
}

func (r *DeadlineRepository) ListByUserAndStatus(userID uuid.UUID, status models.RequestStatus) ([]models.DeadlineRequestWithDetails, error) {
	query := `
		SELECT dr.id, dr.task_id, dr.requested_by, dr.current_deadline, dr.requested_deadline,
			dr.reason, dr.status, dr.reviewed_by, dr.reviewed_at, dr.created_at,
			t.task_name,
			rb.full_name as requested_by_name, rb.email as requested_by_email,
			rv.full_name as reviewed_by_name, rv.email as reviewed_by_email
		FROM deadline_requests dr
		JOIN tasks t ON dr.task_id = t.id
		JOIN profiles rb ON dr.requested_by = rb.id
		LEFT JOIN profiles rv ON dr.reviewed_by = rv.id
		WHERE dr.requested_by = $1 AND dr.status = $2
		ORDER BY dr.created_at DESC
	`

	return r.queryList(query, userID, status)
}

func (r *DeadlineRepository) ListAll() ([]models.DeadlineRequestWithDetails, error) {
	query := `
		SELECT dr.id, dr.task_id, dr.requested_by, dr.current_deadline, dr.requested_deadline,
			dr.reason, dr.status, dr.reviewed_by, dr.reviewed_at, dr.created_at,
			t.task_name,
			rb.full_name as requested_by_name, rb.email as requested_by_email,
			rv.full_name as reviewed_by_name, rv.email as reviewed_by_email
		FROM deadline_requests dr
		JOIN tasks t ON dr.task_id = t.id
		JOIN profiles rb ON dr.requested_by = rb.id
		LEFT JOIN profiles rv ON dr.reviewed_by = rv.id
		ORDER BY dr.created_at DESC
	`

	return r.queryList(query)
}

func (r *DeadlineRepository) ListByStatus(status models.RequestStatus) ([]models.DeadlineRequestWithDetails, error) {
	query := `
		SELECT dr.id, dr.task_id, dr.requested_by, dr.current_deadline, dr.requested_deadline,
			dr.reason, dr.status, dr.reviewed_by, dr.reviewed_at, dr.created_at,
			t.task_name,
			rb.full_name as requested_by_name, rb.email as requested_by_email,
			rv.full_name as reviewed_by_name, rv.email as reviewed_by_email
		FROM deadline_requests dr
		JOIN tasks t ON dr.task_id = t.id
		JOIN profiles rb ON dr.requested_by = rb.id
		LEFT JOIN profiles rv ON dr.reviewed_by = rv.id
		WHERE dr.status = $1
		ORDER BY dr.created_at DESC
	`

	return r.queryList(query, status)
}

func (r *DeadlineRepository) ListForUserOrManager(userID uuid.UUID, isAdmin bool) ([]models.DeadlineRequestWithDetails, error) {
	var query string
	var args []interface{}

	if isAdmin {
		return r.ListAll()
	}

	query = `
		SELECT DISTINCT dr.id, dr.task_id, dr.requested_by, dr.current_deadline, dr.requested_deadline,
			dr.reason, dr.status, dr.reviewed_by, dr.reviewed_at, dr.created_at,
			t.task_name,
			rb.full_name as requested_by_name, rb.email as requested_by_email,
			rv.full_name as reviewed_by_name, rv.email as reviewed_by_email
		FROM deadline_requests dr
		JOIN tasks t ON dr.task_id = t.id
		JOIN profiles rb ON dr.requested_by = rb.id
		LEFT JOIN profiles rv ON dr.reviewed_by = rv.id
		LEFT JOIN team_members tm_requester ON dr.requested_by = tm_requester.user_id
		LEFT JOIN team_members tm_manager ON tm_requester.team_id = tm_manager.team_id
		WHERE 
			dr.requested_by = $1
			OR (
				tm_manager.user_id = $1 
				AND (tm_manager.is_head = true OR tm_manager.is_lead = true)
			)
		ORDER BY dr.created_at DESC
	`
	args = []interface{}{userID}

	return r.queryList(query, args...)
}

func (r *DeadlineRepository) queryList(query string, args ...interface{}) ([]models.DeadlineRequestWithDetails, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list deadline requests: %w", err)
	}
	defer rows.Close()

	var requests []models.DeadlineRequestWithDetails
	for rows.Next() {
		var req models.DeadlineRequestWithDetails
		err := rows.Scan(
			&req.ID, &req.TaskID, &req.RequestedBy, &req.CurrentDeadline,
			&req.RequestedDeadline, &req.Reason, &req.Status, &req.ReviewedBy,
			&req.ReviewedAt, &req.CreatedAt,
			&req.TaskName, &req.RequestedByName, &req.RequestedByEmail,
			&req.ReviewedByName, &req.ReviewedByEmail,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan deadline request: %w", err)
		}
		requests = append(requests, req)
	}

	return requests, nil
}

func (r *DeadlineRepository) UpdateStatus(id uuid.UUID, status models.RequestStatus, reviewedBy uuid.UUID) error {
	now := time.Now()
	query := `
		UPDATE deadline_requests 
		SET status = $1, reviewed_by = $2, reviewed_at = $3
		WHERE id = $4
	`

	_, err := r.db.Exec(query, status, reviewedBy, now, id)
	if err != nil {
		return fmt.Errorf("failed to update deadline request status: %w", err)
	}

	return nil
}

func (r *DeadlineRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM deadline_requests WHERE id = $1`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete deadline request: %w", err)
	}
	return nil
}
