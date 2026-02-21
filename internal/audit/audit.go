package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
)

type Action string

const (
	ActionUserCreated       Action = "USER_CREATED"
	ActionUserApproved      Action = "USER_APPROVED"
	ActionUserDeleted       Action = "USER_DELETED"
	ActionRoleChanged       Action = "ROLE_CHANGED"
	ActionTaskCreated       Action = "TASK_CREATED"
	ActionTaskUpdated       Action = "TASK_UPDATED"
	ActionTaskDeleted       Action = "TASK_DELETED"
	ActionTaskStatusChanged Action = "TASK_STATUS_CHANGED"
	ActionTeamCreated       Action = "TEAM_CREATED"
	ActionTeamDeleted       Action = "TEAM_DELETED"
	ActionMemberAdded       Action = "MEMBER_ADDED"
	ActionMemberRemoved     Action = "MEMBER_REMOVED"
	ActionDeadlineApproved  Action = "DEADLINE_APPROVED"
	ActionDeadlineRejected  Action = "DEADLINE_REJECTED"
	ActionPasswordReset     Action = "PASSWORD_RESET"
)

type AuditLog struct {
	ID         uuid.UUID  `json:"id"`
	ActorID    *uuid.UUID `json:"actor_id"`
	ActorEmail string     `json:"actor_email"`
	Action     Action     `json:"action"`
	EntityType string     `json:"entity_type"`
	EntityID   string     `json:"entity_id"`
	OldValue   *string    `json:"old_value,omitempty"`
	NewValue   *string    `json:"new_value,omitempty"`
	IPAddress  *string    `json:"ip_address,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Logger struct {
	db *sql.DB
}

func NewLogger(db *sql.DB) *Logger {
	return &Logger{db: db}
}

func (l *Logger) Log(actorID uuid.UUID, actorEmail string, action Action, entityType, entityID string, oldVal, newVal interface{}) {
	go func() {
		var oldJSON, newJSON *string

		if oldVal != nil {
			b, err := json.Marshal(oldVal)
			if err == nil {
				s := string(b)
				oldJSON = &s
			}
		}

		if newVal != nil {
			b, err := json.Marshal(newVal)
			if err == nil {
				s := string(b)
				newJSON = &s
			}
		}

		query := `
			INSERT INTO audit_logs (actor_id, actor_email, action, entity_type, entity_id, old_value, new_value, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		`
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := l.db.ExecContext(ctx, query,
			actorID, actorEmail, string(action), entityType, entityID,
			oldJSON, newJSON,
		)
		if err != nil {
			log.Printf("audit: failed to write log [%s %s/%s]: %v", action, entityType, entityID, err)
		}
	}()
}
