package requirement

import (
	"context"
	"errors"
)

var ErrProjectDeletionInvalid = errors.New("requirement project deletion is invalid")

type ProjectDeletionStore interface {
	DeleteProjectRequirements(context.Context, uint, uint, []uint) error
}
type ProjectDeletionService struct{ store ProjectDeletionStore }

func NewProjectDeletionService(store ProjectDeletionStore) (*ProjectDeletionService, error) {
	if store == nil {
		return nil, ErrProjectDeletionInvalid
	}
	return &ProjectDeletionService{store: store}, nil
}
func (service *ProjectDeletionService) DeleteProjectRequirements(ctx context.Context, workspaceID, projectSetID uint, requirementIDs []uint) error {
	if service == nil || service.store == nil || workspaceID == 0 || projectSetID == 0 {
		return ErrProjectDeletionInvalid
	}
	return service.store.DeleteProjectRequirements(ctx, workspaceID, projectSetID, requirementIDs)
}
