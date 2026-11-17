package identitygorm

import (
	"context"
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const workspaceReviewRoute = "/api/v1/platform/workspace-access-requests/:id/:requestId"

// Workspace management retains its existing action and visibility policy.
func (r *AccessGuideDirectory) CanReviewWorkspace(ctx context.Context, principal, workspace uint) (bool, error) {
	result, err := identity.NewAccessService(NewAccessRepository(r.db), nil).Authorize(ctx, principal, "PATCH", workspaceReviewRoute)
	if err != nil || result != identity.AuthorizationAllowed {
		return false, err
	}
	_, err = NewAccessRepository(r.db).FindVisibleManagedWorkspace(ctx, principal, workspace)
	if errors.Is(err, identity.ErrWorkspaceAdminNotFound) {
		return false, nil
	}
	return err == nil, err
}
func (r *AccessGuideDirectory) WorkspaceJoinScope(ctx context.Context, actor project.JoinActor, workspace uint, lock bool) (project.JoinScope, error) {
	var row Workspace
	q := r.db.WithContext(ctx)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := q.Where("id = ? AND enabled = ?", workspace, true).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return project.JoinScope{}, project.ErrGuideUnavailable
		}
		return project.JoinScope{}, err
	}
	member, err := NewWorkspaceMembershipRepository(r.db).HasActiveWorkspaceMembership(ctx, actor.UserID, workspace)
	if err != nil {
		return project.JoinScope{}, err
	}
	review, err := r.CanReviewWorkspace(ctx, actor.UserID, workspace)
	if err != nil {
		return project.JoinScope{}, err
	}
	// Candidates are active members or access administrators; each is checked by the same policy.
	var candidates []User
	err = r.db.WithContext(ctx).Where("active = ? AND status = ?", true, identity.AccountActive).
		Where("id IN (SELECT user_id FROM workspace_members WHERE workspace_id = ? AND enabled = ?) OR id IN (SELECT aur.user_id FROM access_user_roles aur JOIN access_roles ar ON ar.id = aur.role_id WHERE ar.code = ? AND ar.enabled = ?)", workspace, true, identity.AccessAdministratorRoleCode, true).Find(&candidates).Error
	if err != nil {
		return project.JoinScope{}, err
	}
	reviewers := []uint{}
	for _, user := range candidates {
		allowed, err := r.CanReviewWorkspace(ctx, user.ID, workspace)
		if err != nil {
			return project.JoinScope{}, err
		}
		if allowed {
			id := user.ID
			reviewers = append(reviewers, id)
		}
	}
	return project.JoinScope{TargetKind: "workspace", AccessGuide: project.AccessGuide{WorkspaceID: workspace, WorkspaceName: row.Name}, HasAccess: member, WorkspaceMember: member, CanReview: review, ReviewerIDs: reviewers}, nil
}
func (r *AccessGuideDirectory) AddApprovedWorkspaceMember(ctx context.Context, workspace, applicantFact, actorFact uint) error {
	var applicant, actor User
	for fact, dest := range map[uint]*User{applicantFact: &applicant, actorFact: &actor} {
		if err := r.db.WithContext(ctx).Where("active = ? AND status = ? AND id = ?", true, identity.AccountActive, fact).First(dest).Error; err != nil {
			return project.ErrJoinForbidden
		}
	}
	allowed, err := r.CanReviewWorkspace(ctx, actor.ID, workspace)
	if err != nil {
		return err
	}
	if !allowed {
		return project.ErrJoinForbidden
	}
	var existing WorkspaceMember
	err = r.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspace, applicant.ID).First(&existing).Error
	if err == nil {
		if existing.Enabled {
			return nil
		}
		return project.ErrJoinForbidden
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	_, err = identity.NewWorkspaceAdminService(NewAccessRepository(r.db)).AddMember(ctx, actor.ID, workspace, identity.WorkspaceMemberInput{UserID: applicant.ID, Role: "member", Enabled: true})
	return err
}
