package pmtools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
	"gorm.io/gorm"
)

type taskCoreMutationCapture struct {
	command task.CoreMutation
	result  task.CoreMutationResult
	err     error
	calls   int
}

func (capture *taskCoreMutationCapture) ApplyCoreMutation(_ context.Context, command task.CoreMutation) (task.CoreMutationResult, error) {
	capture.calls++
	capture.command = command
	return capture.result, capture.err
}

type taskDeletionCapture struct {
	command task.TaskDeletionCommand
	result  task.TaskDeletionResult
	err     error
	calls   int
}

func (capture *taskDeletionCapture) DeleteTasks(_ context.Context, command task.TaskDeletionCommand) (task.TaskDeletionResult, error) {
	capture.calls++
	capture.command = command
	return capture.result, capture.err
}

func TestTaskCoreWriteHandlersMapOperationsStrictly(t *testing.T) {
	now := time.Now().UTC()
	snapshot := task.Snapshot{ID: 9, WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, PlanUnitID: 10, RequirementID: 11, TaskKey: "TASK-9", Title: "Task", UpdatedAt: now}
	created := snapshot
	created.ID = 12
	reference := task.Reference{ID: 9, TaskKey: "TASK-9", Title: "Task"}
	tests := []struct {
		name      string
		input     json.RawMessage
		operation task.CoreMutationOperation
	}{
		{TaskCreate, json.RawMessage(`{"requirement_id":11,"title":"New"}`), task.CoreCreate},
		{TaskUpdate, json.RawMessage(`{"id":9,"title":"Updated"}`), task.CoreUpdate},
		{TaskStatusUpdate, json.RawMessage(`{"id":9,"status":"in_dev"}`), task.CoreUpdate},
		{TaskClone, json.RawMessage(`{"id":9,"title_suffix":"-copy"}`), task.CoreClone},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			core := &taskCoreMutationCapture{result: task.CoreMutationResult{Task: &created, ClonedFrom: &reference}}
			deletion := &taskDeletionCapture{result: task.TaskDeletionResult{Deleted: true, DeletedCount: 1, DeletedIDs: []uint{9}}}
			handlers, err := NewTaskCoreWriteHandlers(taskTestingQueryStub{snapshot: snapshot}, core, deletion)
			if err != nil {
				t.Fatal(err)
			}
			handler := handlers[item.name]
			mutation, ok := handler.(toolruntime.IdempotentMutationHandler)
			if !ok {
				t.Fatal("handler is not idempotent")
			}
			if _, err := handler.Invoke(context.Background(), toolruntime.ToolCall{}); !errors.Is(err, toolruntime.ErrChangeSetRequired) {
				t.Fatalf("Invoke error=%v", err)
			}
			if _, err := mutation.Apply(context.Background(), toolruntime.MutationCall{Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7}, Input: item.input, ChangeSetID: "changeset-core", ApplyKey: "apply-core"}); err != nil {
				t.Fatal(err)
			}
			if core.command.Operation != item.operation || core.command.WorkspaceID != 7 || core.command.PrincipalID != 3 || len(core.command.InputDigest) != 64 {
				t.Fatalf("command=%+v", core.command)
			}
			if item.name != TaskCreate && (core.command.ID != 9 || !core.command.ExpectedUpdatedAt.Equal(now)) {
				t.Fatalf("CAS command=%+v", core.command)
			}
			if item.name == TaskClone && (!core.command.TitleSuffixSet || core.command.TitleSuffix != "-copy") {
				t.Fatalf("clone suffix command=%+v", core.command)
			}
		})
	}
	core := &taskCoreMutationCapture{result: task.CoreMutationResult{Task: &created}}
	deletion := &taskDeletionCapture{result: task.TaskDeletionResult{Deleted: true, DeletedCount: 1, DeletedIDs: []uint{9}}}
	handlers, _ := NewTaskCoreWriteHandlers(taskTestingQueryStub{snapshot: snapshot}, core, deletion)
	if _, err := handlers[TaskDelete].(toolruntime.IdempotentMutationHandler).Apply(context.Background(), toolruntime.MutationCall{Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7}, Input: json.RawMessage(`{"id":9,"cascade":true}`), ChangeSetID: "changeset-delete", ApplyKey: "apply-delete"}); err != nil {
		t.Fatal(err)
	}
	if len(deletion.command.RootIDs) != 1 || deletion.command.RootIDs[0] != 9 || !deletion.command.Cascade || deletion.command.PrincipalID != 3 {
		t.Fatalf("deletion=%+v", deletion.command)
	}
	_, err := handlers[TaskUpdate].(toolruntime.IdempotentMutationHandler).Apply(context.Background(), toolruntime.MutationCall{Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7}, Input: json.RawMessage(`{"id":9,"title":"Updated","unknown":true}`), ChangeSetID: "changeset-unknown", ApplyKey: "apply-unknown"})
	if !errors.Is(err, task.ErrCoreMutationInvalid) {
		t.Fatalf("unknown field error=%v", err)
	}
	_, err = handlers[TaskUpdate].(toolruntime.IdempotentMutationHandler).Apply(context.Background(), toolruntime.MutationCall{Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7}, Input: json.RawMessage(`{"id":9,"status":null}`), ChangeSetID: "changeset-null", ApplyKey: "apply-null"})
	if !errors.Is(err, task.ErrCoreMutationInvalid) {
		t.Fatalf("non-nullable field error=%v", err)
	}
	_, err = handlers[TaskUpdate].(toolruntime.IdempotentMutationHandler).Apply(context.Background(), toolruntime.MutationCall{Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7}, Input: json.RawMessage(`{"id":9,"due_at":""}`), ChangeSetID: "changeset-empty-time", ApplyKey: "apply-empty-time"})
	if !errors.Is(err, task.ErrCoreMutationInvalid) {
		t.Fatalf("empty date-time error=%v", err)
	}
}

