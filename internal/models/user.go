package models

import (
	"time"

	"github.com/google/uuid"
)

type AppRole string

const (
	RoleAdmin AppRole = "admin"
	RoleUser  AppRole = "user"
)

type Profile struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	Email           string     `json:"email" db:"email"`
	FullName        *string    `json:"full_name" db:"full_name"`
	AvatarURL       *string    `json:"avatar_url" db:"avatar_url"`
	IsApproved      bool       `json:"is_approved" db:"is_approved"`
	PasswordHash    string     `json:"-" db:"password_hash"`
	EmailVerified   bool       `json:"email_verified" db:"email_verified"`
	RequestedRole   *string    `json:"requested_role" db:"requested_role"`
	RequestedTeamID *uuid.UUID `json:"requested_team_id" db:"requested_team_id"`
	DeletedAt       *time.Time `json:"-" db:"deleted_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

type UserRole struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Role      AppRole   `json:"role" db:"role"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type User struct {
	Profile Profile `json:"profile"`
	Role    AppRole `json:"role"`
}

// DTOs for API requests/responses
type SignUpRequest struct {
	Email           string     `json:"email" binding:"required,email"`
	Password        string     `json:"password" binding:"required,min=8"`
	FullName        string     `json:"full_name" binding:"required"`
	RequestedRole   string     `json:"requested_role" binding:"required"`
	RequestedTeamID *uuid.UUID `json:"requested_team_id"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type UpdateProfileRequest struct {
	FullName  *string `json:"full_name"`
	AvatarURL *string `json:"avatar_url"`
}

type UserListResponse struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	FullName        *string    `json:"full_name"`
	AvatarURL       *string    `json:"avatar_url"`
	IsApproved      bool       `json:"is_approved"`
	EmailVerified   bool       `json:"email_verified"`
	Role            AppRole    `json:"role"`
	RequestedRole   *string    `json:"requested_role"`
	RequestedTeamID *uuid.UUID `json:"requested_team_id"`
	IsHeadAnywhere  bool       `json:"is_head_anywhere"`
	IsLeadAnywhere  bool       `json:"is_lead_anywhere"`
	CustomRoles     *string    `json:"custom_roles"`
	CreatedAt       time.Time  `json:"created_at"`
}

// AdminCreateUserRequest is the DTO for admin-initiated user creation.
type AdminCreateUserRequest struct {
	Name     string     `json:"name" binding:"required"`
	Email    string     `json:"email" binding:"required,email"`
	Role     AppRole    `json:"role"`     // defaults to RoleUser if empty
	TeamID   *uuid.UUID `json:"team_id"`  // optional
	RoleID   *uuid.UUID `json:"role_id"`  // NEW: custom team-specific role
	Password string     `json:"password"` // optional — auto-generated if blank
}

// PaginatedUsersResponse wraps a paginated user list.
type PaginatedUsersResponse struct {
	Users  []UserListResponse `json:"users"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

type RefreshToken struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	TokenHash string    `json:"-" db:"token_hash"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	Revoked   bool      `json:"revoked" db:"revoked"`
}
