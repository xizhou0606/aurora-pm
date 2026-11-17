package api

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	ai "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	workflow "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

type projectDeletionImpactQueries struct {
	local        *project.ImpactService
	requirements *requirement.ProjectImpactService
	tasks        *task.ProjectImpactService
	taskGuard    *task.CascadeDeletionGuardService
	quality      *quality.ProjectImpactService
	workflows    *workflow.ProjectImpactService
	aiRuns       *ai.ProjectImpactService
}

func newProjectDeletionImpactQueries(db *gorm.DB) (*projectDeletionImpactQueries, error) {
	if db == nil {
		return nil, project.ErrImpactQueryInvalid
	}
	local, err := project.NewImpactService(projectgorm.NewImpactRepository(db))
	if err != nil {
		return nil, err
	}
	requirements, err := requirement.NewProjectImpactService(requirementgorm.NewProjectImpactRepository(db))
	if err != nil {
		return nil, err
	}
	tasks, err := task.NewProjectImpactService(taskgorm.NewProjectImpactRepository(db))
	if err != nil {
		return nil, err
	}
	taskGuard, err := task.NewCascadeDeletionGuardService(taskgorm.NewCascadeDeletionGuardRepository(db))
	if err != nil {
		return nil, err
	}
	qualityImpact, err := quality.NewProjectImpactService(qualitygorm.NewProjectImpactRepository(db))
	if err != nil {
		return nil, err
	}
	workflows, err := workflow.NewProjectImpactService(workflowgorm.NewProjectImpactRepository(db))
	if err != nil {
		return nil, err
	}
	aiRuns, err := ai.NewProjectImpactService(aigorm.NewProjectImpactRepository(db))
	if err != nil {
		return nil, err
	}
	return &projectDeletionImpactQueries{
		local: local, requirements: requirements, tasks: tasks, taskGuard: taskGuard, quality: qualityImpact,
		workflows: workflows, aiRuns: aiRuns,
	}, nil
}

func (queries *projectDeletionImpactQueries) InspectProjectDeletionImpact(
	ctx context.Context,
	workspaceID uint,
	projectSetID uint,
) (project.DeletionImpact, error) {
	if queries == nil {
		return project.DeletionImpact{}, project.ErrImpactQueryInvalid
	}
	if err := queries.taskGuard.ValidateCascadeDeletion(ctx, task.CascadeDeletionScope{WorkspaceID: workspaceID, ProjectSetID: projectSetID}); err != nil {
		if errors.Is(err, task.ErrCascadeDeletionInvalid) || errors.Is(err, task.ErrCascadeDeletionConflict) {
			return project.DeletionImpact{}, project.ErrMutationConflict
		}
		return project.DeletionImpact{}, err
	}
	local, err := queries.local.InspectLocalProjectImpact(ctx, workspaceID, projectSetID)
	if err != nil {
		return project.DeletionImpact{}, err
	}
	requirements, err := queries.requirements.InspectProjectImpact(ctx, workspaceID, projectSetID)
	if err != nil {
		return project.DeletionImpact{}, err
	}
	taskCount, err := queries.tasks.CountProjectReferences(ctx, workspaceID, projectSetID)
	if err != nil {
		return project.DeletionImpact{}, err
	}
	qualityCount, err := queries.quality.CountProjectReferences(ctx, workspaceID, projectSetID)
	if err != nil {
		return project.DeletionImpact{}, err
	}
	workflowCount, err := queries.workflows.CountProjectSubjectReferences(ctx, workspaceID, workflow.ProjectSubjects{
		PlanUnitIDs: local.PlanUnitIDs, RequirementIDs: requirements.RequirementIDs,
	})
	if err != nil {
		return project.DeletionImpact{}, err
	}
	aiRunCount, err := queries.aiRuns.CountProjectSubjectReferences(ctx, workspaceID, ai.ProjectSubjects{
		PlanUnitIDs: local.PlanUnitIDs, RequirementIDs: requirements.RequirementIDs,
	})
	if err != nil {
		return project.DeletionImpact{}, err
	}
	if requirements.Count != int64(len(requirements.RequirementIDs)) {
		return project.DeletionImpact{}, fmt.Errorf("requirement impact snapshot changed during inspection")
	}
	return project.DeletionImpact{
		Local: local, RequirementIDs: requirements.RequirementIDs, RequirementCount: requirements.Count,
		TaskCount: taskCount, QualityCount: qualityCount, WorkflowCount: workflowCount, AIRunCount: aiRunCount,
	}, nil
}

var _ project.DeletionImpactInspector = (*projectDeletionImpactQueries)(nil)
