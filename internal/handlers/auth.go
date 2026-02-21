package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/taskflow/backend/internal/auth"
	"github.com/taskflow/backend/internal/events"
	"github.com/taskflow/backend/internal/mail"
	"github.com/taskflow/backend/internal/models"
	"github.com/taskflow/backend/internal/repository"
)

type AuthHandler struct {
	userRepo     *repository.UserRepository
	jwtManager   *auth.JWTManager
	emailService *mail.EmailService
}

func NewAuthHandler(userRepo *repository.UserRepository, jwtManager *auth.JWTManager, emailService *mail.EmailService) *AuthHandler {
	return &AuthHandler{
		userRepo:     userRepo,
		jwtManager:   jwtManager,
		emailService: emailService,
	}
}

func (h *AuthHandler) SignUp(c *gin.Context) {
	var req models.SignUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists
	existing, err := h.userRepo.GetByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing user"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User with this email already exists"})
		return
	}

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Create user
	profile, err := h.userRepo.Create(req.Email, passwordHash, req.FullName, &req.RequestedRole, req.RequestedTeamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Publish user created event
	go func() {
		if err := events.PublishUserCreated(profile); err != nil {
			fmt.Printf("Failed to publish user created event: %v\n", err)
		}
	}()

	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully. Please wait for admin approval.",
		"user_id": profile.ID,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user by email
	profile, err := h.userRepo.GetByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to authenticate"})
		return
	}
	if profile == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Check password
	if !auth.CheckPasswordHash(req.Password, profile.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Check if user is approved
	if !profile.IsApproved {
		c.JSON(http.StatusForbidden, gin.H{"error": "Your account is pending admin approval"})
		return
	}

	// Get user role
	role, err := h.userRepo.GetUserRole(profile.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user role"})
		return
	}

	// Generate tokens
	accessToken, err := h.jwtManager.GenerateAccessToken(profile.ID, profile.Email, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access token"})
		return
	}

	refreshToken, err := h.jwtManager.GenerateRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}

	// Save refresh token
	expiresAt := time.Now().Add(h.jwtManager.RefreshTokenExpiration())
	if err := h.userRepo.SaveRefreshToken(profile.ID, refreshToken, expiresAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save refresh token"})
		return
	}

	c.JSON(http.StatusOK, models.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: models.User{
			Profile: *profile,
			Role:    role,
		},
	})
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req models.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate refresh token
	userID, err := h.userRepo.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	// Get user
	user, err := h.userRepo.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}
	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Generate new access token
	accessToken, err := h.jwtManager.GenerateAccessToken(user.Profile.ID, user.Profile.Email, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": accessToken,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req models.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Revoke refresh token
	if err := h.userRepo.RevokeRefreshToken(req.RefreshToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	user, err := h.userRepo.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}
	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req models.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user by email
	profile, err := h.userRepo.GetByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process request"})
		return
	}

	// Don't reveal if user exists or not for security
	if profile == nil {
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a password reset link will be sent"})
		return
	}

	// If new_password is provided, perform immediate reset (simplified flow for dev)
	if req.NewPassword != "" {
		hashedPassword, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		if err := h.userRepo.UpdatePassword(profile.ID, hashedPassword); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Password has been reset successfully. You can now login with your new password.",
		})
		return
	}

	// Generate reset token
	resetToken, err := h.jwtManager.GenerateRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate reset token"})
		return
	}

	// Save token (expires in 1 hour)
	expiresAt := time.Now().Add(1 * time.Hour)
	if err := h.userRepo.CreatePasswordResetToken(profile.ID, resetToken, expiresAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create reset token"})
		return
	}

	// In development, we print to console and optionally try to send email
	resetURL := fmt.Sprintf("http://localhost:5173/reset-password?token=%s", resetToken)
	fmt.Printf("\n=== PASSWORD RESET LINK ===\n%s\n===========================\n", resetURL)

	if err := h.emailService.SendPasswordResetEmail(profile.Email, resetURL); err != nil {
		fmt.Printf("Warning: Failed to send reset email: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "If the email exists, a password reset link will be sent",
		"token":   resetToken, // For dev convenience
	})
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req models.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate reset token
	userID, err := h.userRepo.ValidatePasswordResetToken(req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Hash new password
	passwordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Update password
	if err := h.userRepo.UpdatePassword(userID, passwordHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	// Mark token as used
	if err := h.userRepo.MarkPasswordResetTokenAsUsed(req.Token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark token as used"})
		return
	}

	// Revoke all existing sessions for security
	if err := h.userRepo.RevokeAllUserTokens(userID); err != nil {
		// Log error but don't fail the request
		c.JSON(http.StatusOK, gin.H{
			"message": "Password reset successful. Please login with your new password.",
			"warning": "Failed to revoke existing sessions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successful. Please login with your new password."})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user profile
	profile, err := h.userRepo.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}
	if profile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Verify old password
	if !auth.CheckPasswordHash(req.OldPassword, profile.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid old password"})
		return
	}

	// Hash new password
	passwordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Update password
	if err := h.userRepo.UpdatePassword(userID, passwordHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}
