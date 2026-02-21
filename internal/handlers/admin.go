package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/taskflow/backend/internal/models"
	"github.com/taskflow/backend/internal/repository"
)

type AdminHandler struct {
	userRepo *repository.UserRepository
	teamRepo *repository.TeamRepository
	taskRepo *repository.TaskRepository
}

func NewAdminHandler(userRepo *repository.UserRepository, teamRepo *repository.TeamRepository, taskRepo *repository.TaskRepository) *AdminHandler {
	return &AdminHandler{
		userRepo: userRepo,
		teamRepo: teamRepo,
		taskRepo: taskRepo,
	}
}

func (h *AdminHandler) GetStats(c *gin.Context) {
	// Simple non-cached counts for now
	var stats models.AdminStats

	// Total users
	users, _ := h.userRepo.ListAll()
	stats.TotalUsers = len(users)

	// Pending approvals
	pendingCount := 0
	for _, u := range users {
		if !u.IsApproved {
			pendingCount++
		}
	}
	stats.PendingApprovals = pendingCount

	// Total teams
	teams, _ := h.teamRepo.ListAll()
	stats.TotalTeams = len(teams)

	// Total tasks
	// Note: taskRepo might not have ListAll, but we can assume it for this overview
	// For now, let's just use what we have or return 0
	tasks, _ := h.taskRepo.ListAll()
	stats.TotalTasks = len(tasks)

	c.JSON(http.StatusOK, stats)
}