func TestTaskCoreWriteHandlerMapsInfrastructureFailuresRetryable(t *testing.T) {
	snapshot := task.Snapshot{ID: 9, WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, PlanUnitID: 10, RequirementID: 11, TaskKey: "TASK-9", Title: "Task", UpdatedAt: time.Now().UTC()}
	core := &taskCoreMutationCapture{err: errors.New("database unavailable")}
	deletion := &taskDeletionCapture{err: errors.New("object storage unavailable")}
	handlers, _ := NewTaskCoreWriteHandlers(taskTestingQueryStub{snapshot: snapshot}, core, deletion)
	_, err := handlers[TaskUpdate].(toolruntime.IdempotentMutationHandler).Apply(context.Background(), toolruntime.MutationCall{Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7}, Input: json.RawMessage(`{"id":9,"title":"Updated"}`), ChangeSetID: "changeset-update", ApplyKey: "apply-update"})
	if !errors.Is(err, toolruntime.ErrMutationRetryable) {
		t.Fatalf("update error=%v", err)
	}
	_, err = handlers[TaskDelete].(toolruntime.IdempotentMutationHandler).Apply(context.Background(), toolruntime.MutationCall{Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7}, Input: json.RawMessage(`{"id":9,"cascade":true}`), ChangeSetID: "changeset-delete", ApplyKey: "apply-delete"})
	if !errors.Is(err, toolruntime.ErrMutationRetryable) {
		t.Fatalf("delete error=%v", err)
	}
}

func TestTaskCoreWriteOutputsPassCanonicalApplicationSchemas(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(toolruntimegorm.ChangeSetMigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	snapshot := task.Snapshot{
		ID: 9, WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, PlanUnitID: 10, RequirementID: 11,
		Kind: task.WorkTaskKindTask, TaskKey: "TASK-9", Title: "Task", Status: "draft", Priority: "normal",
		Children: []task.Reference{}, CreatedAt: now, UpdatedAt: now,
	}
	reference := task.Reference{ID: 8, TaskKey: "TASK-8", Title: "Source"}
	core := &taskCoreMutationCapture{result: task.CoreMutationResult{Task: &snapshot, ClonedFrom: &reference}}
	deletion := &taskDeletionCapture{result: task.TaskDeletionResult{Deleted: true, DeletedCount: 1, DeletedIDs: []uint{9}}}
	names := TaskCoreWriteHandlerNames()
	handlers, err := NewTaskCoreWriteHandlers(taskTestingQueryStub{snapshot: snapshot}, core, deletion)
	if err != nil {
		t.Fatal(err)
	}
	source, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: names})
	if err != nil {
		t.Fatal(err)
	}
	definitions := make([]toolruntime.Definition, 0, len(names))
	for _, name := range names {
		definition, found := source.Find(name)
		if !found {
			t.Fatalf("missing %s", name)
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
	application, err := toolruntime.NewApplication(catalog, taskTestingWriteAuthorizer{names: names}, handlers, toolruntime.WithChangeSetService(changeSets))
	if err != nil {
		t.Fatal(err)
	}
	cli := toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7, GrantID: 5, ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI}
	mcp := cli
	mcp.ClientID, mcp.Channel, mcp.Audience = "codex", toolruntime.DeliveryMCP, toolruntime.AudienceMCP
	inputs := map[string]json.RawMessage{
		TaskCreate:       json.RawMessage(`{"requirement_id":11,"title":"Task"}`),
		TaskUpdate:       json.RawMessage(`{"id":9,"title":"Task"}`),
		TaskStatusUpdate: json.RawMessage(`{"id":9,"status":"in_dev"}`),
		TaskDelete:       json.RawMessage(`{"id":9,"cascade":false}`),
		TaskClone:        json.RawMessage(`{"id":9}`),
	}
	for index, name := range names {
		beforeCore, beforeDelete := core.calls, deletion.calls
		cliResult := invokeAndConfirmCoreMutation(t, application, name, inputs[name], fmt.Sprintf("core-cli-%02d", index), cli)
		mcpResult := invokeAndConfirmCoreMutation(t, application, name, inputs[name], fmt.Sprintf("core-mcp-%02d", index), mcp)
		if !bytes.Equal(cliResult, mcpResult) {
			t.Fatalf("%s CLI=%s MCP=%s", name, cliResult, mcpResult)
		}
		if name == TaskDelete {
			if deletion.calls-beforeDelete != 2 || core.calls != beforeCore {
				t.Fatalf("%s core calls=%d delete calls=%d", name, core.calls-beforeCore, deletion.calls-beforeDelete)
			}
		} else if core.calls-beforeCore != 2 || deletion.calls != beforeDelete {
			t.Fatalf("%s core calls=%d delete calls=%d", name, core.calls-beforeCore, deletion.calls-beforeDelete)
		}
		if name == TaskCreate && !bytes.Contains(cliResult, []byte(`"children":[]`)) {
			t.Fatalf("task.create children must be an array: %s", cliResult)
		}
	}
}

func invokeAndConfirmCoreMutation(
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
	if err != nil || replayed.Status != toolruntime.ChangeSetSucceeded || !bytes.Equal(replayed.Result, succeeded.Result) {
		t.Fatalf("%s replay=%+v err=%v", name, replayed, err)
	}
	return succeeded.Result
}

var _ TaskCoreMutationService = (*taskCoreMutationCapture)(nil)
var _ task.TaskDeletionExecutor = (*taskDeletionCapture)(nil)
