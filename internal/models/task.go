package models

import (
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusInReview   TaskStatus = "in_review"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusCompleted  TaskStatus = "completed"
)

type Task struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	TaskName        string     `json:"task_name" db:"task_name"`
	AssignedUserID  uuid.UUID  `json:"assigned_user_id" db:"assigned_user_id"`
	Status          TaskStatus `json:"status" db:"status"`
	StartTime       *time.Time `json:"start_time" db:"start_time"`
	FinishTime      *time.Time `json:"finish_time" db:"finish_time"`
	Deadline        time.Time  `json:"deadline" db:"deadline"`
	CreatedBy       uuid.UUID  `json:"created_by" db:"created_by"`
	PendingApproval bool       `json:"pending_approval" db:"pending_approval"`
	TeamID          *uuid.UUID `json:"team_id" db:"team_id"`
	DeletedAt       *time.Time `json:"-" db:"deleted_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

// DTOs for API requests/responses
type CreateTaskRequest struct {
	TaskName       string     `json:"task_name" binding:"required"`
	AssignedUserID uuid.UUID  `json:"assigned_user_id" binding:"required"`
	Deadline       time.Time  `json:"deadline" binding:"required"`
	TeamID         *uuid.UUID `json:"team_id"`
}

type UpdateTaskRequest struct {
	TaskName       *string     `json:"task_name"`
	AssignedUserID *uuid.UUID  `json:"assigned_user_id"`
	Status         *TaskStatus `json:"status"`
	Deadline       *time.Time  `json:"deadline"`
}

type UpdateTaskStatusRequest struct {
	Status TaskStatus `json:"status" binding:"required"`
}

type TaskWithDetails struct {
	Task
	AssignedUserName  *string `json:"assigned_user_name"`
	AssignedUserEmail string  `json:"assigned_user_email"`
	CreatedByName     *string `json:"created_by_name"`
	CreatedByEmail    string  `json:"created_by_email"`
}

type TaskQueryParams struct {
	AssignedUserID *uuid.UUID
	Status         *TaskStatus
	CreatedBy      *uuid.UUID
	TeamID         *uuid.UUID
	TeamIDs        []uuid.UUID // For filtering by multiple teams
	Search         string
	Limit          int
	Offset         int
}
