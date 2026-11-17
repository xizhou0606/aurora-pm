package requirement

import (
	"context"
	"errors"
)

var (
	ErrMutationScopeInvalid  = errors.New("requirement mutation scope is invalid")
	ErrMutationScopeNotFound = errors.New("requirement mutation scope not found")
)

// MutationScope is the immutable hierarchy fact consumed by another owning
// domain while applying a mutation. It deliberately contains no persistence
// or transport types.
type MutationScope struct {
	WorkspaceID   uint
	TeamID        uint
	ProjectSetID  uint
	PlanUnitID    uint
	RequirementID uint
	Version       uint
}

type MutationScopeStore interface {
	LockMutationScope(context.Context, uint, uint) (MutationScope, error)
}

type MutationScopeResolver interface {
	ResolveMutationScope(context.Context, uint, uint) (MutationScope, error)
}

type MutationScopeService struct{ store MutationScopeStore }

func NewMutationScopeService(store MutationScopeStore) (*MutationScopeService, error) {
	if store == nil {
		return nil, ErrMutationScopeInvalid
	}
	return &MutationScopeService{store: store}, nil
}

func (service *MutationScopeService) ResolveMutationScope(
	ctx context.Context, workspaceID, requirementID uint,
) (MutationScope, error) {
	if service == nil || service.store == nil || workspaceID == 0 || requirementID == 0 {
		return MutationScope{}, ErrMutationScopeInvalid
	}
	scope, err := service.store.LockMutationScope(ctx, workspaceID, requirementID)
	if err != nil {
		return MutationScope{}, err
	}
	if scope.WorkspaceID != workspaceID || scope.RequirementID != requirementID || scope.TeamID == 0 ||
		scope.ProjectSetID == 0 || scope.PlanUnitID == 0 || scope.Version == 0 {
		return MutationScope{}, ErrMutationScopeInvalid
	}
	return scope, nil
}

var _ MutationScopeResolver = (*MutationScopeService)(nil)
