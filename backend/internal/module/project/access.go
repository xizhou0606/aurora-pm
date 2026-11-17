// Package project owns project-set facts and the membership access model.
// Project sets carry a single membership fact — their creator. All other
// membership lives at the plan-unit (project/version item) level: a user sees
// a project set when they created it or hold any role on one of its plan
// units, and version-level data is gated by the unit's owner/participants.
// The retired project-level owner/participant and role/group/user data-scope
// implementations are not read by any online path.
package project

import (
	"context"
	"errors"
	"fmt"
)

// Participant errors are the canonical contract errors surfaced by the HTTP
// adapter. They are intentionally distinct from the old data-scope errors.
var (
	ErrParticipantInvalid          = errors.New("project participant is invalid")
	ErrParticipantCrossWorkspace   = errors.New("project member spans another workspace")
	ErrParticipantProjectNotFound  = errors.New("project set not found")
	ErrParticipantPlanUnitNotFound = errors.New("plan unit not found")
	ErrParticipantStale            = errors.New("project members changed concurrently")
	ErrParticipantInUse            = errors.New("project participant still has outstanding responsibilities")
	ErrParticipantPermissionDenied = errors.New("project member modification is not permitted")
	ErrParticipantNotFound         = errors.New("project participant not found")
	ErrParticipantAlready          = errors.New("user is already a project participant")
	ErrParticipantNotOwner         = errors.New("user is not a project owner or admin for this operation")
	ErrProjectAccessNotFound       = errors.New("project not accessible") // maps to HTTP 404
	ErrParticipantIsOwner          = errors.New("owner cannot be stored as a participant")
)

// UserRef is the minimal stable user projection used by member lists. The ID
// is the canonical internal users.id. It never
// carries credentials or legacy role fields.
type UserRef struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// MemberPage is the paged member projection shared by project sets and plan
// units.
type MemberPage struct {
	Items    []UserRef `json:"items"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
	Total    int64     `json:"total"`
	HasMore  bool      `json:"has_more"`
}

// AccessChecker is the stable project-set data gate consumed by every
// project-scoped read, write, tool, workflow, workbench and BI path.
type AccessChecker interface {
	CanAccessProject(ctx context.Context, workspaceID, projectSetID, userID uint) (bool, error)
	RequireProjectAccess(ctx context.Context, workspaceID, projectSetID, userID uint) error
	// IsAccessAdministrator reports whether the user is an active workspace
	// member holding the access-administrator role. It is the narrow capability
	// used to allow non-owner member management without exposing role names to
	// the transport layer.
	IsAccessAdministrator(ctx context.Context, workspaceID, userID uint) (bool, error)
}

// AccessFilter is a reusable SQL subquery filter that restricts a
// project-referencing query to the user's accessible projects. Empty SQL
// means "no additional restriction" (the Admin fast path).
type AccessFilter struct {
	SQL  string
	Args []any
}

// AccessFilterBuilder builds the DB-side EXISTS filter. The GORM adapter owns
// the SQL; the domain service resolves the Admin fast path. This is the sole
// interface for producing SQL-level project membership filters (unified from
// the retired ProjectMemberFilterBuilder).
type AccessFilterBuilder interface {
	BuildMemberFilter(ctx context.Context, table, projectColumn string, workspaceID, userID uint) (AccessFilter, error)
}

// RemovalImpact aggregates outstanding responsibilities that block a
// participant removal. Implementations must scope every query by workspace
// and subject and treat completion states per the contract.
type RemovalImpact struct {
	OpenTaskCount       int64        `json:"open_task_count"`
	OpenBugCount        int64        `json:"open_bug_count"`
	ActiveWorkflowCount int64        `json:"active_workflow_count"`
	PlanUnitOwnerCount  int64        `json:"plan_unit_owner_count"`
	ResponsibleItems    []ImpactItem `json:"responsible_items"`
}

// HasOutstanding returns true when there are any outstanding responsibilities
// blocking participant removal.
func (r RemovalImpact) HasOutstanding() bool {
	return len(r.ResponsibleItems) > 0 || r.OpenTaskCount > 0 || r.OpenBugCount > 0 || r.ActiveWorkflowCount > 0 || r.PlanUnitOwnerCount > 0
}

// Summary returns a human-readable summary of outstanding responsibilities.
func (r RemovalImpact) Summary() string {
	parts := make([]string, 0, 4)
	if r.OpenTaskCount > 0 {
		parts = append(parts, fmt.Sprintf("%d open tasks", r.OpenTaskCount))
	}
	if r.OpenBugCount > 0 {
		parts = append(parts, fmt.Sprintf("%d open bugs", r.OpenBugCount))
	}
	if r.ActiveWorkflowCount > 0 {
		parts = append(parts, fmt.Sprintf("%d active workflows", r.ActiveWorkflowCount))
	}
	if r.PlanUnitOwnerCount > 0 {
		parts = append(parts, fmt.Sprintf("%d plan units owned", r.PlanUnitOwnerCount))
	}
	return fmt.Sprintf("%d item(s): %s", len(r.ResponsibleItems), joinParts(parts))
}

func joinParts(parts []string) string {
	if len(parts) == 0 {
		return "none"
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += "; " + parts[i]
	}
	return result
}

// ImpactItem is a safe, displayable responsibility reference. It never leaks
// another workspace's or project's data.
type ImpactItem struct {
	ResourceType string `json:"resource_type"`
	ResourceID   uint   `json:"resource_id"`
	Title        string `json:"title"`
	Status       string `json:"status"`
	Relation     string `json:"relation"`
}
