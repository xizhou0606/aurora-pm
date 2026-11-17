package task

import (
	"context"
	"errors"
)

var (
	ErrCascadeDeletionInvalid  = errors.New("task cascade deletion preflight is invalid")
	ErrCascadeDeletionConflict = errors.New("task cascade deletion conflicts with attachments or an in-progress deletion")
)

type CascadeDeletionScope struct {
	WorkspaceID   uint
	ProjectSetID  uint
	RequirementID uint
	TaskIDs       []uint
}

type CascadeDeletionGuardStore interface {
	ValidateCascadeDeletion(context.Context, CascadeDeletionScope) error
}

type CascadeDeletionGuardService struct{ store CascadeDeletionGuardStore }

func NewCascadeDeletionGuardService(store CascadeDeletionGuardStore) (*CascadeDeletionGuardService, error) {
	if store == nil {
		return nil, ErrCascadeDeletionInvalid
	}
	return &CascadeDeletionGuardService{store: store}, nil
}

func (service *CascadeDeletionGuardService) ValidateCascadeDeletion(ctx context.Context, scope CascadeDeletionScope) error {
	selectors := 0
	if scope.ProjectSetID > 0 {
		selectors++
	}
	if scope.RequirementID > 0 {
		selectors++
	}
	if len(scope.TaskIDs) > 0 {
		selectors++
	}
	if service == nil || service.store == nil || scope.WorkspaceID == 0 || selectors != 1 || !positiveUniqueCascadeIDs(scope.TaskIDs) {
		return ErrCascadeDeletionInvalid
	}
	return service.store.ValidateCascadeDeletion(ctx, scope)
}

func positiveUniqueCascadeIDs(ids []uint) bool {
	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			return false
		}
		if _, exists := seen[id]; exists {
			return false
		}
		seen[id] = struct{}{}
	}
	return true
}
