package pmtools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const (
	TaskTestingDefinitionGet    = "task.testing.definition.get"
	TaskTestingContextGet       = "task.testing.context.get"
	TaskTestingExecutionRunList = "task.testing.execution_run.list"
	TaskWorkflowKindList        = "task.workflow.kind.list"
	TaskWorkflowStatusList      = "task.workflow.status.list"
	TaskWorkflowTransitionList  = "task.workflow.transition.list"
)

var taskTestingReadHandlerNames = []string{
	TaskTestingDefinitionGet,
	TaskTestingContextGet,
	TaskTestingExecutionRunList,
	TaskWorkflowKindList,
	TaskWorkflowStatusList,
	TaskWorkflowTransitionList,
}

type TaskTestingQueries interface {
	GetDefinition(context.Context, uint, uint) (task.TestDefinitionSnapshot, error)
	ListExecutions(context.Context, uint, uint, int, int) (task.TestExecutionPage, error)
	GetExecution(context.Context, uint, uint, uint) (task.TestExecutionSnapshot, error)
	ListExecutionRuns(context.Context, uint, uint, uint, task.TestRunFilter) (task.TestExecutionRunPage, error)
}

type TaskWorkflowReferenceQueries interface {
	ListKinds(context.Context, uint, task.WorkflowScope) ([]task.WorkflowKind, error)
	ListStatuses(context.Context, uint, string, task.WorkflowScope) ([]task.WorkflowStatus, error)
	ListTransitions(context.Context, uint, string, task.WorkflowScope) (task.WorkflowTransitionProfile, error)
}

type TaskTestingReadDependencies struct {
	Projects     ProjectQueries
	Requirements RequirementQueries
	Tasks        TaskQueries
	Testing      TaskTestingQueries
	Workflow     TaskWorkflowReferenceQueries
}

type taskTestDefinitionOutput struct {
	task.TestDefinitionSnapshot
	Task task.Snapshot `json:"task"`
}

type taskTestingContextOutput struct {
	Task        task.Snapshot                `json:"task"`
	Project     project.ProjectSetSnapshot   `json:"project"`
	Requirement requirement.Snapshot         `json:"requirement"`
	Definition  *task.TestDefinitionSnapshot `json:"definition"`
	Executions  task.TestExecutionPage       `json:"executions"`
}

type taskTestingRunListOutput struct {
	Task      task.Snapshot                   `json:"task"`
	Execution task.TestExecutionSnapshot      `json:"execution"`
	Items     []task.TestExecutionRunSnapshot `json:"items"`
	Total     int64                           `json:"total"`
}

func TaskTestingReadHandlerNames() []string {
	return append([]string(nil), taskTestingReadHandlerNames...)
}

