package api

import (
	"context"
	"errors"
	"fmt"

	ai "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	workflow "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

type projectDeletionSteps struct {
	workflow    *workflow.ProjectDeletionService
	ai          *ai.ProjectDeletionService
	quality     *quality.ProjectDeletionService
	task        *task.ProjectDeletionService
	requirement *requirement.ProjectDeletionService
}

func newProjectDeletionSteps(db *gorm.DB) (*projectDeletionSteps, error) {
	workflowService, err := workflow.NewProjectDeletionService(workflowgorm.NewProjectDeletionRepository(db))
	if err != nil {
		return nil, err
	}
	aiService, err := ai.NewProjectDeletionService(aigorm.NewProjectDeletionRepository(db))
	if err != nil {
		return nil, err
	}
	qualityService, err := quality.NewProjectDeletionService(qualitygorm.NewProjectDeletionRepository(db))
	if err != nil {
		return nil, err
	}
	taskService, err := task.NewProjectDeletionService(taskgorm.NewProjectDeletionRepository(db))
	if err != nil {
		return nil, err
	}
	requirementService, err := requirement.NewProjectDeletionService(requirementgorm.NewProjectDeletionRepository(db))
	if err != nil {
		return nil, err
	}
	return &projectDeletionSteps{workflow: workflowService, ai: aiService, quality: qualityService, task: taskService, requirement: requirementService}, nil
}

func (steps *projectDeletionSteps) RunProjectDeletionStep(ctx context.Context, step project.DeletionStep, saga project.DeletionSaga) error {
	subjectsWorkflow := workflow.ProjectSubjects{PlanUnitIDs: saga.PlanUnitIDs, RequirementIDs: saga.RequirementIDs}
	subjectsAI := ai.ProjectSubjects{PlanUnitIDs: saga.PlanUnitIDs, RequirementIDs: saga.RequirementIDs}
	var err error
	switch step {
	case project.DeletionWorkflow:
		err = steps.workflow.DeleteProjectSubjects(ctx, saga.WorkspaceID, subjectsWorkflow)
	case project.DeletionAI:
		err = steps.ai.DeleteProjectSubjects(ctx, saga.WorkspaceID, subjectsAI)
	case project.DeletionQuality:
		err = steps.quality.DeleteProjectData(ctx, saga.WorkspaceID, saga.ProjectSetID)
	case project.DeletionTask:
		err = steps.task.DeleteProjectData(ctx, saga.WorkspaceID, saga.ProjectSetID)
	case project.DeletionRequirement:
		err = steps.requirement.DeleteProjectRequirements(ctx, saga.WorkspaceID, saga.ProjectSetID, saga.RequirementIDs)
	default:
		return fmt.Errorf("unsupported project deletion step %d", step)
	}
	if errors.Is(err, task.ErrCascadeDeletionInvalid) || errors.Is(err, task.ErrCascadeDeletionConflict) {
		return project.ErrMutationConflict
	}
	return err
}

var _ project.DeletionStepRunner = (*projectDeletionSteps)(nil)
