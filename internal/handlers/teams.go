package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/taskflow/backend/internal/audit"
	"github.com/taskflow/backend/internal/auth"
	"github.com/taskflow/backend/internal/models"
	"github.com/taskflow/backend/internal/repository"
)

type TeamHandler struct {
	teamRepo    *repository.TeamRepository
	userRepo    *repository.UserRepository
	auditLogger *audit.Logger
}

func NewTeamHandler(teamRepo *repository.TeamRepository, userRepo *repository.UserRepository, auditLogger *audit.Logger) *TeamHandler {
	return &TeamHandler{
		teamRepo:    teamRepo,
		userRepo:    userRepo,
		auditLogger: auditLogger,
	}
}

// CreateTeam creates a new team (admin only)
func (h *TeamHandler) CreateTeam(c *gin.Context) {
	userID, _ := auth.GetUserID(c)

	var req models.CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create the team
	team, err := h.teamRepo.CreateTeam(req.Name, req.Description, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create team"})
		return
	}

	// If a head user is specified, add them as team head
	if req.HeadUserID != nil {
		err = h.teamRepo.AddMember(team.ID, *req.HeadUserID, nil, true, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add team head"})
			return
		}
	}

	actorEmail, _ := auth.GetUserEmail(c)
	h.auditLogger.Log(userID, actorEmail, audit.ActionTeamCreated, "team", team.ID.String(), nil, map[string]string{"name": team.Name})

	c.JSON(http.StatusCreated, team)
}

// ListTeams retrieves all teams
func (h *TeamHandler) ListTeams(c *gin.Context) {
	teams, err := h.teamRepo.ListTeams()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list teams"})
		return
	}

	c.JSON(http.StatusOK, teams)
}

// GetTeam retrieves a specific team with members
func (h *TeamHandler) GetTeam(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	team, err := h.teamRepo.GetTeamWithMembers(teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get team"})
		return
	}
	if team == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return
	}

	c.JSON(http.StatusOK, team)
}

// GetUserTeams retrieves all teams the current user belongs to
func (h *TeamHandler) GetUserTeams(c *gin.Context) {
	userID, _ := auth.GetUserID(c)

	teams, err := h.teamRepo.ListUserTeams(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user teams"})
		return
	}

	c.JSON(http.StatusOK, teams)
}

// UpdateTeam updates team information
func (h *TeamHandler) UpdateTeam(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	userID, _ := auth.GetUserID(c)
	role, _ := auth.GetUserRole(c)

	// Check if user is admin or team head
	if role != models.RoleAdmin {
		member, err := h.teamRepo.GetMemberRole(userID, teamID)
		if err != nil || member == nil || !member.IsHead {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only team heads can update team information"})
			return
		}
	}

	var req models.UpdateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.teamRepo.UpdateTeam(teamID, req.Name, req.Description); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update team"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Team updated successfully"})
}

// DeleteTeam deletes a team (admin only)
func (h *TeamHandler) DeleteTeam(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	if err := h.teamRepo.DeleteTeam(teamID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete team"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Team deleted successfully"})
}

// AddMember adds a member to a team
func (h *TeamHandler) AddMember(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	userID, _ := auth.GetUserID(c)
	role, _ := auth.GetUserRole(c)

	// Check if user is admin or team head/lead
	if role != models.RoleAdmin {
		member, err := h.teamRepo.GetMemberRole(userID, teamID)
		if err != nil || member == nil || (!member.IsHead && !member.IsLead) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only team heads and leads can add members"})
			return
		}
	}

	var req models.AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user exists
	user, err := h.userRepo.GetUser(req.UserID)
	if err != nil || user == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User not found"})
		return
	}

	// Check if already a member
	isMember, err := h.teamRepo.IsMemberOfTeam(req.UserID, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check membership"})
		return
	}
	if isMember {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User is already a member of this team"})
		return
	}

	if err := h.teamRepo.AddMember(teamID, req.UserID, req.RoleID, req.IsHead, req.IsLead); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add member"})
		return
	}

	actorID2, _ := auth.GetUserID(c)
	actorEmail2, _ := auth.GetUserEmail(c)
	h.auditLogger.Log(actorID2, actorEmail2, audit.ActionMemberAdded, "team", teamID.String(), nil, map[string]string{"user_id": req.UserID.String()})

	c.JSON(http.StatusOK, gin.H{"message": "Member added successfully"})
}

// RemoveMember removes a member from a team
func (h *TeamHandler) RemoveMember(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	memberUserID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	userID, _ := auth.GetUserID(c)
	role, _ := auth.GetUserRole(c)

	// Check if user is admin or team head
	if role != models.RoleAdmin {
		member, err := h.teamRepo.GetMemberRole(userID, teamID)
		if err != nil || member == nil || !member.IsHead {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only team heads can remove members"})
			return
		}
	}

	if err := h.teamRepo.RemoveMember(teamID, memberUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove member"})
		return
	}

	actorEmail3, _ := auth.GetUserEmail(c)
	h.auditLogger.Log(userID, actorEmail3, audit.ActionMemberRemoved, "team", teamID.String(), nil, map[string]string{"user_id": memberUserID.String()})

	c.JSON(http.StatusOK, gin.H{"message": "Member removed successfully"})
}

// UpdateMemberRole updates a member's role in a team
func (h *TeamHandler) UpdateMemberRole(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	memberUserID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	userID, _ := auth.GetUserID(c)
	role, _ := auth.GetUserRole(c)

	// Check if user is admin or team head
	if role != models.RoleAdmin {
		member, err := h.teamRepo.GetMemberRole(userID, teamID)
		if err != nil || member == nil || !member.IsHead {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only team heads can update member roles"})
			return
		}
	}

	var req models.UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.teamRepo.UpdateMemberRole(teamID, memberUserID, req.RoleID, req.IsHead, req.IsLead); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update member role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member role updated successfully"})
}

// CreateRole creates a custom role for a team
func (h *TeamHandler) CreateRole(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	userID, _ := auth.GetUserID(c)
	role, _ := auth.GetUserRole(c)

	// Check if user is admin or team head/lead
	if role != models.RoleAdmin {
		member, err := h.teamRepo.GetMemberRole(userID, teamID)
		if err != nil || member == nil || (!member.IsHead && !member.IsLead) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only team heads and leads can create roles"})
			return
		}
	}

	var req models.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	customRole, err := h.teamRepo.CreateCustomRole(teamID, req.RoleName, req.Permissions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create role"})
		return
	}

	c.JSON(http.StatusCreated, customRole)
}

// GetTeamRoles retrieves all custom roles for a team
func (h *TeamHandler) GetTeamRoles(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	roles, err := h.teamRepo.GetTeamRoles(teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get team roles"})
		return
	}

	c.JSON(http.StatusOK, roles)
}

// GetAssignableUsers retrieves users that the current user can assign tasks to
func (h *TeamHandler) GetAssignableUsers(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	userID, _ := auth.GetUserID(c)

	users, err := h.teamRepo.GetAssignableUsers(userID, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get assignable users"})
		return
	}

	c.JSON(http.StatusOK, users)
}
