package ai

import (
	"context"
	"errors"
)

var ErrProjectDeletionInvalid = errors.New("ai project deletion is invalid")

type ProjectDeletionStore interface {
	DeleteProjectSubjects(context.Context, uint, ProjectSubjects) error
}

type ProjectDeletionService struct{ store ProjectDeletionStore }

func NewProjectDeletionService(store ProjectDeletionStore) (*ProjectDeletionService, error) {
	if store == nil {
		return nil, ErrProjectDeletionInvalid
	}
	return &ProjectDeletionService{store: store}, nil
}

func (service *ProjectDeletionService) DeleteProjectSubjects(ctx context.Context, workspaceID uint, subjects ProjectSubjects) error {
	if service == nil || service.store == nil || workspaceID == 0 {
		return ErrProjectDeletionInvalid
	}
	return service.store.DeleteProjectSubjects(ctx, workspaceID, subjects)
}
