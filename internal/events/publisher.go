// Package events handles Redis Pub/Sub publishing for the Task Flow backend.
// Channel naming convention: "taskflow:v1:<entity>:<action>"
// Backward-compatible short names are retained as package-level constants.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/taskflow/backend/internal/database"
	"github.com/taskflow/backend/internal/models"
)

// ---------------------------------------------------------------------------
// Channel name constants — versioned + backward-compat aliases
// ---------------------------------------------------------------------------

const (
	// Versioned channel names (preferred for new subscribers)
	ChanUserCreated  = "taskflow:v1:user:created"
	ChanUserUpdated  = "taskflow:v1:user:updated"
	ChanUserApproved = "taskflow:v1:user:approved"
	ChanUserDeleted  = "taskflow:v1:user:deleted"
	ChanRoleChanged  = "taskflow:v1:user:role_changed"

	// Legacy short names — kept for backward compatibility with the API subscriber.
	// Do not remove until all subscribers are migrated to versioned channels.
	UserCreatedChannel = "user:created"
	UserUpdatedChannel = "user:updated"
)

// ---------------------------------------------------------------------------
// Legacy UserEvent struct — kept as-is for backward-compat with old subscribers.
// New code should use UserEventPayload from payload.go instead.
// ---------------------------------------------------------------------------

// UserEvent is the legacy payload shape; still published on the short channels.
type UserEvent struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func buildPayload(user *models.Profile, role models.AppRole) UserEventPayload {
	name := ""
	if user.FullName != nil {
		name = *user.FullName
	}
	return UserEventPayload{
		ID:         user.ID,
		Name:       name,
		Email:      user.Email,
		Role:       string(role),
		IsApproved: user.IsApproved,
		AvatarURL:  user.AvatarURL,
	}
}

func publish(channel string, payload any) error {
	if database.RedisClient == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return database.RedisClient.Publish(context.Background(), channel, data).Err()
}

func publishEnvelope(channel, action string, payload any) error {
	if database.RedisClient == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	env := Envelope{
		Version:    EventVersion,
		Source:     EventSource,
		Action:     action,
		OccurredAt: time.Now().UTC(),
		Payload:    payload,
	}
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return database.RedisClient.Publish(context.Background(), channel, data).Err()
}

// ---------------------------------------------------------------------------
// Public publish functions
// ---------------------------------------------------------------------------

// PublishUserCreated publishes on both the legacy and versioned channels so that
// old and new subscribers both receive the event during migration.
func PublishUserCreated(user *models.Profile) error {
	if database.RedisClient == nil {
		return fmt.Errorf("redis client is not initialized")
	}

	name := ""
	if user.FullName != nil {
		name = *user.FullName
	}

	// Legacy payload (short channel)
	legacy := UserEvent{
		ID:    user.ID,
		Name:  name,
		Email: user.Email,
		Role:  string(models.RoleUser),
	}
	if err := publish(UserCreatedChannel, legacy); err != nil {
		return err
	}

	// Versioned payload (new channel)
	p := buildPayload(user, models.RoleUser)
	return publishEnvelope(ChanUserCreated, ActionUserCreated, p)
}

// PublishUserUpdated publishes on both channels.
func PublishUserUpdated(user *models.Profile, role models.AppRole) error {
	if database.RedisClient == nil {
		return fmt.Errorf("redis client is not initialized")
	}

	name := ""
	if user.FullName != nil {
		name = *user.FullName
	}

	// Legacy payload (short channel)
	legacy := UserEvent{
		ID:        user.ID,
		Name:      name,
		Email:     user.Email,
		Role:      string(role),
		AvatarURL: user.AvatarURL,
	}
	if err := publish(UserUpdatedChannel, legacy); err != nil {
		return err
	}

	// Versioned payload (new channel)
	p := buildPayload(user, role)
	return publishEnvelope(ChanUserUpdated, ActionUserUpdated, p)
}

// PublishUserApproved publishes on the versioned channel only.
func PublishUserApproved(user *models.Profile, approvedRole models.AppRole) error {
	p := buildPayload(user, approvedRole)
	p.IsApproved = true
	return publishEnvelope(ChanUserApproved, ActionUserApproved, p)
}

// PublishUserDeleted publishes on the versioned channel only.
func PublishUserDeleted(userID uuid.UUID, email string) error {
	p := UserEventPayload{
		ID:    userID,
		Email: email,
	}
	return publishEnvelope(ChanUserDeleted, ActionUserDeleted, p)
}

// PublishRoleChanged publishes on the versioned channel only.
func PublishRoleChanged(user *models.Profile, newRole models.AppRole) error {
	p := buildPayload(user, newRole)
	return publishEnvelope(ChanRoleChanged, ActionRoleChanged, p)
}
