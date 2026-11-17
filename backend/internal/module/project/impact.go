package project

import (
	"context"
	"errors"
)

var ErrImpactQueryInvalid = errors.New("project impact query is invalid")

type LocalProjectImpact struct {
	PlanUnitIDs        []uint
	DeliveryNodeCount  int64
	DeliveryLabelCount int64
	PermissionCount    int64
	NewsCount          int64
}

type DeletionImpact struct {
	Local            LocalProjectImpact
	RequirementIDs   []uint
	RequirementCount int64
	TaskCount        int64
	QualityCount     int64
	WorkflowCount    int64
	AIRunCount       int64
}

func (impact DeletionImpact) HasReferences() bool {
	return len(impact.Local.PlanUnitIDs) > 0 || impact.Local.DeliveryNodeCount > 0 ||
		impact.Local.DeliveryLabelCount > 0 || impact.Local.PermissionCount > 0 || impact.Local.NewsCount > 0 ||
		impact.RequirementCount > 0 || impact.TaskCount > 0 || impact.QualityCount > 0 ||
		impact.WorkflowCount > 0 || impact.AIRunCount > 0
}

type DeletionImpactInspector interface {
	InspectProjectDeletionImpact(context.Context, uint, uint) (DeletionImpact, error)
}

type LocalImpactStore interface {
	InspectLocalProjectImpact(context.Context, uint, uint) (LocalProjectImpact, error)
}

type ImpactService struct{ store LocalImpactStore }

func NewImpactService(store LocalImpactStore) (*ImpactService, error) {
	if store == nil {
		return nil, ErrImpactQueryInvalid
	}
	return &ImpactService{store: store}, nil
}

func (service *ImpactService) InspectLocalProjectImpact(
	ctx context.Context,
	workspaceID uint,
	projectSetID uint,
) (LocalProjectImpact, error) {
	if service == nil || service.store == nil || workspaceID == 0 || projectSetID == 0 {
		return LocalProjectImpact{}, ErrImpactQueryInvalid
	}
	impact, err := service.store.InspectLocalProjectImpact(ctx, workspaceID, projectSetID)
	if impact.PlanUnitIDs == nil && err == nil {
		impact.PlanUnitIDs = []uint{}
	}
	return impact, err
}
