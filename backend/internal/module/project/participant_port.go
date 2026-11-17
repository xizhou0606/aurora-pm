package project

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"time"
)

// PlanUnitMemberFactsRepository persists the plan-unit (project/version item)
// owner + participant membership. Every query is workspace-scoped; the adapter
// never reads the retired project-level participant tables.
type PlanUnitMemberFactsRepository interface {
	LoadPlanUnitMemberState(ctx context.Context, workspaceID, planUnitID uint) (MemberState, error)
	SavePlanUnitMemberState(ctx context.Context, workspaceID, planUnitID uint, state MemberState, meta MemberMutationMeta) (MemberState, error)
	ListPlanUnitMemberUsers(ctx context.Context, workspaceID, planUnitID uint, query string, page, pageSize int) (MemberPage, error)
	// SearchWorkspaceActiveUsers enumerates every active user of the workspace
	// (not only current unit members) so member-option pickers can offer users
	// who have not joined the plan unit yet. IDs are canonical fact IDs.
	SearchWorkspaceActiveUsers(ctx context.Context, workspaceID uint, query string, page, pageSize int) (MemberPage, error)
}

// MemberState is the authoritative owner + participant state for one subject
// (a plan unit). OwnerUserID stores the platform fact user ID.
type MemberState struct {
	OwnerUserID        *uint
	ParticipantUserIDs []uint
	UpdatedAt          time.Time
}

// MemberMutationMeta records the actor and optimistic-lock timestamp for one
// atomic owner + participant replacement.
type MemberMutationMeta struct {
	ActorUserID    uint
	ExpectedAt     *time.Time
	Reason         string
	TaskResolution *task.MemberTaskResolution
}

// PlanUnitResponsibilityInspector aggregates outstanding responsibilities that
// block a plan unit participant removal. Implementations must scope every
// query by workspace and plan unit.
type PlanUnitResponsibilityInspector interface {
	InspectPlanUnitRemovalImpact(ctx context.Context, workspaceID, planUnitID, userID uint) (RemovalImpact, error)
}
