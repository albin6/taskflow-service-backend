package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/taskflow/backend/internal/audit"
	"github.com/taskflow/backend/internal/auth"
	"github.com/taskflow/backend/internal/models"
	"github.com/taskflow/backend/internal/repository"
)

type TaskHandler struct {
	taskRepo    *repository.TaskRepository
	teamRepo    *repository.TeamRepository
	userRepo    *repository.UserRepository
	auditLogger *audit.Logger
}

func NewTaskHandler(taskRepo *repository.TaskRepository, teamRepo *repository.TeamRepository, userRepo *repository.UserRepository, auditLogger *audit.Logger) *TaskHandler {
	return &TaskHandler{
		taskRepo:    taskRepo,
		teamRepo:    teamRepo,
		userRepo:    userRepo,
		auditLogger: auditLogger,
	}
}

func (h *TaskHandler) CreateTask(c *gin.Context) {
	var req models.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate deadline (cannot be before today)
	now := time.Now().Truncate(24 * time.Hour)
	deadlineDay := req.Deadline.Truncate(24 * time.Hour)
	if deadlineDay.Before(now) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Deadline cannot be in the past"})
		return
	}

	userID, _ := auth.GetUserID(c)
	role, _ := auth.GetUserRole(c)

	// If not admin, check if user has permission to assign to the target user
	if role != models.RoleAdmin {
		// Allow self-assignment (user creating task for themselves)
		if userID != req.AssignedUserID {
			// If not self-assignment, check if user is a manager of the target user
			isManager, err := h.taskRepo.IsManagerOf(userID, req.AssignedUserID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
				return
			}

			// If not a manager, check if both are regular members of the same team
			if !isManager {
				// Check if both users are in the same team
				areBothMembers := false
				if req.TeamID != nil {
					areBothMembers, err = h.teamRepo.AreBothRegularMembers(userID, req.AssignedUserID, *req.TeamID)
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify team membership"})
						return
					}
				}

				if !areBothMembers {
					c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to assign tasks to this user"})
					return
				}
			}
		}
	}

	task := &models.Task{
		TaskName:       req.TaskName,
		AssignedUserID: req.AssignedUserID,
		Deadline:       req.Deadline,
		CreatedBy:      userID,
		Status:         models.TaskStatusTodo,
		TeamID:         req.TeamID,
	}

	if err := h.taskRepo.Create(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task"})
		return
	}

	actorEmail, _ := auth.GetUserEmail(c)
	h.auditLogger.Log(userID, actorEmail, audit.ActionTaskCreated, "task", task.ID.String(), nil, map[string]string{"task_name": task.TaskName})

	c.JSON(http.StatusCreated, task)
}

func (h *TaskHandler) ListTasks(c *gin.Context) {
	userID, _ := auth.GetUserID(c)
	role, _ := auth.GetUserRole(c)

	params := models.TaskQueryParams{}

	// Parse query parameters
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			params.Limit = limit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			params.Offset = offset
		}
	}

	// Filter by status if provided
	if statusStr := c.Query("status"); statusStr != "" {
		status := models.TaskStatus(statusStr)
		params.Status = &status
	}

	// Filter by search term if provided
	if search := c.Query("search"); search != "" {
		params.Search = strings.TrimSpace(search)
	}

	// Parse team_id if provided
	var teamID *uuid.UUID
	if teamIDStr := c.Query("team_id"); teamIDStr != "" {
		if id, err := uuid.Parse(teamIDStr); err == nil {
			teamID = &id
			params.TeamID = teamID
		}
	}

	// Permission logic
	if role != models.RoleAdmin {
		if teamID != nil {
			// If filtering by team, check if user is manager of that team
			isManager, err := h.taskRepo.IsManagerOfTeam(userID, *teamID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
				return
			}

			if isManager {
				// Manager can see everything in this team
				// They can also optionally filter by member within this team
				if assignedUserStr := c.Query("assigned_user_id"); assignedUserStr != "" {
					if assignedUserID, err := uuid.Parse(assignedUserStr); err == nil {
						params.AssignedUserID = &assignedUserID
					}
				}
			} else {
				// Not a manager of this team, only show their own tasks in this team
				params.AssignedUserID = &userID
			}
		} else {
			// No team specified
			// Check if user is explicitly filtering by assigned_user_id
			if assignedUserStr := c.Query("assigned_user_id"); assignedUserStr != "" {
				if assignedUserID, err := uuid.Parse(assignedUserStr); err == nil {
					// Allow filtering by assigned_user_id if it's the current user
					if assignedUserID == userID {
						params.AssignedUserID = &assignedUserID
					} else {
						c.JSON(http.StatusForbidden, gin.H{"error": "You can only query tasks assigned to yourself"})
						return
					}
				}
			} else if createdByStr := c.Query("created_by"); createdByStr != "" {
				// Check if user is querying by created_by (their own tasks they created)
				if createdByID, err := uuid.Parse(createdByStr); err == nil {
					// Only allow querying tasks created by themselves
					if createdByID == userID {
						params.CreatedBy = &createdByID
					} else {
						c.JSON(http.StatusForbidden, gin.H{"error": "You can only query tasks created by yourself"})
						return
					}
				}
			} else {
				// No specific filter - check if user has can_manage_tasks permission
				managedTeams, err := h.teamRepo.GetUserManagedTeams(userID)
				if err == nil && len(managedTeams) > 0 {
					// User can manage tasks - show all tasks from their managed teams
					params.TeamIDs = managedTeams
				} else {
					// Default: only show tasks assigned to them
					params.AssignedUserID = &userID
				}
			}
		}
	} else {
		// Admin can filter by anything
		if assignedUserStr := c.Query("assigned_user_id"); assignedUserStr != "" {
			if assignedUserID, err := uuid.Parse(assignedUserStr); err == nil {
				params.AssignedUserID = &assignedUserID
			}
		}
		if createdByStr := c.Query("created_by"); createdByStr != "" {
			if createdByID, err := uuid.Parse(createdByStr); err == nil {
				params.CreatedBy = &createdByID
			}
		}
	}

	tasks, err := h.taskRepo.List(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tasks"})
		return
	}

	// Get total count
	count, _ := h.taskRepo.Count(params)

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"total": count,
	})
}

