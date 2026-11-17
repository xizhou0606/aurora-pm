package taskgorm

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

type RequirementLifecycleRepository struct{ db *gorm.DB }

func NewRequirementLifecycleRepository(db *gorm.DB) *RequirementLifecycleRepository {
	return &RequirementLifecycleRepository{db: db}
}
func (repository *RequirementLifecycleRepository) MoveRequirementTasks(ctx context.Context, workspaceID, requirementID, teamID, projectSetID, planUnitID uint) (int64, error) {
	updated := repository.db.WithContext(ctx).Model(&WorkTask{}).Where("workspace_id = ? AND requirement_id = ?", workspaceID, requirementID).
		Updates(map[string]any{"team_id": teamID, "project_set_id": projectSetID, "project_id": planUnitID})
	return updated.RowsAffected, updated.Error
}
func (repository *RequirementLifecycleRepository) DeleteRequirementTasks(ctx context.Context, workspaceID, requirementID uint) (int64, error) {
	var ids []uint
	tx := repository.db.WithContext(ctx)
	if err := NewCascadeDeletionGuardRepository(tx).ValidateCascadeDeletion(ctx, task.CascadeDeletionScope{WorkspaceID: workspaceID, RequirementID: requirementID}); err != nil {
		return 0, err
	}
	if err := tx.Model(&WorkTask{}).Where("workspace_id = ? AND requirement_id = ?", workspaceID, requirementID).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	for _, row := range []any{&TaskTestExecutionRun{}, &TaskTestExecution{}, &TaskTestDefinition{}, &WorkTaskComment{}, &WorkTaskActivity{}, &WorkTaskWatcher{}, &WorkTaskAttachment{}} {
		if err := tx.Where("workspace_id = ? AND work_task_id IN ?", workspaceID, ids).Delete(row).Error; err != nil {
			return 0, err
		}
	}
	deleted := tx.Where("workspace_id = ? AND id IN ?", workspaceID, ids).Delete(&WorkTask{})
	return deleted.RowsAffected, deleted.Error
}

var _ task.RequirementLifecycleStore = (*RequirementLifecycleRepository)(nil)
