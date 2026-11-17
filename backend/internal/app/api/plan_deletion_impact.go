package api

import (
	"context"

	ai "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	workflow "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
)

type planDeletionImpact struct {
	requirements *requirement.PlanUnitImpactService
	workflow     *workflow.ProjectDeletionService
	ai           *ai.ProjectImpactService
}

func (impact *planDeletionImpact) InspectPlanUnitDeletion(ctx context.Context, workspaceID, planUnitID uint) (project.PlanDeletionImpact, error) {
	requirements, err := impact.requirements.CountPlanUnitReferences(ctx, workspaceID, planUnitID)
	if err != nil {
		return project.PlanDeletionImpact{}, err
	}
	history, err := impact.workflow.PlanUnitHasExecutionHistory(ctx, workspaceID, planUnitID)
	if err != nil {
		return project.PlanDeletionImpact{}, err
	}
	runs, err := impact.ai.CountProjectSubjectReferences(ctx, workspaceID, ai.ProjectSubjects{PlanUnitIDs: []uint{planUnitID}})
	if err != nil {
		return project.PlanDeletionImpact{}, err
	}
	return project.PlanDeletionImpact{RequirementCount: requirements, WorkflowHistory: history, AIRunCount: runs}, nil
}

var _ project.PlanDeletionInspector = (*planDeletionImpact)(nil)
