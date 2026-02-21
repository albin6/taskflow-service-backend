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

type DeadlineHandler struct {
	deadlineRepo *repository.DeadlineRepository
	taskRepo     *repository.TaskRepository
	auditLogger  *audit.Logger
}

func NewDeadlineHandler(deadlineRepo *repository.DeadlineRepository, taskRepo *repository.TaskRepository, auditLogger *audit.Logger) *DeadlineHandler {
	return &DeadlineHandler{
		deadlineRepo: deadlineRepo,
		taskRepo:     taskRepo,
		auditLogger:  auditLogger,
	}
}

func (h *DeadlineHandler) CreateRequest(c *gin.Context) {
	var req models.CreateDeadlineRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := auth.GetUserID(c)

	// Get task to verify it exists and get current deadline
	task, err := h.taskRepo.GetByID(req.TaskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get task"})
		return
	}
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// Verify user is assigned to the task
	if task.AssignedUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only request deadline changes for your own tasks"})
		return
	}

	deadlineReq := &models.DeadlineRequest{
		TaskID:            req.TaskID,
		RequestedBy:       userID,
		CurrentDeadline:   task.Deadline,
		RequestedDeadline: req.RequestedDeadline,
		Reason:            req.Reason,
	}

	if err := h.deadlineRepo.Create(deadlineReq); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create deadline request"})
		return
	}

	c.JSON(http.StatusCreated, deadlineReq)
}

func (h *DeadlineHandler) ListRequests(c *gin.Context) {
	userID, _ := auth.GetUserID(c)
	role, _ := auth.GetUserRole(c)

	requests, err := h.deadlineRepo.ListForUserOrManager(userID, role == models.RoleAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list deadline requests"})
		return
	}

	c.JSON(http.StatusOK, requests)
}

func (h *DeadlineHandler) GetRequest(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	request, err := h.deadlineRepo.GetWithDetails(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get deadline request"})
		return
	}
	if request == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deadline request not found"})
		return
	}

	// Check authorization
	userID, _ := auth.GetUserID(c)
	role, _ := auth.GetUserRole(c)
	if role != models.RoleAdmin && request.RequestedBy != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, request)
}

func (h *DeadlineHandler) ApproveRequest(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	userID, _ := auth.GetUserID(c)

	// Get the request
	request, err := h.deadlineRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get deadline request"})
		return
	}
	if request == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deadline request not found"})
		return
	}

	// Update request status
	if err := h.deadlineRepo.UpdateStatus(id, models.RequestStatusApproved, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve request"})
		return
	}

	// Update task deadline
	updates := map[string]interface{}{
		"deadline": request.RequestedDeadline,
	}
	if err := h.taskRepo.Update(request.TaskID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update task deadline"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deadline request approved successfully"})
}

func (h *DeadlineHandler) RejectRequest(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	userID, _ := auth.GetUserID(c)

	if err := h.deadlineRepo.UpdateStatus(id, models.RequestStatusRejected, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject request"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deadline request rejected successfully"})
}
