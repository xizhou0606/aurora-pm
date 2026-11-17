package projectgorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

type ImpactRepository struct{ db *gorm.DB }

func NewImpactRepository(db *gorm.DB) *ImpactRepository { return &ImpactRepository{db: db} }

func (repository *ImpactRepository) InspectLocalProjectImpact(
	ctx context.Context,
	workspaceID uint,
	projectSetID uint,
) (project.LocalProjectImpact, error) {
	if repository == nil || repository.db == nil {
		return project.LocalProjectImpact{}, project.ErrImpactQueryInvalid
	}
	db := repository.db.WithContext(ctx)
	var exists int64
	if err := db.Model(&ProjectSet{}).Where("workspace_id = ? AND id = ?", workspaceID, projectSetID).Count(&exists).Error; err != nil {
		return project.LocalProjectImpact{}, err
	}
	if exists != 1 {
		return project.LocalProjectImpact{}, project.ErrMutationNotFound
	}
	impact := project.LocalProjectImpact{PlanUnitIDs: []uint{}}
	if err := db.Model(&PlanUnit{}).Where("workspace_id = ? AND project_new_id = ?", workspaceID, projectSetID).
		Order("id ASC").Pluck("id", &impact.PlanUnitIDs).Error; err != nil {
		return project.LocalProjectImpact{}, err
	}
	if len(impact.PlanUnitIDs) > 0 {
		if err := db.Model(&DeliveryNode{}).Where("workspace_id = ? AND plan_unit_id IN ?", workspaceID, impact.PlanUnitIDs).
			Count(&impact.DeliveryNodeCount).Error; err != nil {
			return project.LocalProjectImpact{}, err
		}
	}
	counts := []struct {
		model any
		where string
		args  []any
		value *int64
	}{
		{&DeliveryNodeLabel{}, "workspace_id = ? AND project_set_id = ?", []any{workspaceID, projectSetID}, &impact.DeliveryLabelCount},
		{&ProjectPermission{}, "workspace_id = ? AND project_new_id = ?", []any{workspaceID, projectSetID}, &impact.PermissionCount},
		{&ProjectNews{}, "workspace_id = ? AND project_set_id = ?", []any{workspaceID, projectSetID}, &impact.NewsCount},
	}
	for _, count := range counts {
		if err := db.Model(count.model).Where(count.where, count.args...).Count(count.value).Error; err != nil {
			return project.LocalProjectImpact{}, err
		}
	}
	return impact, nil
}

var _ project.LocalImpactStore = (*ImpactRepository)(nil)
