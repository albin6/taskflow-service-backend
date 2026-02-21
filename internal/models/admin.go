package models

type AdminStats struct {
	TotalUsers       int `json:"total_users"`
	PendingApprovals int `json:"pending_approvals"`
	TotalTeams       int `json:"total_teams"`
	TotalTasks       int `json:"total_tasks"`
}
