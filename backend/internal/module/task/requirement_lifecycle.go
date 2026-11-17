package task

import (
	"context"
	"errors"
)

var ErrRequirementLifecycleInvalid = errors.New("task requirement lifecycle is invalid")

type RequirementLifecycleStore interface {
	MoveRequirementTasks(context.Context, uint, uint, uint, uint, uint) (int64, error)
	DeleteRequirementTasks(context.Context, uint, uint) (int64, error)
}
type RequirementLifecycleService struct{ store RequirementLifecycleStore }

func NewRequirementLifecycleService(store RequirementLifecycleStore) (*RequirementLifecycleService, error) {
	if store == nil {
		return nil, ErrRequirementLifecycleInvalid
	}
	return &RequirementLifecycleService{store: store}, nil
}
func (service *RequirementLifecycleService) MoveRequirementTasks(ctx context.Context, workspaceID, requirementID, teamID, projectSetID, planUnitID uint) (int64, error) {
	if service == nil || workspaceID == 0 || requirementID == 0 || teamID == 0 || projectSetID == 0 || planUnitID == 0 {
		return 0, ErrRequirementLifecycleInvalid
	}
	return service.store.MoveRequirementTasks(ctx, workspaceID, requirementID, teamID, projectSetID, planUnitID)
}
func (service *RequirementLifecycleService) DeleteRequirementTasks(ctx context.Context, workspaceID, requirementID uint) (int64, error) {
	if service == nil || workspaceID == 0 || requirementID == 0 {
		return 0, ErrRequirementLifecycleInvalid
	}
	return service.store.DeleteRequirementTasks(ctx, workspaceID, requirementID)
}
