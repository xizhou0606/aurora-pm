package taskgorm

import (
	"context"
	"errors"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

type WorkflowScopeRepository struct{ db *gorm.DB }

func NewWorkflowScopeRepository(db *gorm.DB) *WorkflowScopeRepository {
	return &WorkflowScopeRepository{db: db}
}

func (repository *WorkflowScopeRepository) ValidateWorkflowScope(ctx context.Context, workspaceID uint, scope task.WorkflowScope) error {
	if repository == nil || repository.db == nil || workspaceID == 0 || (scope.ProjectID > 0 && scope.TeamID == 0) {
		return task.ErrWorkflowReferenceInvalid
	}
	if scope.TeamID == 0 {
		return nil
	}
	var team identitygorm.Team
	err := repository.db.WithContext(ctx).Where("id = ? AND workspace_id = ?", scope.TeamID, workspaceID).First(&team).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.ErrWorkflowReferenceInvalid
	}
	if err != nil {
		return err
	}
	if scope.ProjectID == 0 {
		return nil
	}
	var plan projectgorm.PlanUnit
	err = repository.db.WithContext(ctx).Where(
		"id = ? AND workspace_id = ? AND team_id = ?", scope.ProjectID, workspaceID, scope.TeamID,
	).First(&plan).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.ErrWorkflowReferenceInvalid
	}
	if err != nil {
		return err
	}
	var projectCount int64
	if err := repository.db.WithContext(ctx).Model(&projectgorm.ProjectSet{}).Where(
		"id = ? AND workspace_id = ? AND team_id = ?", plan.ProjectSetID, workspaceID, scope.TeamID,
	).Count(&projectCount).Error; err != nil {
		return err
	}
	if projectCount != 1 {
		return task.ErrWorkflowReferenceInvalid
	}
	return nil
}

var _ task.WorkflowScopeStore = (*WorkflowScopeRepository)(nil)
