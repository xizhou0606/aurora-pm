package api

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"gorm.io/gorm"
)

func membershipScope(tx *gorm.DB, a project.JoinActor, t project.JoinTarget, lock bool) (project.JoinScope, error) {
	ctx := tx.Statement.Context
	directory := identitygorm.NewAccessGuideDirectory(tx)
	if t.Kind == "workspace" {
		return directory.WorkspaceJoinScope(ctx, a, t.ID, lock)
	}
	repo := projectgorm.NewAccessGuideRepository(tx)
	var guide project.AccessGuide
	var err error
	guide, err = projectGuideService(tx).Get(ctx, t.Kind, t.ID)
	if err == nil && lock {
		err = repo.LockGuideUnit(ctx, guide.WorkspaceID, guide.PlanUnitID)
		if err == nil {
			guide, err = projectGuideService(tx).Get(ctx, t.Kind, t.ID)
		}
	}
	if err != nil {
		return project.JoinScope{}, err
	}
	check, err := newProjectAccessService(tx)
	if err != nil {
		return project.JoinScope{}, err
	}
	member, err := identitygorm.NewWorkspaceMembershipRepository(tx).HasActiveWorkspaceMembership(ctx, a.UserID, guide.WorkspaceID)
	if err != nil {
		return project.JoinScope{}, err
	}
	var allowed, manager bool
	kind := "plan_unit"
	allowed, err = check.CanAccessPlanUnit(ctx, guide.WorkspaceID, guide.PlanUnitID, a.FactID)
	if err != nil {
		return project.JoinScope{}, err
	}
	manager, err = check.CanManagePlanUnitMembers(ctx, guide.WorkspaceID, guide.PlanUnitID, a.FactID)
	if err != nil {
		return project.JoinScope{}, err
	}

	if manager {
		result, e := identity.NewAccessService(identitygorm.NewAccessRepository(tx), nil).Authorize(ctx, a.UserID, "PATCH", "/api/v1/platform/project-access-guide/:kind/:id/requests/:requestId")
		if e != nil {
			return project.JoinScope{}, e
		}
		manager = result == identity.AuthorizationAllowed
	}
	reviewers := []uint{}
	if guide.OwnerUserID > 0 {
		reviewers = append(reviewers, guide.OwnerUserID)
	}
	return project.JoinScope{TargetKind: kind, ReviewerIDs: reviewers, AccessGuide: guide, HasAccess: allowed, CanReview: manager, WorkspaceMember: member}, nil
}
