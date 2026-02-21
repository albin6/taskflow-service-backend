package models

import (
	"time"

	"github.com/google/uuid"
)

type CustomRole struct {
	ID                      uuid.UUID  `json:"id" db:"id"`
	TeamID                  uuid.UUID  `json:"team_id" db:"team_id"`
	RoleName                string     `json:"role_name" db:"role_name"`
	CanManageTasks          bool       `json:"can_manage_tasks" db:"can_manage_tasks"`
	CanViewAnalytics        bool       `json:"can_view_analytics" db:"can_view_analytics"`
	CanManageMembersLimited bool       `json:"can_manage_members_limited" db:"can_manage_members_limited"`
	CreatedBy               *uuid.UUID `json:"created_by" db:"created_by"`
	CreatedAt               time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at" db:"updated_at"`
}

type CreateCustomRoleRequest struct {
	RoleName                string `json:"role_name" binding:"required"`
	CanManageTasks          bool   `json:"can_manage_tasks"`
	CanViewAnalytics        bool   `json:"can_view_analytics"`
	CanManageMembersLimited bool   `json:"can_manage_members_limited"`
}

type UpdateCustomRoleRequest struct {
	RoleName                *string `json:"role_name"`
	CanManageTasks          *bool   `json:"can_manage_tasks"`
	CanViewAnalytics        *bool   `json:"can_view_analytics"`
	CanManageMembersLimited *bool   `json:"can_manage_members_limited"`
}

type UpdateMemberPositionRequest struct {
	Position string `json:"position" binding:"required"`
}

type AssignRoleRequest struct {
	CustomRoleID *uuid.UUID `json:"custom_role_id"`
}
