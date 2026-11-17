package pmtools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
	"gorm.io/gorm"
)

func TestTaskBatchWriteHandlersDecodeAllOperationsAndServerAuthorization(t *testing.T) {
	service := &taskBatchMutationCapture{}
	handlers, err := NewTaskBatchWriteHandlers(service)
	if err != nil || len(handlers) != 5 || !reflect.DeepEqual(TaskBatchWriteHandlerNames(), []string{
		TaskBatchCreate, TaskBatchDelete, TaskBatchStatusUpdate, TaskBatchUpdate, TaskChildrenBatchCreate,
	}) {
		t.Fatalf("handlers=%v names=%v err=%v", handlers, TaskBatchWriteHandlerNames(), err)
	}
	for name, handler := range handlers {
		if _, ok := handler.(toolruntime.ToolHandler); !ok {
			t.Fatalf("%s is not ToolHandler", name)
		}
		if _, ok := handler.(toolruntime.IdempotentMutationHandler); !ok {
			t.Fatalf("%s is not IdempotentMutationHandler", name)
		}
		if _, invokeErr := handler.Invoke(context.Background(), toolruntime.ToolCall{}); !errors.Is(invokeErr, toolruntime.ErrChangeSetRequired) {
			t.Fatalf("%s invoke err=%v", name, invokeErr)
		}
	}

	cases := []struct {
		name  string
		input string
		want  func(task.BatchMutation) bool
	}{
		{TaskBatchCreate, `{"tasks":[{"client_id":"parent","requirement_id":13,"title":"Parent","priority":"high","starts_at":"2026-08-27T01:00:00+08:00","children":[{"client_id":"child","title":"Child"}]}],"idempotency_key":"caller-key"}`,
			func(command task.BatchMutation) bool {
				return command.Operation == task.BatchCreate && len(command.Tasks) == 1 && len(command.Tasks[0].Children) == 1 &&
					command.Tasks[0].StartsAt != nil && command.Tasks[0].StartsAt.Equal(time.Date(2026, 8, 26, 17, 0, 0, 0, time.UTC))
			}},
		{TaskChildrenBatchCreate, `{"id":71,"children":[{"client_id":"child","title":"Child","assignee_id":42}],"idempotency_key":"caller-key"}`,
			func(command task.BatchMutation) bool {
				return command.Operation == task.ChildrenBatchCreate && command.ParentID == 71 && len(command.Tasks) == 1 && *command.Tasks[0].AssigneeID == 42
			}},
		{TaskBatchDelete, `{"ids":[81],"task_refs":["PM-82"],"project_set_id":11,"project_new_id":12,"requirement_id":13,"cascade":true,"idempotency_key":"caller-key"}`,
			func(command task.BatchMutation) bool {
				return command.Operation == task.BatchDelete && reflect.DeepEqual(command.Selector.IDs, []uint{81}) &&
					reflect.DeepEqual(command.Selector.TaskRefs, []string{"PM-82"}) && command.Selector.PlanUnitID == 12 && command.Cascade
			}},
		{TaskBatchUpdate, `{"task_refs":["PM-83"],"updates":{"status":"completed","assignee_id":null,"starts_at":null,"due_at":"2026-08-30T10:00:00Z"},"idempotency_key":"caller-key"}`,
			func(command task.BatchMutation) bool {
				return command.Operation == task.BatchUpdate && command.Updates.Status.Set && *command.Updates.Status.Value == "completed" &&
					command.Updates.AssigneeID.Set && command.Updates.AssigneeID.Value == nil && command.Updates.StartsAt.Set && command.Updates.StartsAt.Value == nil &&
					command.Updates.DueAt.Set && command.Updates.DueAt.Value.Equal(time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC))
			}},
		{TaskBatchStatusUpdate, `{"ids":[84],"updates":{"status":"in_dev"},"idempotency_key":"caller-key"}`,
			func(command task.BatchMutation) bool {
				return command.Operation == task.BatchUpdate && command.Updates.Status.Set &&
					command.Updates.Status.Value != nil && *command.Updates.Status.Value == "in_dev"
			}},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			service.commands = nil
			output, applyErr := handlers[item.name].(toolruntime.IdempotentMutationHandler).Apply(context.Background(), toolruntime.MutationCall{
				Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7, PrincipalID: 41},
				ChangeSetID:   "changeset-batch-0001", ApplyKey: "apply-batch-0001", Input: json.RawMessage(item.input),
			})
			if applyErr != nil || len(service.commands) != 1 || !item.want(service.commands[0]) || service.commands[0].WorkspaceID != 7 ||
				service.commands[0].PrincipalID != 41 || service.commands[0].ChangeSetID != "changeset-batch-0001" || len(service.commands[0].InputDigest) != 64 {
				t.Fatalf("command=%+v output=%s err=%v", service.commands, output, applyErr)
			}
			var decoded task.BatchMutationResult
			if json.Unmarshal(output, &decoded) != nil || !decoded.Atomic {
				t.Fatalf("output=%s", output)
			}
		})
	}
}

