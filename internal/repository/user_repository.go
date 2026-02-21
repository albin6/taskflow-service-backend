package repository

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/taskflow/backend/internal/cache"
	"github.com/taskflow/backend/internal/models"
)

type UserRepository struct {
	db    *sql.DB
	cache *cache.RedisClient
}

func NewUserRepository(db *sql.DB, cache *cache.RedisClient) *UserRepository {
	return &UserRepository{db: db, cache: cache}
}

func (r *UserRepository) invalidateUsersCache() {
	if r.cache != nil {
		r.cache.Client.Del(context.Background(), "users:all")
	}
}

func (r *UserRepository) Create(email, passwordHash, fullName string, requestedRole *string, requestedTeamID *uuid.UUID) (*models.Profile, error) {
	profile := &models.Profile{
		ID:              uuid.New(),
		Email:           email,
		PasswordHash:    passwordHash,
		IsApproved:      false,
		EmailVerified:   false,
		RequestedRole:   requestedRole,
		RequestedTeamID: requestedTeamID,
	}

	if fullName != "" {
		profile.FullName = &fullName
	}

	query := `
		INSERT INTO profiles (id, email, full_name, password_hash, is_approved, email_verified, requested_role, requested_team_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRow(query, profile.ID, profile.Email, profile.FullName,
		profile.PasswordHash, profile.IsApproved, profile.EmailVerified,
		profile.RequestedRole, profile.RequestedTeamID).
		Scan(&profile.CreatedAt, &profile.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}

	// Assign default user role
	roleQuery := `INSERT INTO user_roles (user_id, role) VALUES ($1, $2)`
	_, err = r.db.Exec(roleQuery, profile.ID, models.RoleUser)
	if err != nil {
		return nil, fmt.Errorf("failed to assign user role: %w", err)
	}

	r.invalidateUsersCache()
	return profile, nil
}

func (r *UserRepository) GetByEmail(email string) (*models.Profile, error) {
	profile := &models.Profile{}
	query := `SELECT id, email, full_name, avatar_url, is_approved, password_hash, 
		email_verified, requested_role, requested_team_id, created_at, updated_at FROM profiles WHERE email = $1 AND deleted_at IS NULL`

	err := r.db.QueryRow(query, email).Scan(
		&profile.ID, &profile.Email, &profile.FullName, &profile.AvatarURL,
		&profile.IsApproved, &profile.PasswordHash, &profile.EmailVerified,
		&profile.RequestedRole, &profile.RequestedTeamID,
		&profile.CreatedAt, &profile.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get profile by email: %w", err)
	}

	return profile, nil
}

func (r *UserRepository) GetByID(id uuid.UUID) (*models.Profile, error) {
	profile := &models.Profile{}
	query := `SELECT id, email, full_name, avatar_url, is_approved, password_hash, 
		email_verified, requested_role, requested_team_id, created_at, updated_at FROM profiles WHERE id = $1 AND deleted_at IS NULL`

	err := r.db.QueryRow(query, id).Scan(
		&profile.ID, &profile.Email, &profile.FullName, &profile.AvatarURL,
		&profile.IsApproved, &profile.PasswordHash, &profile.EmailVerified,
		&profile.RequestedRole, &profile.RequestedTeamID,
		&profile.CreatedAt, &profile.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get profile by ID: %w", err)
	}

	return profile, nil
}

func (r *UserRepository) GetUserRole(userID uuid.UUID) (models.AppRole, error) {
	var role models.AppRole
	query := `SELECT role FROM user_roles WHERE user_id = $1 LIMIT 1`

	err := r.db.QueryRow(query, userID).Scan(&role)
	if err == sql.ErrNoRows {
		return models.RoleUser, nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get user role: %w", err)
	}

	return role, nil
}

func (r *UserRepository) GetUser(userID uuid.UUID) (*models.User, error) {
	profile, err := r.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, nil
	}

	role, err := r.GetUserRole(userID)
	if err != nil {
		return nil, err
	}

	return &models.User{
		Profile: *profile,
		Role:    role,
	}, nil
}

func (r *UserRepository) ListAll() ([]models.UserListResponse, error) {
	// Try cache first
	if r.cache != nil {
		ctx := context.Background()
		val, err := r.cache.Client.Get(ctx, "users:all").Result()
		if err == nil {
			var users []models.UserListResponse
			if err := json.Unmarshal([]byte(val), &users); err == nil {
				return users, nil
			}
		}
	}

	query := `
		SELECT p.id, p.email, p.full_name, p.avatar_url, p.is_approved, 
			p.email_verified, COALESCE(ur.role, 'user') as role, p.created_at,
			p.requested_role, p.requested_team_id,
			EXISTS(SELECT 1 FROM team_members tm WHERE tm.user_id = p.id AND tm.is_head = true) as is_head_anywhere,
			EXISTS(SELECT 1 FROM team_members tm WHERE tm.user_id = p.id AND tm.is_lead = true) as is_lead_anywhere,
			(SELECT STRING_AGG(DISTINCT cr.role_name, ', ') 
			 FROM team_members tm 
			 JOIN custom_roles cr ON tm.custom_role_id = cr.id 
			 WHERE tm.user_id = p.id) as custom_roles
		FROM profiles p
		LEFT JOIN user_roles ur ON p.id = ur.user_id
		WHERE p.deleted_at IS NULL
		ORDER BY p.created_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []models.UserListResponse
	for rows.Next() {
		var user models.UserListResponse
		err := rows.Scan(
			&user.ID, &user.Email, &user.FullName, &user.AvatarURL,
			&user.IsApproved, &user.EmailVerified, &user.Role, &user.CreatedAt,
			&user.RequestedRole, &user.RequestedTeamID,
			&user.IsHeadAnywhere, &user.IsLeadAnywhere,
			&user.CustomRoles,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if r.cache != nil {
		data, _ := json.Marshal(users)
		r.cache.Client.Set(context.Background(), "users:all", data, 30*time.Minute)
	}

	return users, nil
}

// ListFiltered returns a page of users matching the given filters, plus total count.
// search is matched against email and full_name (case-insensitive).
// role filters by user_roles.role. status: "approved"|"pending" filters by is_approved.
func (r *UserRepository) ListFiltered(search, role, status string, sortBy, sortDir string, limit, offset int) ([]models.UserListResponse, int, error) {
	args := []any{}
	argIdx := 1

	baseConditions := "p.deleted_at IS NULL"

	if search != "" {
		baseConditions += fmt.Sprintf(` AND (LOWER(p.email) LIKE LOWER($%d) OR LOWER(p.full_name) LIKE LOWER($%d))`, argIdx, argIdx+1)
		like := "%" + search + "%"
		args = append(args, like, like)
		argIdx += 2
	}
	if role != "" {
		baseConditions += fmt.Sprintf(` AND COALESCE(ur.role, 'user') = $%d`, argIdx)
		args = append(args, role)
		argIdx++
	}
	if status == "approved" {
		baseConditions += " AND p.is_approved = true"
	} else if status == "pending" {
		baseConditions += " AND p.is_approved = false"
	}

	// Validate sort column to prevent SQL injection
	allowedSortCols := map[string]string{
		"name":       "COALESCE(p.full_name, p.email)",
		"email":      "p.email",
		"created_at": "p.created_at",
		"role":       "COALESCE(ur.role, 'user')",
	}
	sortCol, ok := allowedSortCols[sortBy]
	if !ok {
		sortCol = "p.created_at"
	}
	if sortDir != "asc" {
		sortDir = "desc"
	}

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM profiles p
		LEFT JOIN user_roles ur ON p.id = ur.user_id
		WHERE %s
	`, baseConditions)

	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	listArgs := append(args, limit, offset)
	listQuery := fmt.Sprintf(`
		SELECT p.id, p.email, p.full_name, p.avatar_url, p.is_approved,
			p.email_verified, COALESCE(ur.role, 'user') as role, p.created_at,
			p.requested_role, p.requested_team_id,
			EXISTS(SELECT 1 FROM team_members tm WHERE tm.user_id = p.id AND tm.is_head = true) as is_head_anywhere,
			EXISTS(SELECT 1 FROM team_members tm WHERE tm.user_id = p.id AND tm.is_lead = true) as is_lead_anywhere,
			(SELECT STRING_AGG(DISTINCT cr.role_name, ', ') 
			 FROM team_members tm 
			 JOIN custom_roles cr ON tm.custom_role_id = cr.id 
			 WHERE tm.user_id = p.id) as custom_roles
		FROM profiles p
		LEFT JOIN user_roles ur ON p.id = ur.user_id
		WHERE %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, baseConditions, sortCol, sortDir, argIdx, argIdx+1)

	rows, err := r.db.Query(listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list filtered users: %w", err)
	}
	defer rows.Close()

	var users []models.UserListResponse
	for rows.Next() {
		var u models.UserListResponse
		if err := rows.Scan(
			&u.ID, &u.Email, &u.FullName, &u.AvatarURL,
			&u.IsApproved, &u.EmailVerified, &u.Role, &u.CreatedAt,
			&u.RequestedRole, &u.RequestedTeamID,
			&u.IsHeadAnywhere, &u.IsLeadAnywhere,
			&u.CustomRoles,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}
	if users == nil {
		users = []models.UserListResponse{}
	}
	return users, total, nil
}

// CreateApproved creates a pre-approved user (admin-initiated).
// The user is immediately approved and assigned the given role.
func (r *UserRepository) CreateApproved(email, passwordHash, fullName string, role models.AppRole) (*models.Profile, error) {
	profile := &models.Profile{
		ID:            uuid.New(),
		Email:         email,
		PasswordHash:  passwordHash,
		IsApproved:    true,
		EmailVerified: true,
	}
	if fullName != "" {
		profile.FullName = &fullName
	}

	query := `
		INSERT INTO profiles (id, email, full_name, password_hash, is_approved, email_verified)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`
	err := r.db.QueryRow(query,
		profile.ID, profile.Email, profile.FullName,
		profile.PasswordHash, profile.IsApproved, profile.EmailVerified,
	).Scan(&profile.CreatedAt, &profile.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}

	if role == "" {
		role = models.RoleUser
	}
	if _, err := r.db.Exec(`INSERT INTO user_roles (user_id, role) VALUES ($1, $2)`, profile.ID, role); err != nil {
		return nil, fmt.Errorf("failed to assign role: %w", err)
	}

	r.invalidateUsersCache()
	return profile, nil
}

func (r *UserRepository) Update(userID uuid.UUID, fullName, avatarURL *string) error {
	query := `UPDATE profiles SET full_name = $1, avatar_url = $2 WHERE id = $3`
	_, err := r.db.Exec(query, fullName, avatarURL, userID)
	if err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}
	r.invalidateUsersCache()
	return nil
}

func (r *UserRepository) Approve(userID uuid.UUID) error {
	query := `UPDATE profiles SET is_approved = true, email_verified = true WHERE id = $1`
	_, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to approve user: %w", err)
	}
	r.invalidateUsersCache()
	return nil
}

