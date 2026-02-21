// Package authz provides centralized authorization helpers for the Task Flow backend.
// All permission-evaluation logic is consolidated here to avoid handler-level duplication.
package authz

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/taskflow/backend/internal/auth"
	"github.com/taskflow/backend/internal/models"
)

// TeamManagerChecker is the minimal interface needed to evaluate team-manager permissions.
// Implemented by *repository.TaskRepository.
type TeamManagerChecker interface {
	IsManagerOfTeam(userID, teamID uuid.UUID) (bool, error)
}

// IsTeamManager evaluates whether the authenticated user is a manager (head/lead/task-manager)
// of the given team OR is a system admin. Returns (isAuthorized, actorID, error).
//
// All 5 duplicated blocks in custom_role.go share this exact pattern:
//
//	canCreate := userRole == models.RoleAdmin
//	if !canCreate { isManager, err := h.taskRepo.IsManagerOfTeam(userID, teamID) ... }
func IsTeamManager(c *gin.Context, checker TeamManagerChecker, teamID uuid.UUID) (bool, uuid.UUID, error) {
	userID, _ := auth.GetUserID(c)
	role, _ := auth.GetUserRole(c)

	if role == models.RoleAdmin {
		return true, userID, nil
	}

	isManager, err := checker.IsManagerOfTeam(userID, teamID)
	if err != nil {
		return false, userID, err
	}
	return isManager, userID, nil
}

// RequireTeamManager returns a gin.HandlerFunc that aborts with 403 if the caller
// is neither a system admin nor a manager of the team identified by the "id" URL param.
// Suitable for use as route-level middleware on team-scoped routes.
func RequireTeamManager(checker TeamManagerChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		teamID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
			c.Abort()
			return
		}

		ok, _, err := IsTeamManager(c, checker, teamID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
			c.Abort()
			return
		}
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: team manager or admin role required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAdmin is a convenience wrapper identical to auth.AdminOnly() but housed here
// alongside other authorization helpers. auth.AdminOnly() continues to work unchanged.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get(auth.UserRoleKey)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
			c.Abort()
			return
		}
		if role.(models.AppRole) != models.RoleAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireOwnerOrAdmin aborts with 403 unless the authenticated user equals ownerID or is admin.
func RequireOwnerOrAdmin(c *gin.Context, ownerID uuid.UUID) bool {
	userID, _ := auth.GetUserID(c)
	role, _ := auth.GetUserRole(c)
	return userID == ownerID || role == models.RoleAdmin
}
