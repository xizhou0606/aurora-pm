package quality

import (
	"context"
	"errors"
)

var ErrRequirementLifecycleInvalid = errors.New("quality requirement lifecycle is invalid")

type RequirementLifecycleStore interface {
	MoveRequirementBugs(context.Context, uint, uint, uint, uint) (int64, error)
	DeleteRequirementBugs(context.Context, uint, uint) (int64, error)
}
type RequirementLifecycleService struct{ store RequirementLifecycleStore }

func NewRequirementLifecycleService(store RequirementLifecycleStore) (*RequirementLifecycleService, error) {
	if store == nil {
		return nil, ErrRequirementLifecycleInvalid
	}
	return &RequirementLifecycleService{store: store}, nil
}
func (service *RequirementLifecycleService) MoveRequirementBugs(ctx context.Context, workspaceID, requirementID, projectSetID, planUnitID uint) (int64, error) {
	if service == nil || workspaceID == 0 || requirementID == 0 || projectSetID == 0 || planUnitID == 0 {
		return 0, ErrRequirementLifecycleInvalid
	}
	return service.store.MoveRequirementBugs(ctx, workspaceID, requirementID, projectSetID, planUnitID)
}
func (service *RequirementLifecycleService) DeleteRequirementBugs(ctx context.Context, workspaceID, requirementID uint) (int64, error) {
	if service == nil || workspaceID == 0 || requirementID == 0 {
		return 0, ErrRequirementLifecycleInvalid
	}
	return service.store.DeleteRequirementBugs(ctx, workspaceID, requirementID)
}