func (r *UserRepository) Delete(userID uuid.UUID) error {
	query := `UPDATE profiles SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to soft-delete user: %w", err)
	}
	r.invalidateUsersCache()
	return nil
}

func (r *UserRepository) SetRole(userID uuid.UUID, role models.AppRole) error {
	// Check if role exists
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM user_roles WHERE user_id = $1)`
	err := r.db.QueryRow(checkQuery, userID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check role existence: %w", err)
	}

	if exists {
		query := `UPDATE user_roles SET role = $1 WHERE user_id = $2`
		_, err = r.db.Exec(query, role, userID)
	} else {
		query := `INSERT INTO user_roles (user_id, role) VALUES ($1, $2)`
		_, err = r.db.Exec(query, userID, role)
	}

	if err != nil {
		return fmt.Errorf("failed to set user role: %w", err)
	}
	r.invalidateUsersCache()
	return nil
}

// Refresh token operations
func (r *UserRepository) SaveRefreshToken(userID uuid.UUID, token string, expiresAt time.Time) error {
	// Hash the token before storing
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	query := `INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`
	_, err := r.db.Exec(query, userID, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to save refresh token: %w", err)
	}
	return nil
}

func (r *UserRepository) ValidateRefreshToken(token string) (uuid.UUID, error) {
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	var userID uuid.UUID
	var expiresAt time.Time
	var revoked bool

	query := `SELECT user_id, expires_at, revoked FROM refresh_tokens WHERE token_hash = $1`
	err := r.db.QueryRow(query, tokenHash).Scan(&userID, &expiresAt, &revoked)

	if err == sql.ErrNoRows {
		return uuid.Nil, fmt.Errorf("invalid refresh token")
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to validate refresh token: %w", err)
	}

	if revoked {
		return uuid.Nil, fmt.Errorf("refresh token has been revoked")
	}

	if time.Now().After(expiresAt) {
		return uuid.Nil, fmt.Errorf("refresh token has expired")
	}

	return userID, nil
}

