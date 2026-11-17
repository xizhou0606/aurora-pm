package taskgorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

type ProjectDeletionRepository struct{ db *gorm.DB }

func NewProjectDeletionRepository(db *gorm.DB) *ProjectDeletionRepository {
	return &ProjectDeletionRepository{db: db}
}

func (repository *ProjectDeletionRepository) DeleteProjectData(ctx context.Context, workspaceID, projectSetID uint) error {
	if repository == nil || repository.db == nil || workspaceID == 0 || projectSetID == 0 {
		return task.ErrProjectDeletionInvalid
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := NewCascadeDeletionGuardRepository(tx).ValidateCascadeDeletion(ctx, task.CascadeDeletionScope{WorkspaceID: workspaceID, ProjectSetID: projectSetID}); err != nil {
			return err
		}
		var taskIDs []uint
		if err := tx.Model(&WorkTask{}).Where("workspace_id = ? AND project_set_id = ?", workspaceID, projectSetID).
			Order("id").Pluck("id", &taskIDs).Error; err != nil {
			return err
		}
		if len(taskIDs) > 0 {
			for _, row := range []any{
				&TaskTestExecutionRun{}, &TaskTestExecution{}, &TaskTestDefinition{},
				&WorkTaskComment{}, &WorkTaskActivity{}, &WorkTaskWatcher{}, &WorkTaskAttachment{},
			} {
				if err := tx.Where("workspace_id = ? AND work_task_id IN ?", workspaceID, taskIDs).Delete(row).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("workspace_id = ? AND id IN ?", workspaceID, taskIDs).Delete(&WorkTask{}).Error; err != nil {
				return err
			}
		}
		return tx.Where("workspace_id = ? AND project_set_id = ?", workspaceID, projectSetID).Delete(&KeySequence{}).Error
	})
}

var _ task.ProjectDeletionStore = (*ProjectDeletionRepository)(nil)
