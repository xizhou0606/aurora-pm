package qualitygorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"gorm.io/gorm"
)

type ProjectDeletionRepository struct{ db *gorm.DB }

func NewProjectDeletionRepository(db *gorm.DB) *ProjectDeletionRepository {
	return &ProjectDeletionRepository{db: db}
}

func (repository *ProjectDeletionRepository) DeleteProjectData(ctx context.Context, workspaceID, projectSetID uint) error {
	if repository == nil || repository.db == nil || workspaceID == 0 || projectSetID == 0 {
		return quality.ErrProjectDeletionInvalid
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var bugIDs []uint
		if err := tx.Unscoped().Model(&QualityBug{}).Where("workspace_id = ? AND project_set_id = ?", workspaceID, projectSetID).
			Order("id").Pluck("id", &bugIDs).Error; err != nil {
			return err
		}
		if len(bugIDs) > 0 {
			for _, row := range []any{&QualityBugComment{}, &QualityBugActivity{}} {
				if err := tx.Where("workspace_id = ? AND quality_bug_id IN ?", workspaceID, bugIDs).Delete(row).Error; err != nil {
					return err
				}
			}
		}
		return tx.Unscoped().Where("workspace_id = ? AND project_set_id = ?", workspaceID, projectSetID).Delete(&QualityBug{}).Error
	})
}

var _ quality.ProjectDeletionStore = (*ProjectDeletionRepository)(nil)
