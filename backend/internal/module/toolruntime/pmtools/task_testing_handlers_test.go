package pmtools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type taskTestingHandlerFake struct {
	workspace   uint
	lookup      task.Lookup
	scope       task.WorkflowScope
	workflowErr error
}

func (fake *taskTestingHandlerFake) GetProjectSet(_ context.Context, workspaceID, id uint) (project.ProjectSetSnapshot, error) {
	fake.workspace = workspaceID
	return project.ProjectSetSnapshot{ID: id, WorkspaceID: workspaceID, Name: "PM", Status: "active"}, nil
}
func (fake *taskTestingHandlerFake) GetPlanUnit(context.Context, uint, uint) (project.PlanUnitSnapshot, error) {
	return project.PlanUnitSnapshot{}, nil
}
func (fake *taskTestingHandlerFake) GetRequirement(_ context.Context, workspaceID, id uint) (requirement.Snapshot, error) {
	fake.workspace = workspaceID
	return requirement.Snapshot{ID: id, WorkspaceID: workspaceID, Title: "Requirement", Priority: "normal", Status: "backlog"}, nil
}
func (fake *taskTestingHandlerFake) GetTask(_ context.Context, workspaceID uint, lookup task.Lookup) (task.Snapshot, error) {
	fake.workspace, fake.lookup = workspaceID, lookup
	return task.Snapshot{
		ID: 44, WorkspaceID: workspaceID, ProjectSetID: 11, RequirementID: 13,
		Kind: task.WorkTaskKindTask, TaskKey: "PM-44", Title: "Task", Status: "in_dev",
		Priority: "normal", Children: []task.Reference{},
	}, nil
}
func (fake *taskTestingHandlerFake) GetDefinition(_ context.Context, workspaceID, taskID uint) (task.TestDefinitionSnapshot, error) {
	fake.workspace = workspaceID
	now := time.Date(2026, 8, 27, 6, 0, 0, 0, time.UTC)
	return task.TestDefinitionSnapshot{
		ID: 301, WorkspaceID: workspaceID, WorkTaskID: taskID, Version: 2,
		Cases:           []task.TestCase{{Key: "TC-1", Title: "主路径", Steps: []task.TestStep{{Action: "打开"}}}},
		CreatedByUserID: 7, UpdatedByUserID: 7, CreatedAt: now, UpdatedAt: now,
	}, nil
}
func (fake *taskTestingHandlerFake) ListExecutions(_ context.Context, workspaceID, taskID uint, _, _ int) (task.TestExecutionPage, error) {
	fake.workspace = workspaceID
	return task.TestExecutionPage{Items: []task.TestExecutionSnapshot{{
		ID: 401, WorkspaceID: workspaceID, WorkTaskID: taskID, TaskTestDefinitionID: 301,
		DefinitionVersion: 2, Environment: "uat", ExecutorUserID: 7, Status: "running", RunCounts: map[string]int64{"total": 1},
	}}, Total: 1}, nil
}
func (fake *taskTestingHandlerFake) GetExecution(_ context.Context, workspaceID, taskID, executionID uint) (task.TestExecutionSnapshot, error) {
	fake.workspace = workspaceID
	return task.TestExecutionSnapshot{
		ID: executionID, WorkspaceID: workspaceID, WorkTaskID: taskID, TaskTestDefinitionID: 301,
		DefinitionVersion: 2, Environment: "uat", ExecutorUserID: 7, Status: "running", RunCounts: map[string]int64{"total": 1},
	}, nil
}
func (fake *taskTestingHandlerFake) ListExecutionRuns(_ context.Context, workspaceID, taskID, executionID uint, _ task.TestRunFilter) (task.TestExecutionRunPage, error) {
	fake.workspace = workspaceID
	return task.TestExecutionRunPage{Items: []task.TestExecutionRunSnapshot{{
		ID: 501, WorkspaceID: workspaceID, WorkTaskID: taskID, TaskTestExecutionID: executionID,
		Version: 1, Sequence: 1, CaseKey: "TC-1", Title: "主路径", Steps: []task.TestStep{{Action: "打开"}},
		StepResults: []task.TestStepResult{}, Status: "pending", Evidence: []task.TestEvidence{},
	}}, Total: 1}, nil
}
func (fake *taskTestingHandlerFake) ListKinds(_ context.Context, workspaceID uint, scope task.WorkflowScope) ([]task.WorkflowKind, error) {
	fake.workspace, fake.scope = workspaceID, scope
	return task.StandardWorkflowKinds(), fake.workflowErr
}
func (fake *taskTestingHandlerFake) ListStatuses(_ context.Context, workspaceID uint, _ string, scope task.WorkflowScope) ([]task.WorkflowStatus, error) {
	fake.workspace, fake.scope = workspaceID, scope
	return task.StandardWorkflowStatuses(), fake.workflowErr
}
func (fake *taskTestingHandlerFake) ListTransitions(_ context.Context, workspaceID uint, _ string, scope task.WorkflowScope) (task.WorkflowTransitionProfile, error) {
	fake.workspace, fake.scope = workspaceID, scope
	return task.StandardWorkflowTransitionProfile(), fake.workflowErr
}

