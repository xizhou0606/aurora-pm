package project

import (
	"context"
	"errors"
	"time"
)

var ErrPlanAssignmentInvalid = errors.New("plan assignment is invalid")

type PlanAssignment struct {
	WorkspaceID, TeamID, ProjectSetID, PlanUnitID uint
	ProjectCode, Type, MutationState              string
	// RequirementsLockedAt carries the requirement-intake lock fact read under
	// the shared plan unit row lock. Consumers decide what it means: only
	// requirement creation treats it as a hard block, moves and reschedules do
	// not.
	RequirementsLockedAt         *time.Time
	PlannedStartAt, PlannedEndAt *time.Time
}

type PlanAssignmentStore interface {
	ResolvePlanAssignment(context.Context, uint, uint) (PlanAssignment, error)
}

type PlanAssignmentService struct{ store PlanAssignmentStore }

func NewPlanAssignmentService(store PlanAssignmentStore) (*PlanAssignmentService, error) {
	if store == nil {
		return nil, ErrPlanAssignmentInvalid
	}
	return &PlanAssignmentService{store: store}, nil
}
func (service *PlanAssignmentService) ResolvePlanAssignment(ctx context.Context, workspaceID, planUnitID uint) (PlanAssignment, error) {
	if service == nil || service.store == nil || workspaceID == 0 || planUnitID == 0 {
		return PlanAssignment{}, ErrPlanAssignmentInvalid
	}
	return service.store.ResolvePlanAssignment(ctx, workspaceID, planUnitID)
}
