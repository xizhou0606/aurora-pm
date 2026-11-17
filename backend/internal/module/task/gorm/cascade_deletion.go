package taskgorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CascadeDeletionGuardRepository struct{ db *gorm.DB }

func NewCascadeDeletionGuardRepository(db *gorm.DB) *CascadeDeletionGuardRepository {
	return &CascadeDeletionGuardRepository{db: db}
}

func (repository *CascadeDeletionGuardRepository) ValidateCascadeDeletion(ctx context.Context, scope task.CascadeDeletionScope) error {
	if repository == nil || repository.db == nil || scope.WorkspaceID == 0 {
		return task.ErrCascadeDeletionInvalid
	}
	query := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("workspace_id = ?", scope.WorkspaceID)
	switch {
	case scope.ProjectSetID > 0:
		query = query.Where("project_set_id = ?", scope.ProjectSetID)
	case scope.RequirementID > 0:
		query = query.Where("requirement_id = ?", scope.RequirementID)
	case len(scope.TaskIDs) > 0:
		query = query.Where("id IN ?", scope.TaskIDs)
	default:
		return task.ErrCascadeDeletionInvalid
	}
	var rows []WorkTask
	if err := query.Order("id").Find(&rows).Error; err != nil {
		return err
	}
	if len(scope.TaskIDs) > 0 && len(rows) != len(scope.TaskIDs) {
		return task.ErrCascadeDeletionConflict
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		if row.MutationState != workTaskStateActive {
			return task.ErrCascadeDeletionConflict
		}
		ids = append(ids, row.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	var attachments int64
	if err := repository.db.WithContext(ctx).Model(&WorkTaskAttachment{}).
		Where("workspace_id = ? AND work_task_id IN ?", scope.WorkspaceID, ids).Count(&attachments).Error; err != nil {
		return err
	}
	if attachments > 0 {
		return task.ErrCascadeDeletionConflict
	}
	return nil
}

var _ task.CascadeDeletionGuardStore = (*CascadeDeletionGuardRepository)(nil)