type taskTestingAuthorizer struct{}

func (taskTestingAuthorizer) AuthorizeGrant(context.Context, toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant(TaskTestingReadHandlerNames(), []string{"task:read"})
}

func TestTaskTestingReadHandlersInvokeAllSixThroughCanonicalApplication(t *testing.T) {
	fake := &taskTestingHandlerFake{}
	handlers, err := NewTaskTestingReadHandlers(TaskTestingReadDependencies{
		Projects: fake, Requirements: fake, Tasks: fake, Testing: fake, Workflow: fake,
	})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: TaskTestingReadHandlerNames()})
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(catalog, taskTestingAuthorizer{}, handlers)
	if err != nil {
		t.Fatal(err)
	}
	authorization := toolruntime.ExecutionAuthorization{
		PrincipalID: 1, WorkspaceID: 7, GrantID: 2, ClientID: "test-client",
		Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
	}
	inputs := map[string]json.RawMessage{
		TaskTestingDefinitionGet:    json.RawMessage(`{"task_ref":"PM-44"}`),
		TaskTestingContextGet:       json.RawMessage(`{"id":44}`),
		TaskTestingExecutionRunList: json.RawMessage(`{"id":44,"execution_id":401,"status":"pending","limit":20,"offset":0}`),
		TaskWorkflowKindList:        json.RawMessage(`{"team_id":10,"project_id":12}`),
		TaskWorkflowStatusList:      json.RawMessage(`{"kind":"task"}`),
		TaskWorkflowTransitionList:  json.RawMessage(`{"kind":"subtask"}`),
	}
	for _, name := range TaskTestingReadHandlerNames() {
		result, invokeErr := application.Invoke(context.Background(), toolruntime.Invocation{
			Name: name, Input: inputs[name], Authorization: authorization,
		})
		if invokeErr != nil {
			t.Fatalf("invoke %s: %v", name, invokeErr)
		}
		if result.Name != name || len(result.Output) == 0 {
			t.Fatalf("invoke %s result=%+v", name, result)
		}
	}
	if fake.workspace != 7 {
		t.Fatalf("workspace=%d", fake.workspace)
	}
}

func TestTaskTestingReadHandlersRejectUnknownInputAndMissingWorkspace(t *testing.T) {
	fake := &taskTestingHandlerFake{}
	handlers, _ := NewTaskTestingReadHandlers(TaskTestingReadDependencies{
		Projects: fake, Requirements: fake, Tasks: fake, Testing: fake, Workflow: fake,
	})
	if _, err := handlers[TaskTestingDefinitionGet].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7}, Input: json.RawMessage(`{"unknown":true}`),
	}); err == nil {
		t.Fatal("expected unknown field rejection")
	}
	if _, err := handlers[TaskWorkflowKindList].Invoke(context.Background(), toolruntime.ToolCall{Input: json.RawMessage(`{}`)}); !errors.Is(err, ErrReadHandlersInvalid) {
		t.Fatalf("missing workspace err=%v", err)
	}
}

func TestTaskWorkflowReadHandlersReturnControlledInputErrorsForInvalidScopes(t *testing.T) {
	fake := &taskTestingHandlerFake{workflowErr: task.ErrWorkflowReferenceInvalid}
	handlers, err := NewTaskTestingReadHandlers(TaskTestingReadDependencies{
		Projects: fake, Requirements: fake, Tasks: fake, Testing: fake, Workflow: fake,
	})
	if err != nil {
		t.Fatal(err)
	}
	call := toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7},
		Input:         json.RawMessage(`{"kind":"task","team_id":10}`),
	}
	for _, name := range []string{TaskWorkflowStatusList, TaskWorkflowTransitionList} {
		if _, invokeErr := handlers[name].Invoke(context.Background(), call); !errors.Is(invokeErr, toolruntime.ErrToolInputInvalid) {
			t.Fatalf("%s err=%v", name, invokeErr)
		}
	}

	call.Input = json.RawMessage(`{"kind":"task","project_id":12}`)
	if _, invokeErr := handlers[TaskWorkflowStatusList].Invoke(context.Background(), call); !errors.Is(invokeErr, toolruntime.ErrToolInputInvalid) {
		t.Fatalf("project-only scope err=%v", invokeErr)
	}
}