func (r *UserRepository) RevokeRefreshToken(token string) error {
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	query := `UPDATE refresh_tokens SET revoked = true WHERE token_hash = $1`
	_, err := r.db.Exec(query, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

func (r *UserRepository) RevokeAllUserTokens(userID uuid.UUID) error {
	query := `UPDATE refresh_tokens SET revoked = true WHERE user_id = $1`
	_, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke user tokens: %w", err)
	}
	return nil
}

// Password reset operations
func (r *UserRepository) CreatePasswordResetToken(userID uuid.UUID, token string, expiresAt time.Time) error {
	query := `INSERT INTO password_reset_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`
	_, err := r.db.Exec(query, userID, token, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to create password reset token: %w", err)
	}
	return nil
}

func (r *UserRepository) ValidatePasswordResetToken(token string) (uuid.UUID, error) {
	var userID uuid.UUID
	var expiresAt time.Time
	var used bool

	query := `SELECT user_id, expires_at, used FROM password_reset_tokens WHERE token = $1`
	err := r.db.QueryRow(query, token).Scan(&userID, &expiresAt, &used)

	if err == sql.ErrNoRows {
		return uuid.Nil, fmt.Errorf("invalid password reset token")
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to validate password reset token: %w", err)
	}

	if used {
		return uuid.Nil, fmt.Errorf("password reset token has already been used")
	}

	if time.Now().After(expiresAt) {
		return uuid.Nil, fmt.Errorf("password reset token has expired")
	}

	return userID, nil
}

func (r *UserRepository) MarkPasswordResetTokenAsUsed(token string) error {
	query := `UPDATE password_reset_tokens SET used = true WHERE token = $1`
	_, err := r.db.Exec(query, token)
	if err != nil {
		return fmt.Errorf("failed to mark password reset token as used: %w", err)
	}
	return nil
}

func (r *UserRepository) UpdatePassword(userID uuid.UUID, passwordHash string) error {
	query := `UPDATE profiles SET password_hash = $1 WHERE id = $2`
	_, err := r.db.Exec(query, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	return nil
}

func (r *UserRepository) DeleteExpiredPasswordResetTokens() error {
	query := `DELETE FROM password_reset_tokens WHERE expires_at < NOW() OR used = true`
	_, err := r.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to delete expired password reset tokens: %w", err)
	}
	return nil
}
