package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/taskflow/backend/internal/audit"
	"github.com/taskflow/backend/internal/auth"
	"github.com/taskflow/backend/internal/events"
	"github.com/taskflow/backend/internal/mail"
	"github.com/taskflow/backend/internal/models"
	"github.com/taskflow/backend/internal/repository"
)

type UserHandler struct {
	userRepo     *repository.UserRepository
	teamRepo     *repository.TeamRepository
	auditLogger  *audit.Logger
	notifRepo    *repository.NotificationRepository
	emailService *mail.EmailService
}

func NewUserHandler(
	userRepo *repository.UserRepository,
	teamRepo *repository.TeamRepository,
	auditLogger *audit.Logger,
	notifRepo *repository.NotificationRepository,
	emailService *mail.EmailService,
) *UserHandler {
	return &UserHandler{
		userRepo:     userRepo,
		teamRepo:     teamRepo,
		auditLogger:  auditLogger,
		notifRepo:    notifRepo,
		emailService: emailService,
	}
}

func (h *UserHandler) ListUsers(c *gin.Context) {
	userID, _ := auth.GetUserID(c)
	role, _ := auth.GetUserRole(c)

	// Admin gets the paginated, filtered view
	if role == models.RoleAdmin {
		search := c.Query("search")
		roleFilter := c.Query("role")
		statusFilter := c.Query("status")
		sortBy := c.Query("sort_by")
		sortDir := c.Query("sort_dir")
		limit := 20
		offset := 0
		if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 && v <= 100 {
			limit = v
		}
		if v, err := strconv.Atoi(c.Query("offset")); err == nil && v >= 0 {
			offset = v
		}

		// If no filter params, fall back to fast cached ListAll
		if search == "" && roleFilter == "" && statusFilter == "" && sortBy == "" {
			allUsers, err := h.userRepo.ListAll()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
				return
			}
			c.JSON(http.StatusOK, models.PaginatedUsersResponse{
				Users: allUsers, Total: len(allUsers), Limit: limit, Offset: offset,
			})
			return
		}

		users, total, err := h.userRepo.ListFiltered(search, roleFilter, statusFilter, sortBy, sortDir, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
			return
		}
		c.JSON(http.StatusOK, models.PaginatedUsersResponse{
			Users: users, Total: total, Limit: limit, Offset: offset,
		})
		return
	}

	// Non-admins keep the existing team-filtered view (unchanged)
	allUsers, err := h.userRepo.ListAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}

	userTeams, err := h.teamRepo.ListUserTeams(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user teams"})
		return
	}

	ledTeamIDs := make(map[uuid.UUID]bool)
	for _, ut := range userTeams {
		if ut.IsHead || ut.IsLead || ut.CanManageTasks || ut.CanManageMembersLimited {
			ledTeamIDs[ut.TeamID] = true
		}
	}

	if len(ledTeamIDs) == 0 {
		filtered := []models.UserListResponse{}
		for _, u := range allUsers {
			if u.ID == userID {
				filtered = append(filtered, u)
			}
		}
		c.JSON(http.StatusOK, filtered)
		return
	}

	filtered := []models.UserListResponse{}
	teamMemberIDs := make(map[uuid.UUID]bool)
	for teamID := range ledTeamIDs {
		members, _ := h.teamRepo.GetTeamMembers(teamID)
		for _, m := range members {
			teamMemberIDs[m.UserID] = true
		}
	}

	for _, u := range allUsers {
		if u.ID == userID {
			filtered = append(filtered, u)
			continue
		}
		if teamMemberIDs[u.ID] {
			filtered = append(filtered, u)
			continue
		}
		if !u.IsApproved && u.RequestedTeamID != nil && u.RequestedRole != nil {
			if ledTeamIDs[*u.RequestedTeamID] && *u.RequestedRole == "team_member" {
				approvers, err := h.teamRepo.GetTeamApprovers(*u.RequestedTeamID)
				if err == nil {
					for _, approverID := range approvers {
						if approverID == userID {
							filtered = append(filtered, u)
							break
						}
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, filtered)
}

func (h *UserHandler) GetUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.userRepo.GetUser(id)
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

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	userID, _ := auth.GetUserID(c)
	role, _ := auth.GetUserRole(c)
	if userID != id && role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot update other user's profile"})
		return
	}

	var req models.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.userRepo.Update(id, req.FullName, req.AvatarURL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	go func() {
		updatedUser, err := h.userRepo.GetUser(id)
		if err == nil {
			events.PublishUserUpdated(&updatedUser.Profile, updatedUser.Role)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

func (h *UserHandler) DeleteUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	userID, _ := auth.GetUserID(c)
	actorEmail, _ := auth.GetUserEmail(c)
	if userID == id {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete your own account"})
		return
	}

	if err := h.userRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	h.auditLogger.Log(userID, actorEmail, audit.ActionUserDeleted, "user", id.String(), nil, nil)
	go events.PublishUserDeleted(id, actorEmail)

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

func (h *UserHandler) ApproveUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	targetUser, err := h.userRepo.GetUser(id)
	if err != nil || targetUser == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	approverID, _ := auth.GetUserID(c)
	approverRole, _ := auth.GetUserRole(c)
	approverEmail, _ := auth.GetUserEmail(c)

	requestedRole := ""
	if targetUser.Profile.RequestedRole != nil {
		requestedRole = *targetUser.Profile.RequestedRole
	}

	if requestedRole == "admin" || requestedRole == "team_lead" {
		if approverRole != models.RoleAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only system admins can approve admins or team leads"})
			return
		}
	} else {
		if targetUser.Profile.RequestedTeamID == nil {
			if approverRole != models.RoleAdmin {
				c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required for general approval"})
				return
			}
		} else {
			isAuthorized := false
			if approverRole == models.RoleAdmin {
				isAuthorized = true
			} else {
				member, err := h.teamRepo.GetMemberRole(approverID, *targetUser.Profile.RequestedTeamID)
				if err == nil && member != nil && (member.IsHead || member.IsLead) {
					isAuthorized = true
				}
			}

			if !isAuthorized {
				c.JSON(http.StatusForbidden, gin.H{"error": "Only team heads or leads can approve members for this team"})
				return
			}
		}
	}

	if err := h.userRepo.Approve(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve user"})
		return
	}

	if requestedRole != "" {
		if requestedRole == "admin" {
			h.userRepo.SetRole(id, models.RoleAdmin)
		} else {
			h.userRepo.SetRole(id, models.RoleUser)
		}
	}

	if targetUser.Profile.RequestedTeamID != nil {
		isLead := requestedRole == "team_lead"
		exists, _ := h.teamRepo.IsMemberOfTeam(id, *targetUser.Profile.RequestedTeamID)
		if !exists {
			h.teamRepo.AddMember(*targetUser.Profile.RequestedTeamID, id, nil, false, isLead)
		}
	}

	h.auditLogger.Log(approverID, approverEmail, audit.ActionUserApproved, "user", id.String(), nil, map[string]string{"requested_role": requestedRole})

	if h.notifRepo != nil {
		go func() {
			notif := &repository.Notification{
				UserID:  id,
				Type:    "SUCCESS",
				Title:   "Account Approved",
				Message: "Your account has been approved. You can now log in and access the platform.",
			}
			h.notifRepo.Create(context.Background(), notif)
		}()
	}

	go func() {
		updatedUser, err := h.userRepo.GetUser(id)
		if err == nil {
			events.PublishUserUpdated(&updatedUser.Profile, updatedUser.Role)
			events.PublishUserApproved(&updatedUser.Profile, updatedUser.Role)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "User approved successfully"})
}

func (h *UserHandler) PromoteToAdmin(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	actorID, _ := auth.GetUserID(c)
	actorEmail, _ := auth.GetUserEmail(c)

	if err := h.userRepo.SetRole(id, models.RoleAdmin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to promote user"})
		return
	}

	h.auditLogger.Log(actorID, actorEmail, audit.ActionRoleChanged, "user", id.String(), map[string]string{"role": "user"}, map[string]string{"role": "admin"})
	go func() {
		if u, err := h.userRepo.GetUser(id); err == nil {
			events.PublishRoleChanged(&u.Profile, models.RoleAdmin)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "User promoted to admin successfully"})
}

func (h *UserHandler) DemoteToUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	actorID, _ := auth.GetUserID(c)
	actorEmail, _ := auth.GetUserEmail(c)

	if actorID == id {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot demote yourself"})
		return
	}

	if err := h.userRepo.SetRole(id, models.RoleUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to demote user"})
		return
	}

	h.auditLogger.Log(actorID, actorEmail, audit.ActionRoleChanged, "user", id.String(), map[string]string{"role": "admin"}, map[string]string{"role": "user"})
	go func() {
		if u, err := h.userRepo.GetUser(id); err == nil {
			events.PublishRoleChanged(&u.Profile, models.RoleUser)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "User demoted successfully"})
}

// AdminCreateUser allows an admin to create a user account directly.
// If no password is provided, a secure temp password is generated and emailed.
func (h *UserHandler) AdminCreateUser(c *gin.Context) {
	var req models.AdminCreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default role
	if req.Role == "" {
		req.Role = models.RoleUser
	}

	// Check for duplicate email
	existing, err := h.userRepo.GetByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check email"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A user with this email already exists"})
		return
	}

	// Generate temp password if not provided
	tempPassword := req.Password
	generatedPassword := tempPassword == ""
	if generatedPassword {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate password"})
			return
		}
		tempPassword = hex.EncodeToString(b) // 16 hex chars
	}

	passwordHash, err := auth.HashPassword(tempPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	profile, err := h.userRepo.CreateApproved(req.Email, passwordHash, req.Name, req.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	actorID, _ := auth.GetUserID(c)
	actorEmail, _ := auth.GetUserEmail(c)

	// Add to team if specified
	if req.TeamID != nil {
		if err := h.teamRepo.AddMember(*req.TeamID, profile.ID, req.RoleID, false, false); err != nil {
			fmt.Printf("Warning: failed to add user to team: %v\n", err)
		}
	}

	// Send temp password email (non-blocking)
	go func(name, email, pw string) {
		if h.emailService != nil {
			if err := h.emailService.SendTempPasswordEmail(email, name, pw); err != nil {
				fmt.Printf("Warning: failed to send temp password email to %s: %v\n", email, err)
			}
		}
	}(req.Name, req.Email, tempPassword)

	// Audit log
	details := map[string]string{"email": req.Email, "role": string(req.Role)}
	if generatedPassword {
		details["password_type"] = "auto-generated"
	} else {
		details["password_type"] = "admin-provided"
	}
	h.auditLogger.Log(actorID, actorEmail, audit.ActionUserCreated, "user", profile.ID.String(), nil, details)

	// Publish event
	go events.PublishUserCreated(profile)

	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully. Login credentials have been sent to their email.",
		"user_id": profile.ID,
	})
}
