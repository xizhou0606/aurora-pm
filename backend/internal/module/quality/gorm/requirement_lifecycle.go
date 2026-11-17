package qualitygorm

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"gorm.io/gorm"
)

type RequirementLifecycleRepository struct{ db *gorm.DB }

func NewRequirementLifecycleRepository(db *gorm.DB) *RequirementLifecycleRepository {
	return &RequirementLifecycleRepository{db: db}
}
func (repository *RequirementLifecycleRepository) MoveRequirementBugs(ctx context.Context, workspaceID, requirementID, projectSetID, planUnitID uint) (int64, error) {
	updated := repository.db.WithContext(ctx).Model(&QualityBug{}).Where("workspace_id = ? AND requirement_id = ?", workspaceID, requirementID).Updates(map[string]any{"project_set_id": projectSetID, "plan_unit_id": planUnitID})
	return updated.RowsAffected, updated.Error
}
func (repository *RequirementLifecycleRepository) DeleteRequirementBugs(ctx context.Context, workspaceID, requirementID uint) (int64, error) {
	tx := repository.db.WithContext(ctx)
	var ids []uint
	if err := tx.Unscoped().Model(&QualityBug{}).Where("workspace_id = ? AND requirement_id = ?", workspaceID, requirementID).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	for _, row := range []any{&QualityBugComment{}, &QualityBugActivity{}} {
		if len(ids) > 0 {
			if err := tx.Where("workspace_id = ? AND quality_bug_id IN ?", workspaceID, ids).Delete(row).Error; err != nil {
				return 0, err
			}
		}
	}
	deleted := tx.Unscoped().Where("workspace_id = ? AND requirement_id = ?", workspaceID, requirementID).Delete(&QualityBug{})
	return deleted.RowsAffected, deleted.Error
}

var _ quality.RequirementLifecycleStore = (*RequirementLifecycleRepository)(nil)
