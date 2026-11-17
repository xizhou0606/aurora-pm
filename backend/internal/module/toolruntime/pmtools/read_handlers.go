// Package pmtools adapts canonical Tool Runtime calls to PM-owned domain queries.
package pmtools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
)

const (
	ProjectSetGet         = "project.set.get"
	ProjectPlanUnitGet    = "project.plan_unit.get"
	ProjectRequirementGet = "project.requirement.get"
	TaskGet               = "task.get"
	WorkflowTemplateList  = "workflow.template.list"
	WorkflowTemplateGet   = "workflow.template.get"
	WorkflowInstanceList  = "workflow.instance.list"
	WorkflowInstanceGet   = "workflow.instance.get"
)

var ErrReadHandlersInvalid = errors.New("PM read tool handlers are invalid")

var readHandlerNames = []string{
	ProjectSetGet,
	ProjectPlanUnitGet,
	ProjectRequirementGet,
	TaskGet,
	WorkflowTemplateList,
	WorkflowTemplateGet,
	WorkflowInstanceList,
	WorkflowInstanceGet,
}

type ProjectQueries interface {
	GetProjectSet(context.Context, uint, uint) (project.ProjectSetSnapshot, error)
	GetPlanUnit(context.Context, uint, uint) (project.PlanUnitSnapshot, error)
}

type RequirementQueries interface {
	GetRequirement(context.Context, uint, uint) (requirement.Snapshot, error)
}

type TaskQueries interface {
	GetTask(context.Context, uint, task.Lookup) (task.Snapshot, error)
}

type WorkflowQueries interface {
	ListTemplates(context.Context, uint, workflow.TemplateFilter) ([]workflow.TemplateSnapshot, error)
	GetTemplate(context.Context, uint, uint) (workflow.TemplateSnapshot, error)
	ListInstances(context.Context, uint, workflow.InstanceFilter) ([]workflow.InstanceSnapshot, error)
	GetInstance(context.Context, uint, uint) (workflow.InstanceSnapshot, error)
}

type ReadDependencies struct {
	Projects     ProjectQueries
	Requirements RequirementQueries
	Tasks        TaskQueries
	Workflows    WorkflowQueries
}

// ReadHandlerNames is the explicit activation contract for the first read
// slice. Catalog owners must keep every other migrated operation in draft.
func ReadHandlerNames() []string { return append([]string(nil), readHandlerNames...) }

func NewReadHandlers(dependencies ReadDependencies) (map[string]toolruntime.ToolHandler, error) {
	if dependencies.Projects == nil || dependencies.Requirements == nil || dependencies.Tasks == nil || dependencies.Workflows == nil {
		return nil, ErrReadHandlersInvalid
	}
	handlers := map[string]toolruntime.ToolHandler{
		ProjectSetGet: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := inputID(call.Input)
			if err != nil {
				return nil, err
			}
			return dependencies.Projects.GetProjectSet(ctx, call.Authorization.WorkspaceID, input.ID)
		}),
		ProjectPlanUnitGet: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := inputID(call.Input)
			if err != nil {
				return nil, err
			}
			return dependencies.Projects.GetPlanUnit(ctx, call.Authorization.WorkspaceID, input.ID)
		}),
		ProjectRequirementGet: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := inputID(call.Input)
			if err != nil {
				return nil, err
			}
			return dependencies.Requirements.GetRequirement(ctx, call.Authorization.WorkspaceID, input.ID)
		}),
		TaskGet: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			var input taskGetInput
			if err := decodeInput(call.Input, &input); err != nil {
				return nil, err
			}
			return dependencies.Tasks.GetTask(ctx, call.Authorization.WorkspaceID, task.Lookup{
				ID: input.ID, Reference: input.TaskRef, ProjectSetID: input.ProjectSetID,
				PlanUnitID: input.PlanUnitID, RequirementID: input.RequirementID,
			})
		}),
		WorkflowTemplateList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			var input workflowTemplateListInput
			if err := decodeInput(call.Input, &input); err != nil {
				return nil, err
			}
			items, err := dependencies.Workflows.ListTemplates(ctx, call.Authorization.WorkspaceID, workflow.TemplateFilter{SubjectType: input.SubjectType})
			return makeListOutput(items), err
		}),
		WorkflowTemplateGet: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := inputID(call.Input)
			if err != nil {
				return nil, err
			}
			return dependencies.Workflows.GetTemplate(ctx, call.Authorization.WorkspaceID, input.ID)
		}),
		WorkflowInstanceList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			var input workflowInstanceListInput
			if err := decodeInput(call.Input, &input); err != nil {
				return nil, err
			}
			if err := validateWorkflowSubject(ctx, call.Authorization.WorkspaceID, input.SubjectType, input.SubjectID, dependencies); err != nil {
				return nil, err
			}
			items, err := dependencies.Workflows.ListInstances(ctx, call.Authorization.WorkspaceID, workflow.InstanceFilter{SubjectType: input.SubjectType, SubjectID: input.SubjectID})
			return makeListOutput(items), err
		}),
		WorkflowInstanceGet: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := inputID(call.Input)
			if err != nil {
				return nil, err
			}
			item, err := dependencies.Workflows.GetInstance(ctx, call.Authorization.WorkspaceID, input.ID)
			if err != nil {
				return nil, err
			}
			if err := validateWorkflowSubject(ctx, call.Authorization.WorkspaceID, item.SubjectType, item.SubjectID, dependencies); err != nil {
				return nil, err
			}
			return item, nil
		}),
	}
	return handlers, nil
}

func validateWorkflowSubject(ctx context.Context, workspaceID uint, subjectType string, subjectID uint, dependencies ReadDependencies) error {
	switch subjectType {
	case "plan_unit":
		_, err := dependencies.Projects.GetPlanUnit(ctx, workspaceID, subjectID)
		return err
	case "requirement":
		_, err := dependencies.Requirements.GetRequirement(ctx, workspaceID, subjectID)
		return err
	default:
		return workflow.ErrReadQueryInvalid
	}
}

func handler(invoke func(context.Context, toolruntime.ToolCall) (any, error)) toolruntime.ToolHandler {
	return toolruntime.ToolHandlerFunc(func(ctx context.Context, call toolruntime.ToolCall) (json.RawMessage, error) {
		if call.Authorization.WorkspaceID == 0 {
			return nil, ErrReadHandlersInvalid
		}
		output, err := invoke(ctx, call)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(output)
		if err != nil {
			return nil, err
		}
		return raw, nil
	})
}
