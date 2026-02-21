package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/taskflow/backend/internal/authz"
	"github.com/taskflow/backend/internal/models"
	"github.com/taskflow/backend/internal/repository"
)

type CustomRoleHandler struct {
	roleRepo *repository.CustomRoleRepository
	taskRepo *repository.TaskRepository
}

func NewCustomRoleHandler(roleRepo *repository.CustomRoleRepository, taskRepo *repository.TaskRepository) *CustomRoleHandler {
	return &CustomRoleHandler{
		roleRepo: roleRepo,
		taskRepo: taskRepo,
	}
}

func (h *CustomRoleHandler) CreateCustomRole(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	var req models.CreateCustomRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user is admin or team lead/head
	ok, userID, err := authz.IsTeamManager(c, h.taskRepo, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
		return
	}
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins and team managers can create custom roles"})
		return
	}

	role := &models.CustomRole{
		TeamID:                  teamID,
		RoleName:                req.RoleName,
		CanManageTasks:          req.CanManageTasks,
		CanViewAnalytics:        req.CanViewAnalytics,
		CanManageMembersLimited: req.CanManageMembersLimited,
		CreatedBy:               &userID,
	}

	if err := h.roleRepo.Create(role); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create custom role"})
		return
	}

	c.JSON(http.StatusCreated, role)
}

func (h *CustomRoleHandler) GetTeamRoles(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	roles, err := h.roleRepo.GetByTeamID(teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch roles: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, roles)
}

func (h *CustomRoleHandler) UpdateCustomRole(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	roleID, err := uuid.Parse(c.Param("roleId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	var req models.UpdateCustomRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check permissions
	ok, _, err := authz.IsTeamManager(c, h.taskRepo, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
		return
	}
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.roleRepo.Update(roleID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role updated successfully"})
}

func (h *CustomRoleHandler) DeleteCustomRole(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	roleID, err := uuid.Parse(c.Param("roleId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	// Check permissions
	ok, _, err := authz.IsTeamManager(c, h.taskRepo, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
		return
	}
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.roleRepo.Delete(roleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role deleted successfully"})
}

func (h *CustomRoleHandler) AssignRoleToMember(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	memberID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid member ID"})
		return
	}

	var req models.AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check permissions
	ok, _, err := authz.IsTeamManager(c, h.taskRepo, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
		return
	}
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.roleRepo.AssignRoleToMember(teamID, memberID, req.CustomRoleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role assigned successfully"})
}

func (h *CustomRoleHandler) UpdateMemberPosition(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	memberID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid member ID"})
		return
	}

	var req models.UpdateMemberPositionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check permissions
	ok, _, err := authz.IsTeamManager(c, h.taskRepo, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
		return
	}
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.roleRepo.UpdateMemberPosition(teamID, memberID, req.Position); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update position"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Position updated successfully"})
}
