package quality

import (
	"context"
	"errors"
)

var ErrProjectDeletionInvalid = errors.New("quality project deletion is invalid")

type ProjectDeletionStore interface {
	DeleteProjectData(context.Context, uint, uint) error
}

type ProjectDeletionService struct{ store ProjectDeletionStore }

func NewProjectDeletionService(store ProjectDeletionStore) (*ProjectDeletionService, error) {
	if store == nil {
		return nil, ErrProjectDeletionInvalid
	}
	return &ProjectDeletionService{store: store}, nil
}

func (service *ProjectDeletionService) DeleteProjectData(ctx context.Context, workspaceID, projectSetID uint) error {
	if service == nil || service.store == nil || workspaceID == 0 || projectSetID == 0 {
		return ErrProjectDeletionInvalid
	}
	return service.store.DeleteProjectData(ctx, workspaceID, projectSetID)
}
