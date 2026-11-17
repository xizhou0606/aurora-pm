package ai

import (
	"context"
	"errors"
	"strings"
)

var ErrWorkflowActionConflict = errors.New("AI workflow action conflicts with the latest run")

type WorkflowActionCommand struct {
	WorkspaceID uint
	PrincipalID uint
	RunID       uint
	Capability  string
	Action      string
}

type WorkflowActionResult struct {
	RunID uint
	Path  string
}

type WorkflowActionStore interface {
	PrepareWorkflowAction(context.Context, WorkflowActionCommand) (WorkflowActionResult, error)
}

type WorkflowActionService struct{ store WorkflowActionStore }

func NewWorkflowActionService(store WorkflowActionStore) (*WorkflowActionService, error) {
	if store == nil {
		return nil, ErrWorkflowActionConflict
	}
	return &WorkflowActionService{store: store}, nil
}

func (service *WorkflowActionService) PrepareWorkflowAction(
	ctx context.Context,
	command WorkflowActionCommand,
) (WorkflowActionResult, error) {
	command.Capability, command.Action = strings.TrimSpace(command.Capability), strings.TrimSpace(command.Action)
	if service == nil || service.store == nil || command.WorkspaceID == 0 || command.PrincipalID == 0 ||
		command.RunID == 0 || command.Capability == "" ||
		(command.Action != "complete" && command.Action != "override_ai" && command.Action != "retry_ai" && command.Action != "recheck_ai") {
		return WorkflowActionResult{}, ErrWorkflowActionConflict
	}
	return service.store.PrepareWorkflowAction(ctx, command)
}
