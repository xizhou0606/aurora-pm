package pmtools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

type workflowMutationCapture struct {
	command workflow.Mutation
}

type workflowWriteAuthorizer struct{ tool string }

func (authorizer workflowWriteAuthorizer) AuthorizeGrant(
	context.Context,
	toolruntime.ExecutionAuthorization,
) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant([]string{authorizer.tool}, []string{"workflow:write"})
}

func TestWorkflowActionExecuteRunsPendingConfirmSucceededAndReplay(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(toolruntimegorm.ChangeSetMigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	capture := &workflowMutationCapture{}
	handlers, err := NewWorkflowWriteHandlers(capture)
	if err != nil {
		t.Fatal(err)
	}
	source, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: []string{WorkflowActionExecute}})
	if err != nil {
		t.Fatal(err)
	}
	definition, found := source.Find(WorkflowActionExecute)
	if !found {
		t.Fatal("workflow action definition missing")
	}
	catalog, err := toolruntime.NewCatalog([]toolruntime.Definition{definition})
	if err != nil {
		t.Fatal(err)
	}
	changeSets, err := toolruntime.NewChangeSetService(
		toolruntimegorm.NewChangeSetRepository(db), toolruntime.DefaultChangeSetPolicy(),
	)
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(
		catalog,
		workflowWriteAuthorizer{tool: WorkflowActionExecute},
		map[string]toolruntime.ToolHandler{WorkflowActionExecute: handlers[WorkflowActionExecute]},
		toolruntime.WithChangeSetService(changeSets),
	)
	if err != nil {
		t.Fatal(err)
	}
	authorization := toolruntime.ExecutionAuthorization{
		PrincipalID: 73, WorkspaceID: 19, GrantID: 5, ClientID: "pm-cli",
		Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
	}
	pending, err := application.Invoke(context.Background(), toolruntime.Invocation{
		Name:           WorkflowActionExecute,
		Input:          json.RawMessage(`{"id":31,"action":"approve","node_id":"review","comment":"通过","next_node_assignees":[{"node_id":"next","assignee_user_id":81}]}`),
		IdempotencyKey: "workflow-action-invoke-0001", Authorization: authorization,
	})
	if err != nil || pending.ChangeSet == nil || pending.ChangeSet.Status != toolruntime.ChangeSetPending {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	succeeded, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if err != nil || succeeded.Status != toolruntime.ChangeSetSucceeded || capture.command.WorkspaceID != 19 || capture.command.PrincipalID != 73 {
		t.Fatalf("succeeded=%+v command=%+v err=%v", succeeded, capture.command, err)
	}
	replayed, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if err != nil || replayed.Status != toolruntime.ChangeSetSucceeded || string(replayed.Result) != string(succeeded.Result) {
		t.Fatalf("replay=%+v err=%v", replayed, err)
	}
}

func (capture *workflowMutationCapture) ApplyWorkflowMutation(
	_ context.Context,
	command workflow.Mutation,
) (workflow.MutationResult, error) {
	capture.command = command
	return workflow.MutationResult{
		ID: command.InstanceID, WorkspaceID: command.WorkspaceID, InstanceID: command.InstanceID,
		NodeID: command.NodeID, Status: "running", Action: command.Action, CurrentNodeIDs: []string{"next"},
	}, nil
}

func TestWorkflowWriteHandlersAreChangeSetMutationsAndUseAuthorizationFacts(t *testing.T) {
	capture := &workflowMutationCapture{}
	handlers, err := NewWorkflowWriteHandlers(capture)
	if err != nil {
		t.Fatal(err)
	}
	if len(WorkflowWriteHandlerNames()) != 5 || len(handlers) != 5 {
		t.Fatalf("names=%v handlers=%d", WorkflowWriteHandlerNames(), len(handlers))
	}
	for _, name := range WorkflowWriteHandlerNames() {
		handler, found := handlers[name]
		if !found {
			t.Fatalf("handler %s missing", name)
		}
		if _, ok := handler.(toolruntime.IdempotentMutationHandler); !ok {
			t.Fatalf("handler %s is not idempotent mutation", name)
		}
		if _, invokeErr := handler.Invoke(context.Background(), toolruntime.ToolCall{}); !errors.Is(invokeErr, toolruntime.ErrChangeSetRequired) {
			t.Fatalf("handler %s invoke err=%v", name, invokeErr)
		}
	}

	action := handlers[WorkflowActionExecute].(toolruntime.IdempotentMutationHandler)
	output, err := action.Apply(context.Background(), toolruntime.MutationCall{
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 73, WorkspaceID: 19},
		Input:         json.RawMessage(`{"id":31,"action":"approve","node_id":"review","comment":"通过","next_node_assignees":[{"node_id":"delivery","assignee_user_id":81}]}`),
		ChangeSetID:   "changeset-workflow-0001", ApplyKey: "apply-workflow-0001",
	})
	if err != nil || capture.command.Operation != workflow.ActionExecute || capture.command.WorkspaceID != 19 ||
		capture.command.PrincipalID != 73 || capture.command.InstanceID != 31 || capture.command.NextAssignees["delivery"] != 81 ||
		capture.command.ApplyKey != "apply-workflow-0001" || len(capture.command.InputDigest) != 64 {
		t.Fatalf("command=%+v output=%s err=%v", capture.command, output, err)
	}
	var result workflow.MutationResult
	if json.Unmarshal(output, &result) != nil || result.WorkspaceID != 19 || result.Action != "approve" {
		t.Fatalf("output=%s", output)
	}
}

func TestWorkflowActionExecuteIsActivationEligibleWithStrictSchema(t *testing.T) {
	catalog, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: WorkflowWriteHandlerNames()})
	if err != nil {
		t.Fatal(err)
	}
	definition, found := catalog.Find(WorkflowActionExecute)
	if !found || definition.Lifecycle != toolruntime.LifecycleActive || definition.ChangeSet != toolruntime.ChangeSetRequired {
		t.Fatalf("definition=%+v found=%v", definition, found)
	}
	var schema map[string]any
	if json.Unmarshal(definition.InputSchema, &schema) != nil || schema["additionalProperties"] != false {
		t.Fatalf("input schema=%s", definition.InputSchema)
	}
	properties, _ := schema["properties"].(map[string]any)
	if properties["action"] == nil || properties["next_node_assignees"] == nil {
		t.Fatalf("action schema properties=%v", properties)
	}
}