func TestTaskBatchWriteHandlerRejectsUnknownInputAndClassifiesFailures(t *testing.T) {
	service := &taskBatchMutationCapture{}
	handlers, _ := NewTaskBatchWriteHandlers(service)
	handler := handlers[TaskBatchUpdate].(toolruntime.IdempotentMutationHandler)
	base := toolruntime.MutationCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7, PrincipalID: 41},
		ChangeSetID:   "changeset-batch-0002", ApplyKey: "apply-batch-0002",
	}
	base.Input = json.RawMessage(`{"ids":[81],"updates":{"title":"Updated"},"unexpected":true}`)
	if _, err := handler.Apply(context.Background(), base); !errors.Is(err, task.ErrBatchMutationInvalid) {
		t.Fatalf("unknown input err=%v", err)
	}
	base.Input = json.RawMessage(`{"ids":[81],"updates":{"due_at":"not-a-time"}}`)
	if _, err := handler.Apply(context.Background(), base); !errors.Is(err, task.ErrBatchMutationInvalid) {
		t.Fatalf("time input err=%v", err)
	}
	base.Input = json.RawMessage(`{"ids":[81],"updates":{"title":"Updated"}}`)
	service.err = task.ErrBatchMutationConflict
	if _, err := handler.Apply(context.Background(), base); !errors.Is(err, task.ErrBatchMutationConflict) {
		t.Fatalf("business err=%v", err)
	}
	service.err = errors.New("database unavailable")
	if _, err := handler.Apply(context.Background(), base); !errors.Is(err, toolruntime.ErrMutationRetryable) {
		t.Fatalf("retryable err=%v", err)
	}
}

func TestTaskBatchStatusWriteHandlerRejectsNonStatusFields(t *testing.T) {
	service := &taskBatchMutationCapture{}
	handlers, _ := NewTaskBatchWriteHandlers(service)
	handler := handlers[TaskBatchStatusUpdate].(toolruntime.IdempotentMutationHandler)
	call := toolruntime.MutationCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7, PrincipalID: 41},
		ChangeSetID:   "changeset-batch-status", ApplyKey: "apply-batch-status",
		Input: json.RawMessage(`{"ids":[81],"updates":{"status":"in_dev","title":"forbidden"}}`),
	}
	if _, err := handler.Apply(context.Background(), call); !errors.Is(err, task.ErrBatchMutationInvalid) {
		t.Fatalf("non-status batch transition error=%v", err)
	}
}

