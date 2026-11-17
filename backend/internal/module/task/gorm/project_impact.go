package taskgorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

type ProjectImpactRepository struct{ db *gorm.DB }

func NewProjectImpactRepository(db *gorm.DB) *ProjectImpactRepository {
	return &ProjectImpactRepository{db: db}
}

func (repository *ProjectImpactRepository) CountProjectReferences(ctx context.Context, workspaceID, projectSetID uint) (int64, error) {
	if repository == nil || repository.db == nil {
		return 0, task.ErrProjectImpactInvalid
	}
	var count int64
	err := repository.db.WithContext(ctx).Model(&WorkTask{}).
		Where("workspace_id = ? AND project_set_id = ?", workspaceID, projectSetID).Count(&count).Error
	return count, err
}

var _ task.ProjectImpactStore = (*ProjectImpactRepository)(nil)
