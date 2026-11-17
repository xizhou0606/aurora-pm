package workflow

import (
	"context"
	"errors"
)

var ErrProjectImpactInvalid = errors.New("workflow project impact query is invalid")

type ProjectSubjects struct {
	PlanUnitIDs    []uint
	RequirementIDs []uint
}

type ProjectImpactStore interface {
	CountProjectSubjectReferences(context.Context, uint, ProjectSubjects) (int64, error)
}

type ProjectImpactService struct{ store ProjectImpactStore }

func NewProjectImpactService(store ProjectImpactStore) (*ProjectImpactService, error) {
	if store == nil {
		return nil, ErrProjectImpactInvalid
	}
	return &ProjectImpactService{store: store}, nil
}

func (service *ProjectImpactService) CountProjectSubjectReferences(
	ctx context.Context,
	workspaceID uint,
	subjects ProjectSubjects,
) (int64, error) {
	if service == nil || service.store == nil || workspaceID == 0 {
		return 0, ErrProjectImpactInvalid
	}
	return service.store.CountProjectSubjectReferences(ctx, workspaceID, subjects)
}