func (h *TaskHandler) GetTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	task, err := h.taskRepo.GetWithDetails(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get task"})
		return
	}
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// Check authorization
	userID, _ := auth.GetUserID(c)
	role, _ := auth.GetUserRole(c)
	if role != models.RoleAdmin && task.AssignedUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) UpdateTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	// Fetch task to check team_id
	task, err := h.taskRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch task"})
		return
	}
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// Check permission
	userID, _ := auth.GetUserID(c)
	role, _ := auth.GetUserRole(c)
	canUpdate := role == models.RoleAdmin
	if !canUpdate && task.TeamID != nil {
		isManager, _ := h.taskRepo.IsManagerOfTeam(userID, *task.TeamID)
		canUpdate = isManager
	}

	if !canUpdate {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin or Team Lead access required"})
		return
	}

	var req models.UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.TaskName != nil {
		updates["task_name"] = *req.TaskName
	}
	if req.AssignedUserID != nil {
		updates["assigned_user_id"] = *req.AssignedUserID
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Deadline != nil {
		// Validate deadline if it's being updated (cannot be before today)
		now := time.Now().Truncate(24 * time.Hour)
		deadlineDay := req.Deadline.Truncate(24 * time.Hour)
		if deadlineDay.Before(now) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Deadline cannot be in the past"})
			return
		}
		updates["deadline"] = *req.Deadline
	}

	if err := h.taskRepo.Update(id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update task"})
		return
	}

	actorID, _ := auth.GetUserID(c)
	actorEmail, _ := auth.GetUserEmail(c)
	h.auditLogger.Log(actorID, actorEmail, audit.ActionTaskUpdated, "task", id.String(), nil, updates)

	c.JSON(http.StatusOK, gin.H{"message": "Task updated successfully"})
}

func (h *TaskHandler) UpdateTaskStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	var req models.UpdateTaskStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Fetch task to check permissions
	task, err := h.taskRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch task"})
		return
	}
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// Get user info
	userID, _ := auth.GetUserID(c)
	userRole, _ := auth.GetUserRole(c)

	// Check permission: Admin, Assignee, Team Lead, or Task Creator (for marking as completed)
	canUpdate := userRole == models.RoleAdmin || task.AssignedUserID == userID

	// Allow task creator to mark as completed if status is being changed to completed
	if !canUpdate && task.CreatedBy == userID && req.Status == "completed" {
		canUpdate = true
	}

	if !canUpdate && task.TeamID != nil {
		isManager, _ := h.taskRepo.IsManagerOfTeam(userID, *task.TeamID)
		canUpdate = isManager
	}

	if !canUpdate {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.taskRepo.UpdateStatus(id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update task status"})
		return
	}

	actorEmail, _ := auth.GetUserEmail(c)
	h.auditLogger.Log(userID, actorEmail, audit.ActionTaskStatusChanged, "task", id.String(), nil, map[string]string{"status": string(req.Status)})

	c.JSON(http.StatusOK, gin.H{"message": "Task status updated successfully"})
}

func (h *TaskHandler) DeleteTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	// Get user info from context
	userID, _ := auth.GetUserID(c)
	userRole, _ := auth.GetUserRole(c)

	// Fetch task to check team_id
	task, err := h.taskRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch task"})
		return
	}
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// Permission check: Admin or Team Lead/Head
	canDelete := userRole == models.RoleAdmin

	if !canDelete && task.TeamID != nil {
		// Check if user is lead or head of the team
		isManager, err := h.taskRepo.IsManagerOfTeam(userID, *task.TeamID)
		if err == nil && isManager {
			canDelete = true
		}
	}

	if !canDelete {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin or Team Lead access required to delete this task"})
		return
	}

	if err := h.taskRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete task"})
		return
	}

	actorEmail, _ := auth.GetUserEmail(c)
	h.auditLogger.Log(userID, actorEmail, audit.ActionTaskDeleted, "task", id.String(), nil, nil)

	c.JSON(http.StatusOK, gin.H{"message": "Task deleted successfully"})
}

func (h *TaskHandler) RunLoadTest(c *gin.Context) {
	var (
		users             []models.UserListResponse
		tasks             []models.TaskWithDetails
		teams             []models.Team
		errU, errT, errTm error
		wg                sync.WaitGroup
	)

	wg.Add(3)

	go func() {
		defer wg.Done()
		users, errU = h.userRepo.ListAll()
	}()

	go func() {
		defer wg.Done()
		tasks, errT = h.taskRepo.List(models.TaskQueryParams{})
	}()

	go func() {
		defer wg.Done()
		teams, errTm = h.teamRepo.ListTeams()
	}()

	wg.Wait()

	if errU != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users: " + errU.Error()})
		return
	}

	if errT != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tasks: " + errT.Error()})
		return
	}

	if errTm != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch teams: " + errTm.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_count": len(users),
		"task_count": len(tasks),
		"team_count": len(teams),
		"users":      users,
		"tasks":      tasks,
		"teams":      teams,
	})
}
