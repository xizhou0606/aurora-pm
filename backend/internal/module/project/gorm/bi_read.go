package projectgorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

type BIReadRepository struct{ db *gorm.DB }

func NewBIReadRepository(db *gorm.DB) *BIReadRepository { return &BIReadRepository{db: db} }

func (repository *BIReadRepository) LoadBIContext(ctx context.Context, workspaceID uint) (project.BIContext, error) {
	var sets []ProjectSet
	if err := repository.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("id").Find(&sets).Error; err != nil {
		return project.BIContext{}, err
	}
	var units []PlanUnit
	if err := repository.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("id").Find(&units).Error; err != nil {
		return project.BIContext{}, err
	}
	result := project.BIContext{ProjectSets: make([]project.ProjectSetSnapshot, 0, len(sets)), PlanUnits: make([]project.PlanUnitSnapshot, 0, len(units))}
	for _, row := range sets {
		result.ProjectSets = append(result.ProjectSets, projectSetListSnapshot(row))
	}
	for _, row := range units {
		result.PlanUnits = append(result.PlanUnits, planUnitListSnapshot(row))
	}
	return result, nil
}

var _ project.BIReadStore = (*BIReadRepository)(nil)
