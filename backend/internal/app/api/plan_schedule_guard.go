package api

import (
	"context"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
)

type planScheduleGuard struct {
	requirements *requirement.PlanUnitImpactService
	workflows    *workflow.SubjectScheduleService
}

func (guard planScheduleGuard) ValidatePlanUnitSchedule(ctx context.Context, workspaceID, planUnitID uint, start, end *time.Time) error {
	if guard.requirements == nil || guard.workflows == nil {
		return project.ErrMutationConflict
	}
	ids, err := guard.requirements.ListPlanUnitRequirementIDs(ctx, workspaceID, planUnitID)
	if err != nil {
		return err
	}
	return guard.workflows.ValidateSubjectSchedules(ctx, workspaceID, planUnitID, ids, start, end)
}

var _ project.PlanScheduleGuard = planScheduleGuard{}
