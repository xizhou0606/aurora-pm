package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

var ErrWorkflowStartUnavailable = errors.New("AI workflow start is unavailable")

type WorkflowStartCommand struct {
	WorkspaceID, PrincipalID uint
	Capability               string
	SubjectType              string
	SubjectID                uint
	SubjectVersion           uint
	SubjectSnapshot          json.RawMessage
}

type WorkflowStartResult struct{ RunID uint }

type WorkflowStartStore interface {
	StartWorkflowNode(context.Context, WorkflowStartCommand) (WorkflowStartResult, error)
}

type WorkflowStartService struct{ store WorkflowStartStore }

func NewWorkflowStartService(store WorkflowStartStore) (*WorkflowStartService, error) {
	if store == nil {
		return nil, ErrWorkflowStartUnavailable
	}
	return &WorkflowStartService{store: store}, nil
}

func (service *WorkflowStartService) StartWorkflowNode(ctx context.Context, command WorkflowStartCommand) (WorkflowStartResult, error) {
	command.Capability, command.SubjectType = strings.TrimSpace(command.Capability), strings.TrimSpace(command.SubjectType)
	if service == nil || service.store == nil || command.WorkspaceID == 0 || command.PrincipalID == 0 ||
		command.Capability == "" || command.SubjectType != "plan_unit" && command.SubjectType != "requirement" || command.SubjectID == 0 ||
		command.SubjectVersion == 0 || len(command.SubjectSnapshot) == 0 || !json.Valid(command.SubjectSnapshot) {
		return WorkflowStartResult{}, ErrWorkflowStartUnavailable
	}
	return service.store.StartWorkflowNode(ctx, command)
}
