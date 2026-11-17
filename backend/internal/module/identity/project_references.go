package identity

import (
	"context"
	"errors"
)

var ErrProjectReferencesInvalid = errors.New("project references are invalid")

type ProjectReferenceStore interface {
	ProjectReferencesValid(context.Context, uint, uint, []uint) (bool, error)
}

type ProjectReferenceService struct{ store ProjectReferenceStore }

func NewProjectReferenceService(store ProjectReferenceStore) (*ProjectReferenceService, error) {
	if store == nil {
		return nil, ErrProjectReferencesInvalid
	}
	return &ProjectReferenceService{store: store}, nil
}

func (service *ProjectReferenceService) ValidateProjectReferences(
	ctx context.Context,
	workspaceID uint,
	teamID uint,
	userIDs []uint,
) error {
	valid, err := service.ProjectReferencesValid(ctx, workspaceID, teamID, userIDs)
	if err != nil {
		return err
	}
	if !valid {
		return ErrProjectReferencesInvalid
	}
	return nil
}

func (service *ProjectReferenceService) ProjectReferencesValid(
	ctx context.Context,
	workspaceID uint,
	teamID uint,
	userIDs []uint,
) (bool, error) {
	if service == nil || service.store == nil || workspaceID == 0 {
		return false, ErrProjectReferencesInvalid
	}
	unique := make([]uint, 0, len(userIDs))
	seen := make(map[uint]struct{}, len(userIDs))
	for _, id := range userIDs {
		if id == 0 {
			return false, ErrProjectReferencesInvalid
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	valid, err := service.store.ProjectReferencesValid(ctx, workspaceID, teamID, unique)
	if err != nil {
		return false, err
	}
	return valid, nil
}