func NewTaskTestingReadHandlers(dependencies TaskTestingReadDependencies) (map[string]toolruntime.ToolHandler, error) {
	if dependencies.Projects == nil || dependencies.Requirements == nil || dependencies.Tasks == nil ||
		dependencies.Testing == nil || dependencies.Workflow == nil {
		return nil, ErrReadHandlersInvalid
	}
	return map[string]toolruntime.ToolHandler{
		TaskTestingDefinitionGet: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			taskItem, err := resolveTestingTask(ctx, call, dependencies.Tasks)
			if err != nil {
				return nil, err
			}
			definition, err := dependencies.Testing.GetDefinition(ctx, call.Authorization.WorkspaceID, taskItem.ID)
			if err != nil {
				return nil, err
			}
			return taskTestDefinitionOutput{TestDefinitionSnapshot: definition, Task: taskItem}, nil
		}),
		TaskTestingContextGet: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			taskItem, err := resolveTestingTask(ctx, call, dependencies.Tasks)
			if err != nil {
				return nil, err
			}
			projectItem, err := dependencies.Projects.GetProjectSet(ctx, call.Authorization.WorkspaceID, taskItem.ProjectSetID)
			if err != nil {
				return nil, err
			}
			requirementItem, err := dependencies.Requirements.GetRequirement(ctx, call.Authorization.WorkspaceID, taskItem.RequirementID)
			if err != nil {
				return nil, err
			}
			definition, definitionErr := dependencies.Testing.GetDefinition(ctx, call.Authorization.WorkspaceID, taskItem.ID)
			var definitionPointer *task.TestDefinitionSnapshot
			switch {
			case definitionErr == nil:
				definitionPointer = &definition
			case !errors.Is(definitionErr, task.ErrTestDefinitionNotFound):
				return nil, definitionErr
			}
			executions, err := dependencies.Testing.ListExecutions(ctx, call.Authorization.WorkspaceID, taskItem.ID, 50, 0)
			if err != nil {
				return nil, err
			}
			return taskTestingContextOutput{
				Task: taskItem, Project: projectItem, Requirement: requirementItem,
				Definition: definitionPointer, Executions: executions,
			}, nil
		}),
		TaskTestingExecutionRunList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			var input taskTestingRunListInput
			if err := decodeInput(call.Input, &input); err != nil || input.ExecutionID == 0 || input.Offset < 0 {
				return nil, fmt.Errorf("%w: task reference, execution_id, and pagination are required", ErrReadHandlersInvalid)
			}
			taskItem, err := dependencies.Tasks.GetTask(ctx, call.Authorization.WorkspaceID, input.lookup())
			if err != nil {
				return nil, err
			}
			execution, err := dependencies.Testing.GetExecution(ctx, call.Authorization.WorkspaceID, taskItem.ID, input.ExecutionID)
			if err != nil {
				return nil, err
			}
			runs, err := dependencies.Testing.ListExecutionRuns(ctx, call.Authorization.WorkspaceID, taskItem.ID, input.ExecutionID, task.TestRunFilter{
				Status: input.Status, Limit: input.Limit, Offset: input.Offset,
			})
			if err != nil {
				return nil, err
			}
			return taskTestingRunListOutput{Task: taskItem, Execution: execution, Items: runs.Items, Total: runs.Total}, nil
		}),
		TaskWorkflowKindList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := workflowReferenceInputFrom(call.Input, false)
			if err != nil {
				return nil, toolruntime.ErrToolInputInvalid
			}
			items, err := dependencies.Workflow.ListKinds(ctx, call.Authorization.WorkspaceID, input.scope())
			if errors.Is(err, task.ErrWorkflowReferenceInvalid) {
				return nil, toolruntime.ErrToolInputInvalid
			}
			return makeListOutput(items), err
		}),
		TaskWorkflowStatusList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := workflowReferenceInputFrom(call.Input, true)
			if err != nil {
				return nil, toolruntime.ErrToolInputInvalid
			}
			items, err := dependencies.Workflow.ListStatuses(ctx, call.Authorization.WorkspaceID, input.Kind, input.scope())
			if errors.Is(err, task.ErrWorkflowReferenceInvalid) {
				return nil, toolruntime.ErrToolInputInvalid
			}
			return makeListOutput(items), err
		}),
		TaskWorkflowTransitionList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := workflowReferenceInputFrom(call.Input, true)
			if err != nil {
				return nil, toolruntime.ErrToolInputInvalid
			}
			profile, err := dependencies.Workflow.ListTransitions(ctx, call.Authorization.WorkspaceID, input.Kind, input.scope())
			if errors.Is(err, task.ErrWorkflowReferenceInvalid) {
				return nil, toolruntime.ErrToolInputInvalid
			}
			return profile, err
		}),
	}, nil
}

func resolveTestingTask(ctx context.Context, call toolruntime.ToolCall, queries TaskQueries) (task.Snapshot, error) {
	var input taskGetInput
	if err := decodeInput(call.Input, &input); err != nil {
		return task.Snapshot{}, err
	}
	return queries.GetTask(ctx, call.Authorization.WorkspaceID, input.lookup())
}

func (input taskGetInput) lookup() task.Lookup {
	return task.Lookup{
		ID: input.ID, Reference: input.TaskRef, ProjectSetID: input.ProjectSetID,
		PlanUnitID: input.PlanUnitID, RequirementID: input.RequirementID,
	}
}

type taskTestingRunListInput struct {
	taskGetInput
	ExecutionID uint   `json:"execution_id"`
	Status      string `json:"status"`
	Limit       int    `json:"limit"`
	Offset      int    `json:"offset"`
}

type workflowReferenceInput struct {
	TeamID    uint   `json:"team_id"`
	ProjectID uint   `json:"project_id"`
	Kind      string `json:"kind"`
}

func workflowReferenceInputFrom(raw json.RawMessage, requireKind bool) (workflowReferenceInput, error) {
	var input workflowReferenceInput
	if err := decodeInput(raw, &input); err != nil || (requireKind && !task.IsPMWorkTaskKind(input.Kind)) ||
		(input.ProjectID > 0 && input.TeamID == 0) {
		return workflowReferenceInput{}, fmt.Errorf("%w: workflow scope or kind is invalid", ErrReadHandlersInvalid)
	}
	return input, nil
}

func (input workflowReferenceInput) scope() task.WorkflowScope {
	return task.WorkflowScope{TeamID: input.TeamID, ProjectID: input.ProjectID}
}

var _ TaskTestingQueries = (*task.TestingReadService)(nil)
var _ TaskWorkflowReferenceQueries = (*task.WorkflowReferenceService)(nil)
