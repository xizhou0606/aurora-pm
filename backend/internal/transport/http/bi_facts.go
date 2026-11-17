package httpapi

import (
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
)

// biWorkspaceContext is an in-memory projection assembled exclusively from
// owning-domain read contracts.
type biWorkspaceContext struct {
	ProjectSets  map[uint]project.ProjectSetSnapshot
	Projects     map[uint]project.PlanUnitSnapshot
	Requirements map[uint]requirement.Snapshot
	Users        map[uint]identity.BIUser
}

func biUserDisplayName(ctx biWorkspaceContext, id *uint) string {
	if id == nil || *id == 0 {
		return ""
	}
	user, ok := ctx.Users[*id]
	if !ok {
		return ""
	}
	if user.DisplayName != "" {
		return user.DisplayName
	}
	return user.Username
}

func biUserAvatarURL(ctx biWorkspaceContext, id *uint) string {
	if id == nil || *id == 0 {
		return ""
	}
	return ctx.Users[*id].AvatarURL
}

func biQualityCurrentOwner(bug quality.BugSnapshot) *uint {
	if isBIQualityReview(bug.Status) && bug.ReviewerUserID != nil {
		return bug.ReviewerUserID
	}
	return bug.AssigneeUserID
}

func biRequirementOwnerID(row requirement.Snapshot) uint {
	if len(row.ProductOwnerUserIDs) == 0 {
		return 0
	}
	return row.ProductOwnerUserIDs[0]
}

func biRequirementPlanUnitID(row requirement.Snapshot) uint {
	if row.PlanUnitID == nil {
		return 0
	}
	return *row.PlanUnitID
}
