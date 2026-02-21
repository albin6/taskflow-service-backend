package models

import (
	"time"

	"github.com/google/uuid"
)

type RequestStatus string

const (
	RequestStatusPending  RequestStatus = "pending"
	RequestStatusApproved RequestStatus = "approved"
	RequestStatusRejected RequestStatus = "rejected"
)

type DeadlineRequest struct {
	ID                uuid.UUID     `json:"id" db:"id"`
	TaskID            uuid.UUID     `json:"task_id" db:"task_id"`
	RequestedBy       uuid.UUID     `json:"requested_by" db:"requested_by"`
	CurrentDeadline   time.Time     `json:"current_deadline" db:"current_deadline"`
	RequestedDeadline time.Time     `json:"requested_deadline" db:"requested_deadline"`
	Reason            string        `json:"reason" db:"reason"`
	Status            RequestStatus `json:"status" db:"status"`
	ReviewedBy        *uuid.UUID    `json:"reviewed_by" db:"reviewed_by"`
	ReviewedAt        *time.Time    `json:"reviewed_at" db:"reviewed_at"`
	CreatedAt         time.Time     `json:"created_at" db:"created_at"`
}

// DTOs for API requests/responses
type CreateDeadlineRequestRequest struct {
	TaskID            uuid.UUID `json:"task_id" binding:"required"`
	RequestedDeadline time.Time `json:"requested_deadline" binding:"required"`
	Reason            string    `json:"reason" binding:"required"`
}

type ReviewDeadlineRequestRequest struct {
	Status RequestStatus `json:"status" binding:"required,oneof=approved rejected"`
}

type DeadlineRequestWithDetails struct {
	DeadlineRequest
	TaskName         string  `json:"task_name"`
	RequestedByName  *string `json:"requested_by_name"`
	RequestedByEmail string  `json:"requested_by_email"`
	ReviewedByName   *string `json:"reviewed_by_name"`
	ReviewedByEmail  *string `json:"reviewed_by_email"`
}
