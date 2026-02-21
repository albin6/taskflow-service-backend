package models

import (
	"time"

	"github.com/google/uuid"
)

type Team struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	Name        string     `json:"name" db:"name"`
	Description *string    `json:"description" db:"description"`
	CreatedBy   uuid.UUID  `json:"created_by" db:"created_by"`
	DeletedAt   *time.Time `json:"-" db:"deleted_at"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

type TeamRole struct {
	ID          uuid.UUID `json:"id" db:"id"`
	TeamID      uuid.UUID `json:"team_id" db:"team_id"`
	RoleName    string    `json:"role_name" db:"role_name"`
	Permissions []string  `json:"permissions" db:"permissions"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type TeamMember struct {
	ID           uuid.UUID   `json:"id" db:"id"`
	UserID       uuid.UUID   `json:"user_id" db:"user_id"`
	TeamID       uuid.UUID   `json:"team_id" db:"team_id"`
	RoleID       *uuid.UUID  `json:"role_id" db:"role_id"`
	CustomRoleID *uuid.UUID  `json:"custom_role_id" db:"custom_role_id"`
	CustomRole   *CustomRole `json:"custom_role,omitempty"` // Full custom role object with permissions
	IsHead       bool        `json:"is_head" db:"is_head"`
	IsLead       bool        `json:"is_lead" db:"is_lead"`
	JoinedAt     time.Time   `json:"joined_at" db:"joined_at"`
}

// DTOs for API requests/responses
type CreateTeamRequest struct {
	Name        string     `json:"name" binding:"required"`
	Description *string    `json:"description"`
	HeadUserID  *uuid.UUID `json:"head_user_id"`
}

type UpdateTeamRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

type AddMemberRequest struct {
	UserID uuid.UUID  `json:"user_id" binding:"required"`
	RoleID *uuid.UUID `json:"role_id"`
	IsHead bool       `json:"is_head"`
	IsLead bool       `json:"is_lead"`
}

type UpdateMemberRoleRequest struct {
	RoleID *uuid.UUID `json:"role_id"`
	IsHead *bool      `json:"is_head"`
	IsLead *bool      `json:"is_lead"`
}

type CreateRoleRequest struct {
	RoleName    string   `json:"role_name" binding:"required"`
	Permissions []string `json:"permissions"`
}

type TeamWithMembers struct {
	Team
	Members []TeamMemberDetail `json:"members"`
}

type TeamMemberDetail struct {
	TeamMember
	Email     string  `json:"email"`
	FullName  *string `json:"full_name"`
	AvatarURL *string `json:"avatar_url"`
	RoleName  *string `json:"role_name"`
}

type AssignableUser struct {
	UserID   uuid.UUID `json:"user_id" db:"user_id"`
	Email    string    `json:"email" db:"email"`
	FullName *string   `json:"full_name" db:"full_name"`
	IsHead   bool      `json:"is_head" db:"is_head"`
	IsLead   bool      `json:"is_lead" db:"is_lead"`
}

type UserTeamMembership struct {
	TeamID                  uuid.UUID `json:"team_id"`
	TeamName                string    `json:"team_name"`
	IsHead                  bool      `json:"is_head"`
	IsLead                  bool      `json:"is_lead"`
	RoleName                *string   `json:"role_name"`
	CanManageTasks          bool      `json:"can_manage_tasks"`
	CanViewAnalytics        bool      `json:"can_view_analytics"`
	CanManageMembersLimited bool      `json:"can_manage_members_limited"`
}
