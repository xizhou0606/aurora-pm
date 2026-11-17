package project

import "context"

// PlanUnitMembersSnapshot is the canonical owner + participant state of one
// plan unit (project/version item) returned by member reads and saves.
type PlanUnitMembersSnapshot struct {
	WorkspaceID        uint   `json:"workspace_id"`
	PlanUnitID         uint   `json:"plan_unit_id"`
	ProjectSetID       uint   `json:"project_set_id"`
	OwnerUserID        *uint  `json:"owner_user_id"`
	ParticipantUserIDs []uint `json:"participant_user_ids"`
	UpdatedAt          any    `json:"updated_at"`
	CanManageMembers   bool   `json:"can_manage_participants"`
}

// SetCreatorReader reads the project set creator fact
// (project_sets.created_by_user_id). The creator is the only project-set
// level membership fact: project sets have no owner or participants.
type SetCreatorReader interface {
	ProjectCreatorID(ctx context.Context, workspaceID, projectSetID uint) (uint, error)
}

// PlanUnitAccessReader resolves version-level membership facts. Every method
// is a workspace-scoped pure fact lookup; admin evaluation stays in the
// service so transport never sees role logic.
type PlanUnitAccessReader interface {
	// HasAnyPlanUnitAccess: the user owns or participates in any plan unit
	// under the project set.
	HasAnyPlanUnitAccess(ctx context.Context, workspaceID, projectSetID, userID uint) (bool, error)
	// HasPlanUnitAccess: the user is the set creator, the unit owner, or a
	// unit participant — the version-level data-access fact.
	HasPlanUnitAccess(ctx context.Context, workspaceID, planUnitID, userID uint) (bool, error)
	// CanManagePlanUnitMembers: the user is the set creator or the unit owner.
	CanManagePlanUnitMembers(ctx context.Context, workspaceID, planUnitID, userID uint) (bool, error)
	// IsProjectSetMemberFact: the user is the set creator or holds any unit
	// role under the set — the "who belongs to this project set" fact used by
	// assignee validation.
	IsProjectSetMemberFact(ctx context.Context, workspaceID, projectSetID, userID uint) (bool, error)
}

// PlanUnitAccessChecker is the version-level point gate consumed by
// plan-unit-scoped reads, writes and member management.
type PlanUnitAccessChecker interface {
	CanAccessPlanUnit(ctx context.Context, workspaceID, planUnitID, userID uint) (bool, error)
	CanManagePlanUnitMembers(ctx context.Context, workspaceID, planUnitID, userID uint) (bool, error)
}
