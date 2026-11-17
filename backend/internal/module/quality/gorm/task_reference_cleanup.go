package qualitygorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"gorm.io/gorm"
)

type TaskReferenceCleanupRepository struct{ db *gorm.DB }

func NewTaskReferenceCleanupRepository(db *gorm.DB) *TaskReferenceCleanupRepository {
	return &TaskReferenceCleanupRepository{db: db}
}

func (repository *TaskReferenceCleanupRepository) ClearTaskReferences(ctx context.Context, facts quality.TaskReferenceCleanupFacts) error {
	if repository == nil || repository.db == nil || facts.WorkspaceID == 0 || len(facts.TaskIDs) == 0 {
		return quality.ErrTaskReferenceCleanupInvalid
	}
	db := repository.db.WithContext(ctx)
	if len(facts.TestExecutionIDs) > 0 {
		if err := db.Model(&QualityBug{}).
			Where("workspace_id = ? AND source_execution_id IN ?", facts.WorkspaceID, facts.TestExecutionIDs).
			Update("source_execution_id", nil).Error; err != nil {
			return err
		}
	}
	if err := db.Model(&QualityBug{}).
		Where("workspace_id = ? AND source_test_id IN ?", facts.WorkspaceID, facts.TaskIDs).
		Update("source_test_id", nil).Error; err != nil {
		return err
	}
	var remaining int64
	query := db.Model(&QualityBug{}).Where("workspace_id = ?", facts.WorkspaceID)
	if len(facts.TestExecutionIDs) > 0 {
		query = query.Where("source_test_id IN ? OR source_execution_id IN ?", facts.TaskIDs, facts.TestExecutionIDs)
	} else {
		query = query.Where("source_test_id IN ?", facts.TaskIDs)
	}
	if err := query.Count(&remaining).Error; err != nil {
		return err
	}
	if remaining != 0 {
		return quality.ErrTaskReferenceCleanupConflict
	}
	return nil
}

var _ quality.TaskReferenceCleanupStore = (*TaskReferenceCleanupRepository)(nil)
