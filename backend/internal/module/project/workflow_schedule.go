package project

import (
	"context"
	"errors"
	"time"
)

var ErrWorkflowScheduleConflict = errors.New("workflow schedule conflicts with project boundaries")

type WorkflowScheduleCommand struct {
	WorkspaceID      uint
	ProjectSetID     uint
	PlanUnitID       uint
	SuggestedStartAt *time.Time
	SuggestedEndAt   *time.Time
}

type WorkflowScheduleStore interface {
	ConfirmWorkflowSchedule(context.Context, WorkflowScheduleCommand) error
}

type WorkflowScheduleService struct{ store WorkflowScheduleStore }

func NewWorkflowScheduleService(store WorkflowScheduleStore) (*WorkflowScheduleService, error) {
	if store == nil {
		return nil, ErrWorkflowScheduleConflict
	}
	return &WorkflowScheduleService{store: store}, nil
}

func (service *WorkflowScheduleService) ConfirmWorkflowSchedule(
	ctx context.Context,
	command WorkflowScheduleCommand,
) error {
	if service == nil || service.store == nil || command.WorkspaceID == 0 || command.ProjectSetID == 0 ||
		command.PlanUnitID == 0 || command.SuggestedStartAt == nil && command.SuggestedEndAt == nil ||
		command.SuggestedStartAt != nil && command.SuggestedEndAt != nil && command.SuggestedStartAt.After(*command.SuggestedEndAt) {
		return ErrWorkflowScheduleConflict
	}
	return service.store.ConfirmWorkflowSchedule(ctx, command)
}
