package requirementgorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"gorm.io/gorm"
)

type ProjectImpactRepository struct{ db *gorm.DB }

func NewProjectImpactRepository(db *gorm.DB) *ProjectImpactRepository {
	return &ProjectImpactRepository{db: db}
}

func (repository *ProjectImpactRepository) InspectProjectImpact(
	ctx context.Context,
	workspaceID uint,
	projectSetID uint,
) (requirement.ProjectImpact, error) {
	if repository == nil || repository.db == nil {
		return requirement.ProjectImpact{}, requirement.ErrProjectImpactInvalid
	}
	query := repository.db.WithContext(ctx).Model(&Requirement{}).
		Where("workspace_id = ? AND project_new_id = ?", workspaceID, projectSetID)
	impact := requirement.ProjectImpact{RequirementIDs: []uint{}}
	if err := query.Count(&impact.Count).Error; err != nil {
		return requirement.ProjectImpact{}, err
	}
	if err := query.Order("id ASC").Pluck("id", &impact.RequirementIDs).Error; err != nil {
		return requirement.ProjectImpact{}, err
	}
	return impact, nil
}

var _ requirement.ProjectImpactStore = (*ProjectImpactRepository)(nil)

func (repository *ProjectImpactRepository) CountPlanUnitReferences(ctx context.Context, workspaceID, planUnitID uint) (int64, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || planUnitID == 0 {
		return 0, requirement.ErrProjectImpactInvalid
	}
	var count int64
	err := repository.db.WithContext(ctx).Model(&Requirement{}).Where("workspace_id = ? AND version_task_new_id = ?", workspaceID, planUnitID).Count(&count).Error
	return count, err
}

var _ requirement.PlanUnitImpactStore = (*ProjectImpactRepository)(nil)

func (repository *ProjectImpactRepository) ListPlanUnitRequirementIDs(ctx context.Context, workspaceID, planUnitID uint) ([]uint, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || planUnitID == 0 {
		return nil, requirement.ErrProjectImpactInvalid
	}
	ids := []uint{}
	err := repository.db.WithContext(ctx).Model(&Requirement{}).
		Where("workspace_id = ? AND version_task_new_id = ?", workspaceID, planUnitID).Order("id").Pluck("id", &ids).Error
	return ids, err
}