func TestTaskBatchWritesUseCanonicalApplicationChangeSetsAndHaveCLIMCPParity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(toolruntimegorm.ChangeSetMigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	service := &taskBatchMutationCapture{results: map[task.BatchMutationOperation]task.BatchMutationResult{
		task.BatchCreate: {
			Created:      []task.BatchCreatedTask{{ClientID: "parent", ID: 101, TaskKey: "PM-101", Children: []task.BatchCreatedTask{{ClientID: "child", ID: 102, TaskKey: "PM-102"}}}},
			CreatedCount: 2, Atomic: true,
		},
		task.ChildrenBatchCreate: {
			Created:      []task.BatchCreatedTask{{ClientID: "child", ID: 103, TaskKey: "PM-103"}},
			CreatedCount: 1, ParentID: 101, Atomic: true,
		},
		task.BatchDelete: {
			DeletedIDs: []uint{201}, DeletedCount: 1, AffectedIDs: []uint{201}, Atomic: true,
		},
		task.BatchUpdate: {
			UpdatedIDs: []uint{301}, UpdatedCount: 1, AffectedIDs: []uint{301}, Atomic: true,
		},
	}}
	handlers, err := NewTaskBatchWriteHandlers(service)
	if err != nil {
		t.Fatal(err)
	}
	names := TaskBatchWriteHandlerNames()
	source, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: names})
	if err != nil {
		t.Fatal(err)
	}
	definitions := make([]toolruntime.Definition, 0, len(names))
	for _, name := range names {
		definition, found := source.Find(name)
		if !found || definition.Lifecycle != toolruntime.LifecycleActive {
			t.Fatalf("definition %s found=%v lifecycle=%s", name, found, definition.Lifecycle)
		}
		definitions = append(definitions, definition)
	}
	catalog, err := toolruntime.NewCatalog(definitions)
	if err != nil {
		t.Fatal(err)
	}
	changeSets, err := toolruntime.NewChangeSetService(toolruntimegorm.NewChangeSetRepository(db), toolruntime.DefaultChangeSetPolicy())
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(
		catalog, taskBatchWriteAuthorizer{names: names}, handlers, toolruntime.WithChangeSetService(changeSets),
	)
	if err != nil {
		t.Fatal(err)
	}
	cli := toolruntime.ExecutionAuthorization{
		PrincipalID: 41, WorkspaceID: 7, GrantID: 5, ClientID: "pm-cli",
		Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
	}
	inputs := map[string]json.RawMessage{
		TaskBatchCreate:         json.RawMessage(`{"tasks":[{"client_id":"parent","requirement_id":13,"title":"Parent","children":[{"client_id":"child","title":"Child"}]}]}`),
		TaskChildrenBatchCreate: json.RawMessage(`{"id":101,"children":[{"client_id":"child","title":"Child"}]}`),
		TaskBatchDelete:         json.RawMessage(`{"ids":[201],"cascade":false}`),
		TaskBatchUpdate:         json.RawMessage(`{"ids":[301],"updates":{"title":"Updated"}}`),
		TaskBatchStatusUpdate:   json.RawMessage(`{"ids":[302],"updates":{"status":"in_dev"}}`),
	}
	results := map[string]json.RawMessage{}
	for index, name := range names {
		results[name] = invokeAndConfirmTaskBatchMutation(t, application, name, inputs[name], fmt.Sprintf("task-batch-cli-%02d", index), cli)
	}
	if !bytes.Contains(results[TaskBatchUpdate], []byte(`"auto_completed_ancestor_ids":[]`)) ||
		!bytes.Contains(results[TaskBatchUpdate], []byte(`"due_propagated_descendant_ids":[]`)) {
		t.Fatalf("update output must contain stable empty arrays: %s", results[TaskBatchUpdate])
	}

	mcp := cli
	mcp.ClientID, mcp.Channel, mcp.Audience = "codex", toolruntime.DeliveryMCP, toolruntime.AudienceMCP
	mcpCreate := invokeAndConfirmTaskBatchMutation(t, application, TaskBatchCreate, inputs[TaskBatchCreate], "task-batch-mcp-01", mcp)
	if string(mcpCreate) != string(results[TaskBatchCreate]) {
		t.Fatalf("CLI output=%s MCP output=%s", results[TaskBatchCreate], mcpCreate)
	}
}

func invokeAndConfirmTaskBatchMutation(
	t *testing.T,
	application *toolruntime.Application,
	name string,
	input json.RawMessage,
	key string,
	authorization toolruntime.ExecutionAuthorization,
) json.RawMessage {
	t.Helper()
	pending, err := application.Invoke(context.Background(), toolruntime.Invocation{
		Name: name, Input: input, IdempotencyKey: key, Authorization: authorization,
	})
	if err != nil || pending.ChangeSet == nil || pending.ChangeSet.Status != toolruntime.ChangeSetPending {
		t.Fatalf("%s pending=%+v err=%v", name, pending, err)
	}
	succeeded, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if err != nil || succeeded.Status != toolruntime.ChangeSetSucceeded {
		t.Fatalf("%s succeeded=%+v err=%v", name, succeeded, err)
	}
	replayed, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if err != nil || replayed.Status != toolruntime.ChangeSetSucceeded || string(replayed.Result) != string(succeeded.Result) {
		t.Fatalf("%s replay=%+v err=%v", name, replayed, err)
	}
	return succeeded.Result
}

type taskBatchWriteAuthorizer struct{ names []string }

func (authorizer taskBatchWriteAuthorizer) AuthorizeGrant(context.Context, toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant(authorizer.names, []string{"task:write"})
}

type taskBatchMutationCapture struct {
	commands []task.BatchMutation
	err      error
	results  map[task.BatchMutationOperation]task.BatchMutationResult
}

func (capture *taskBatchMutationCapture) ApplyTaskBatchMutation(_ context.Context, command task.BatchMutation) (task.BatchMutationResult, error) {
	capture.commands = append(capture.commands, command)
	if result, found := capture.results[command.Operation]; found {
		return result, capture.err
	}
	return task.BatchMutationResult{Atomic: true}, capture.err
}
