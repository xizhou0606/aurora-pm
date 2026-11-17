package quality

import (
	"context"
	"errors"
)

var ErrProjectImpactInvalid = errors.New("quality project impact query is invalid")

type ProjectImpactStore interface {
	CountProjectReferences(context.Context, uint, uint) (int64, error)
}

type ProjectImpactService struct{ store ProjectImpactStore }

func NewProjectImpactService(store ProjectImpactStore) (*ProjectImpactService, error) {
	if store == nil {
		return nil, ErrProjectImpactInvalid
	}
	return &ProjectImpactService{store: store}, nil
}

func (service *ProjectImpactService) CountProjectReferences(ctx context.Context, workspaceID, projectSetID uint) (int64, error) {
	if service == nil || service.store == nil || workspaceID == 0 || projectSetID == 0 {
		return 0, ErrProjectImpactInvalid
	}
	return service.store.CountProjectReferences(ctx, workspaceID, projectSetID)
}
