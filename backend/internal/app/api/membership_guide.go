package api

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	resourceaccessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
	"gorm.io/gorm"
)

type membershipGuideReader struct {
	db       *gorm.DB
	requests *resourceaccessgorm.ProjectRequestRepository
}

func (r *membershipGuideReader) GuideMemberships(ctx context.Context, a project.JoinActor, g project.AccessGuide) ([]project.GuideMembership, error) {
	pending, err := r.requests.PendingMembershipKinds(ctx, a, g)
	if err != nil {
		return nil, err
	}
	out := []project.GuideMembership{}
	for _, target := range []project.JoinTarget{{Kind: "workspace", ID: g.WorkspaceID}, {Kind: "plan_unit", ID: g.PlanUnitID}} {
		scope, err := membershipScope(r.db.WithContext(ctx), a, target, false)
		if err != nil {
			return nil, err
		}
		status := "requestable"
		switch {
		case scope.HasAccess:
			status = "joined"
		case pending[target.Kind]:
			status = "pending"
		case target.Kind != "workspace" && !scope.WorkspaceMember:
			status = "workspace_required"
		case len(scope.ReviewerIDs) == 0:
			status = "unavailable"
		}
		out = append(out, project.GuideMembership{Kind: target.Kind, ID: target.ID, Status: status})
	}
	return out, nil
}
