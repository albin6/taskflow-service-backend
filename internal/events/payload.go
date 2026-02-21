// Package events defines the canonical payload types for all Redis Pub/Sub events
// published by the Task Flow backend. These structs form the cross-service contract
// between the backend and any subscriber service (e.g. the API/Follow-Up service).
//
// Versioning strategy:
//   - Channel names are prefixed: "taskflow:v1:<entity>:<action>"
//   - Old short channel names are kept as aliases for backward compatibility
//   - Payload fields are additive only — never remove or rename existing fields
package events

import (
	"time"

	"github.com/google/uuid"
)

// EventVersion identifies the payload schema version.
const EventVersion = "v1"

// EventSource identifies the originating service.
const EventSource = "taskflow-backend"

// Envelope wraps every published event with tracing metadata.
// Subscribers MUST handle unknown action values gracefully.
type Envelope struct {
	Version    string    `json:"version"`     // e.g. "v1"
	Source     string    `json:"source"`      // e.g. "taskflow-backend"
	Action     string    `json:"action"`      // e.g. "user.created"
	OccurredAt time.Time `json:"occurred_at"` // UTC timestamp of the event
	Payload    any       `json:"payload"`     // concrete payload struct
}

// UserEventPayload is the canonical payload for all user-lifecycle events.
// It is a strict superset of the legacy UserEvent struct.
//
// API subscriber compatibility:
//   - The subscriber in API/internal/adapters/redis/client.go must mirror these fields.
//   - Fields added in future versions will default to zero-value on old subscribers.
type UserEventPayload struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Role       string    `json:"role"`
	IsApproved bool      `json:"is_approved"`
	AvatarURL  *string   `json:"avatar_url,omitempty"`
}

// Action constants for Envelope.Action field.
const (
	ActionUserCreated  = "user.created"
	ActionUserUpdated  = "user.updated"
	ActionUserApproved = "user.approved"
	ActionUserDeleted  = "user.deleted"
	ActionRoleChanged  = "user.role_changed"
)
