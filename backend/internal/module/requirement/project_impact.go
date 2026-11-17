package requirement

import (
	"context"
	"errors"
)

var ErrProjectImpactInvalid = errors.New("requirement project impact query is invalid")

type ProjectImpact struct {
	RequirementIDs []uint
	Count          int64
}

type ProjectImpactStore interface {
	InspectProjectImpact(context.Context, uint, uint) (ProjectImpact, error)
}

type PlanUnitImpactStore interface {
	CountPlanUnitReferences(context.Context, uint, uint) (int64, error)
	ListPlanUnitRequirementIDs(context.Context, uint, uint) ([]uint, error)
}

func (service *PlanUnitImpactService) ListPlanUnitRequirementIDs(ctx context.Context, workspaceID, planUnitID uint) ([]uint, error) {
	if service == nil || service.store == nil || workspaceID == 0 || planUnitID == 0 {
		return nil, ErrProjectImpactInvalid
	}
	return service.store.ListPlanUnitRequirementIDs(ctx, workspaceID, planUnitID)
}

type PlanUnitImpactService struct{ store PlanUnitImpactStore }

func NewPlanUnitImpactService(store PlanUnitImpactStore) (*PlanUnitImpactService, error) {
	if store == nil {
		return nil, ErrProjectImpactInvalid
	}
	return &PlanUnitImpactService{store: store}, nil
}
func (service *PlanUnitImpactService) CountPlanUnitReferences(ctx context.Context, workspaceID, planUnitID uint) (int64, error) {
	if service == nil || service.store == nil || workspaceID == 0 || planUnitID == 0 {
		return 0, ErrProjectImpactInvalid
	}
	return service.store.CountPlanUnitReferences(ctx, workspaceID, planUnitID)
}

type ProjectImpactService struct{ store ProjectImpactStore }

func NewProjectImpactService(store ProjectImpactStore) (*ProjectImpactService, error) {
	if store == nil {
		return nil, ErrProjectImpactInvalid
	}
	return &ProjectImpactService{store: store}, nil
}

func (service *ProjectImpactService) InspectProjectImpact(ctx context.Context, workspaceID, projectSetID uint) (ProjectImpact, error) {
	if service == nil || service.store == nil || workspaceID == 0 || projectSetID == 0 {
		return ProjectImpact{}, ErrProjectImpactInvalid
	}
	impact, err := service.store.InspectProjectImpact(ctx, workspaceID, projectSetID)
	if impact.RequirementIDs == nil && err == nil {
		impact.RequirementIDs = []uint{}
	}
	return impact, err
}
