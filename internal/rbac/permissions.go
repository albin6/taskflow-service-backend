// Package rbac provides a structured permission registry for the Task Flow backend.
// It defines named Permission constants and a PermissionSet value-object that wraps
// the existing boolean flags from the database — no schema changes required.
//
// Usage:
//
//	ps := rbac.ResolveFromRole(models.RoleAdmin)
//	if ps.Has(rbac.PermAdmin) { ... }
//
//	ps := rbac.ResolveFromCustomRole(cr)
//	if ps.Has(rbac.PermManageTasks) { ... }
package rbac

import (
	"github.com/taskflow/backend/internal/models"
)

// Permission is a named capability string. Using string constants (not integers or booleans)
// makes permissions self-documenting and easy to log / audit.
type Permission string

const (
	// System-level permissions
	PermAdmin Permission = "admin"

	// Team-scoped permissions (mirrors the existing boolean flags in custom_roles table)
	PermManageTasks          Permission = "manage_tasks"
	PermViewAnalytics        Permission = "view_analytics"
	PermManageMembersLimited Permission = "manage_members_limited"

	// Structural permissions (heads and leads)
	PermTeamHead Permission = "team_head"
	PermTeamLead Permission = "team_lead"

	// Derived composite permission — any management capability
	PermManageTeam Permission = "manage_team"
)

// PermissionSet is an immutable set of resolved permissions for a given user context.
type PermissionSet struct {
	bits map[Permission]bool
}

// Has reports whether the permission set contains the given permission.
func (ps PermissionSet) Has(p Permission) bool {
	return ps.bits[p]
}

// All returns a slice of all granted permissions (useful for logging/auditing).
func (ps PermissionSet) All() []Permission {
	result := make([]Permission, 0, len(ps.bits))
	for p := range ps.bits {
		result = append(result, p)
	}
	return result
}

// grant returns a new PermissionSet with the given permissions added.
func grant(perms ...Permission) PermissionSet {
	bits := make(map[Permission]bool, len(perms))
	for _, p := range perms {
		bits[p] = true
	}
	return PermissionSet{bits: bits}
}

// ResolveFromRole returns the PermissionSet for a global app role.
// Admins get all permissions. Regular users get none globally
// (their team-scoped permissions are resolved separately via ResolveFromCustomRole).
func ResolveFromRole(role models.AppRole) PermissionSet {
	switch role {
	case models.RoleAdmin:
		return grant(
			PermAdmin,
			PermManageTeam,
			PermManageTasks,
			PermViewAnalytics,
			PermManageMembersLimited,
			PermTeamHead,
			PermTeamLead,
		)
	default:
		return PermissionSet{bits: make(map[Permission]bool)}
	}
}

// ResolveFromTeamMembership returns the PermissionSet for a team-scoped context.
// It reads the existing boolean fields that are already in the DB — no schema change.
func ResolveFromTeamMembership(isHead, isLead, canManageTasks, canViewAnalytics, canManageMembersLimited bool) PermissionSet {
	perms := make([]Permission, 0, 6)

	if isHead {
		perms = append(perms, PermTeamHead, PermManageTeam, PermManageTasks, PermManageMembersLimited)
	}
	if isLead {
		perms = append(perms, PermTeamLead, PermManageTeam)
	}
	if canManageTasks {
		perms = append(perms, PermManageTasks)
	}
	if canViewAnalytics {
		perms = append(perms, PermViewAnalytics)
	}
	if canManageMembersLimited {
		perms = append(perms, PermManageMembersLimited)
	}

	return grant(perms...)
}

// ResolveFromCustomRole returns the PermissionSet for a custom team role.
// Wraps the existing *models.CustomRole boolean flags.
func ResolveFromCustomRole(cr *models.CustomRole) PermissionSet {
	if cr == nil {
		return PermissionSet{bits: make(map[Permission]bool)}
	}
	return ResolveFromTeamMembership(
		false,
		false,
		cr.CanManageTasks,
		cr.CanViewAnalytics,
		cr.CanManageMembersLimited,
	)
}

// ResolveFromUserTeamMembership resolves permissions from a UserTeamMembership
// (the cached team list returned by the team repository).
func ResolveFromUserTeamMembership(m models.UserTeamMembership) PermissionSet {
	return ResolveFromTeamMembership(
		m.IsHead,
		m.IsLead,
		m.CanManageTasks,
		m.CanViewAnalytics,
		m.CanManageMembersLimited,
	)
}
