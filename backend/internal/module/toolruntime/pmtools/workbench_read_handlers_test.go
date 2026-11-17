package pmtools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
)

type workbenchReadFake struct {
	workspace uint
	principal uint
	filter    workbench.TodoFilter
}

func (fake *workbenchReadFake) TodoSummary(_ context.Context, workspaceID, principalID uint, filter workbench.TodoFilter) (workbench.TodoSummaryOutput, error) {
	fake.workspace, fake.principal, fake.filter = workspaceID, principalID, filter
	return workbench.TodoSummaryOutput{Summary: workbench.TodoSummary{}, GeneratedAt: time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)}, nil
}

func (fake *workbenchReadFake) TodoFeed(_ context.Context, workspaceID, principalID uint, filter workbench.TodoFilter) (workbench.TodoFeedOutput, error) {
	fake.workspace, fake.principal, fake.filter = workspaceID, principalID, filter
	return workbench.TodoFeedOutput{Items: []workbench.TodoFeedItem{}, Total: 0}, nil
}

type workbenchReadAuthorizer struct{}

func (workbenchReadAuthorizer) AuthorizeGrant(context.Context, toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant(WorkbenchReadHandlerNames(), []string{"workbench:read"})
}

func TestWorkbenchReadHandlersUseServerWorkspaceAndPrincipalThroughApplication(t *testing.T) {
	fake := &workbenchReadFake{}
	handlers, err := NewWorkbenchReadHandlers(fake)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: WorkbenchReadHandlerNames()})
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(catalog, workbenchReadAuthorizer{}, handlers)
	if err != nil {
		t.Fatal(err)
	}
	authorization := toolruntime.ExecutionAuthorization{
		PrincipalID: 19, WorkspaceID: 7, GrantID: 8, ClientID: "workbench-read-test",
		Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
	}
	for name, input := range map[string]json.RawMessage{
		WorkbenchTodoSummary: json.RawMessage(`{"domains":"task,bug","include_mentions":true}`),
		WorkbenchTodoFeed:    json.RawMessage(`{"type":"workflow","limit":10,"offset":0}`),
	} {
		result, invokeErr := application.Invoke(context.Background(), toolruntime.Invocation{Name: name, Input: input, Authorization: authorization})
		if invokeErr != nil {
			t.Fatalf("invoke %s: %v", name, invokeErr)
		}
		if result.Name != name || len(result.Output) == 0 {
			t.Fatalf("result=%+v", result)
		}
		if fake.workspace != 7 || fake.principal != 19 {
			t.Fatalf("workspace=%d principal=%d", fake.workspace, fake.principal)
		}
	}
	if _, err := handlers[WorkbenchTodoSummary].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7}, Input: json.RawMessage(`{}`),
	}); err == nil {
		t.Fatal("expected missing principal rejection")
	}
}
