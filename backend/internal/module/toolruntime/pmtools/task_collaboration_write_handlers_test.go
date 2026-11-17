package pmtools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
	"gorm.io/gorm"
)

type taskCollaborationMutationCapture struct {
	command task.CollaborationMutation
	result  task.CollaborationMutationResult
	err     error
}

type taskCollaborationNotificationNoop struct{}

func (taskCollaborationNotificationNoop) RecordTaskComment(context.Context, task.CommentNotification) error {
	return nil
}

func (capture *taskCollaborationMutationCapture) ApplyCollaborationMutation(_ context.Context, command task.CollaborationMutation) (task.CollaborationMutationResult, error) {
	capture.command = command
	return capture.result, capture.err
}

func TestTaskCollaborationWriteHandlersAreStrictActiveAndUseServerPrincipal(t *testing.T) {
	snapshot := task.Snapshot{ID: 9, WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, PlanUnitID: 10, RequirementID: 11, TaskKey: "TASK-9", Title: "Task"}
	watched := true
	comment := task.CommentSnapshot{ID: 31, WorkspaceID: 7, WorkTaskID: 9, UserID: 3, Body: "hello", CreatedAt: time.Now().UTC()}
	tests := []struct {
		name      string
		input     json.RawMessage
		operation task.CollaborationMutationOperation
		result    task.CollaborationMutationResult
	}{
		{TaskCommentCreate, json.RawMessage(`{"id":9,"body":"hello"}`), task.CommentCreate, task.CollaborationMutationResult{Comment: &comment}},
		{TaskCommentDelete, json.RawMessage(`{"id":9,"cid":31}`), task.CommentDelete, task.CollaborationMutationResult{DeletedID: 31, Deleted: true}},
		{TaskAttachmentDelete, json.RawMessage(`{"id":9,"aid":41}`), task.AttachmentDelete, task.CollaborationMutationResult{DeletedID: 41, Deleted: true}},
		{TaskWatchUpdate, json.RawMessage(`{"id":9,"watch":true}`), task.WatchUpdate, task.CollaborationMutationResult{WorkTaskID: 9, Watched: &watched}},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			capture := &taskCollaborationMutationCapture{result: item.result}
			handlers, err := NewTaskCollaborationWriteHandlers(taskTestingQueryStub{snapshot: snapshot}, capture)
			if err != nil {
				t.Fatal(err)
			}
			handler := handlers[item.name]
			mutation, ok := handler.(toolruntime.IdempotentMutationHandler)
			if !ok {
				t.Fatalf("%s is not idempotent", item.name)
			}
			if _, err := handler.Invoke(context.Background(), toolruntime.ToolCall{}); !errors.Is(err, toolruntime.ErrChangeSetRequired) {
				t.Fatalf("Invoke error=%v", err)
			}
			if _, err := mutation.Apply(context.Background(), toolruntime.MutationCall{
				Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7}, Input: item.input,
				ChangeSetID: "changeset-12345678", ApplyKey: "apply-12345678",
			}); err != nil {
				t.Fatal(err)
			}
			if capture.command.Operation != item.operation || capture.command.TaskID != 9 || capture.command.WorkspaceID != 7 ||
				capture.command.PrincipalID != 3 || capture.command.TaskScope.TeamID != 8 || len(capture.command.InputDigest) != 64 {
				t.Fatalf("command=%+v", capture.command)
			}
			catalog, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: []string{item.name}})
			if err != nil {
				t.Fatal(err)
			}
			definition, found := catalog.Find(item.name)
			if !found || definition.Lifecycle != toolruntime.LifecycleActive || definition.ChangeSet != toolruntime.ChangeSetRequired {
				t.Fatalf("definition=%+v found=%v", definition, found)
			}
			for label, raw := range map[string]json.RawMessage{"input": definition.InputSchema, "output": definition.OutputSchema} {
				var schema map[string]any
				if json.Unmarshal(raw, &schema) != nil || schema["additionalProperties"] != false {
					t.Fatalf("%s %s schema is not strict: %s", item.name, label, raw)
				}
			}
		})
	}
	capture := &taskCollaborationMutationCapture{}
	handlers, _ := NewTaskCollaborationWriteHandlers(taskTestingQueryStub{snapshot: snapshot}, capture)
	_, err := handlers[TaskWatchUpdate].(toolruntime.IdempotentMutationHandler).Apply(context.Background(), toolruntime.MutationCall{
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input:         json.RawMessage(`{"id":9,"watch":true,"principal_id":99}`), ChangeSetID: "changeset-12345678", ApplyKey: "apply-12345678",
	})
	if !errors.Is(err, task.ErrCollaborationMutationInvalid) {
		t.Fatalf("client-selected principal error=%v", err)
	}
	capture.err = errors.New("storage unavailable")
	_, err = handlers[TaskAttachmentDelete].(toolruntime.IdempotentMutationHandler).Apply(context.Background(), toolruntime.MutationCall{
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input:         json.RawMessage(`{"id":9,"aid":41}`), ChangeSetID: "changeset-storage", ApplyKey: "apply-storage",
	})
	if !errors.Is(err, toolruntime.ErrMutationRetryable) {
		t.Fatalf("storage error must be retryable: %v", err)
	}
}

type taskCollaborationWriteAuthorizer struct{ names []string }

func (authorizer taskCollaborationWriteAuthorizer) AuthorizeGrant(context.Context, toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant(authorizer.names, []string{"task:write"})
}

func TestTaskCollaborationWritesShareOneApplicationForCLIAndMCP(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	records := []any{&taskgorm.WorkTask{}, &taskgorm.WorkTaskComment{}, &taskgorm.WorkTaskActivity{}, &taskgorm.WorkTaskWatcher{}, &taskgorm.WorkTaskAttachment{}}
	records = append(records, taskgorm.CollaborationMutationMigrationRecords()...)
	records = append(records, toolruntimegorm.ChangeSetMigrationRecords()...)
	if err := db.AutoMigrate(records...); err != nil {
		t.Fatal(err)
	}
	workTask := taskgorm.WorkTask{WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, ProjectID: 10, RequirementID: 11, Kind: "task", TaskKey: "TASK-1", Title: "Task", Status: "todo", Priority: "normal"}
	if err := db.Create(&workTask).Error; err != nil {
		t.Fatal(err)
	}
	repository, _ := taskgorm.NewCollaborationMutationRepository(db, func(*gorm.DB) (task.CollaborationNotificationWriter, error) {
		return taskCollaborationNotificationNoop{}, nil
	})
	service, _ := task.NewCollaborationMutationService(repository, &collaborationObjectRemover{})
	names := TaskCollaborationWriteHandlerNames()
	handlers, err := NewTaskCollaborationWriteHandlers(taskTestingQueryStub{snapshot: task.Snapshot{
		ID: workTask.ID, WorkspaceID: 7, TeamID: workTask.TeamID, ProjectSetID: workTask.ProjectSetID,
		PlanUnitID: workTask.ProjectID, RequirementID: workTask.RequirementID, TaskKey: workTask.TaskKey, Title: workTask.Title,
	}}, service)
	if err != nil {
		t.Fatal(err)
	}
	source, _ := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: names})
	definitions := make([]toolruntime.Definition, 0, len(names))
	for _, name := range names {
		definition, found := source.Find(name)
		if !found {
			t.Fatalf("missing %s", name)
		}
		definitions = append(definitions, definition)
	}
	catalog, _ := toolruntime.NewCatalog(definitions)
	changeSets, _ := toolruntime.NewChangeSetService(toolruntimegorm.NewChangeSetRepository(db), toolruntime.DefaultChangeSetPolicy())
	application, err := toolruntime.NewApplication(catalog, taskCollaborationWriteAuthorizer{names: names}, handlers, toolruntime.WithChangeSetService(changeSets))
	if err != nil {
		t.Fatal(err)
	}
	cli := toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7, GrantID: 5, ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI}
	mcp := cli
	mcp.ClientID, mcp.Channel, mcp.Audience = "codex", toolruntime.DeliveryMCP, toolruntime.AudienceMCP
	cliResult := invokeAndConfirmCollaborationMutation(t, application, TaskCommentCreate, json.RawMessage(`{"id":1,"body":"same"}`), "comment-cli-0001", cli)
	mcpResult := invokeAndConfirmCollaborationMutation(t, application, TaskCommentCreate, json.RawMessage(`{"id":1,"body":"same"}`), "comment-mcp-0001", mcp)
	var cliComment, mcpComment task.CommentSnapshot
	if json.Unmarshal(cliResult, &cliComment) != nil || json.Unmarshal(mcpResult, &mcpComment) != nil ||
		cliComment.Body != mcpComment.Body || cliComment.UserID != mcpComment.UserID || cliComment.WorkTaskID != mcpComment.WorkTaskID {
		t.Fatalf("CLI=%s MCP=%s", cliResult, mcpResult)
	}
	var comments, ledgers int64
	_ = db.Model(&taskgorm.WorkTaskComment{}).Count(&comments).Error
	_ = db.Model(&taskgorm.TaskToolMutationApply{}).Count(&ledgers).Error
	if comments != 2 || ledgers != 2 {
		t.Fatalf("comments=%d ledgers=%d", comments, ledgers)
	}
}

type collaborationObjectRemover struct{}

func (*collaborationObjectRemover) RemoveObject(context.Context, string) error { return nil }

type recoveringCollaborationObjectRemover struct {
	err   error
	calls int
}

func (remover *recoveringCollaborationObjectRemover) RemoveObject(context.Context, string) error {
	remover.calls++
	return remover.err
}

func TestAttachmentDeleteChangeSetResumesAfterObjectStorageRecovery(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	records := []any{&taskgorm.WorkTask{}, &taskgorm.WorkTaskActivity{}, &taskgorm.WorkTaskAttachment{}}
	records = append(records, taskgorm.CollaborationMutationMigrationRecords()...)
	records = append(records, toolruntimegorm.ChangeSetMigrationRecords()...)
	if err := db.AutoMigrate(records...); err != nil {
		t.Fatal(err)
	}
	workTask := taskgorm.WorkTask{WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, ProjectID: 10, RequirementID: 11, Kind: "task", TaskKey: "TASK-1", Title: "Task", Status: "todo", Priority: "normal"}
	if err := db.Create(&workTask).Error; err != nil {
		t.Fatal(err)
	}
	attachment := taskgorm.WorkTaskAttachment{WorkspaceID: 7, WorkTaskID: workTask.ID, UploaderUserID: 3, Filename: "report.txt", Size: 4, StorageBackend: "s3", ObjectKey: "workspace/7/tasks/1/report.txt", MutationState: "active"}
	if err := db.Create(&attachment).Error; err != nil {
		t.Fatal(err)
	}
	repository, _ := taskgorm.NewCollaborationMutationRepository(db, func(*gorm.DB) (task.CollaborationNotificationWriter, error) {
		return taskCollaborationNotificationNoop{}, nil
	})
	remover := &recoveringCollaborationObjectRemover{err: errors.New("object storage disabled")}
	service, _ := task.NewCollaborationMutationService(repository, remover)
	handlers, _ := NewTaskCollaborationWriteHandlers(taskTestingQueryStub{snapshot: task.Snapshot{
		ID: workTask.ID, WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, PlanUnitID: 10, RequirementID: 11, TaskKey: workTask.TaskKey,
	}}, service)
	handlers = map[string]toolruntime.ToolHandler{TaskAttachmentDelete: handlers[TaskAttachmentDelete]}
	source, _ := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: []string{TaskAttachmentDelete}})
	definition, _ := source.Find(TaskAttachmentDelete)
	catalog, _ := toolruntime.NewCatalog([]toolruntime.Definition{definition})
	changeSets, _ := toolruntime.NewChangeSetService(toolruntimegorm.NewChangeSetRepository(db), toolruntime.DefaultChangeSetPolicy())
	application, err := toolruntime.NewApplication(catalog, taskCollaborationWriteAuthorizer{names: []string{TaskAttachmentDelete}}, handlers, toolruntime.WithChangeSetService(changeSets))
	if err != nil {
		t.Fatal(err)
	}
	authorization := toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7, GrantID: 5, ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI}
	pending, err := application.Invoke(context.Background(), toolruntime.Invocation{
		Name: TaskAttachmentDelete, Input: json.RawMessage(`{"id":1,"aid":1}`), IdempotencyKey: "attachment-recovery-0001", Authorization: authorization,
	})
	if err != nil || pending.ChangeSet == nil {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	released, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if !errors.Is(err, toolruntime.ErrChangeSetInProgress) || released.Status == toolruntime.ChangeSetFailed {
		t.Fatalf("released=%+v err=%v", released, err)
	}
	var metadata taskgorm.WorkTaskAttachment
	if err := db.First(&metadata, attachment.ID).Error; err != nil || metadata.MutationState != "deleting" {
		t.Fatalf("metadata=%+v err=%v", metadata, err)
	}
	remover.err = nil
	succeeded, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if err != nil || succeeded.Status != toolruntime.ChangeSetSucceeded {
		t.Fatalf("succeeded=%+v err=%v", succeeded, err)
	}
	replayed, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if err != nil || string(replayed.Result) != string(succeeded.Result) || remover.calls != 2 {
		t.Fatalf("replayed=%+v remove_calls=%d err=%v", replayed, remover.calls, err)
	}
	var remaining int64
	_ = db.Model(&taskgorm.WorkTaskAttachment{}).Where("id = ?", attachment.ID).Count(&remaining).Error
	if remaining != 0 {
		t.Fatal("attachment metadata remains after storage recovered")
	}
}

func invokeAndConfirmCollaborationMutation(t *testing.T, application *toolruntime.Application, name string, input json.RawMessage, key string, authorization toolruntime.ExecutionAuthorization) json.RawMessage {
	t.Helper()
	pending, err := application.Invoke(context.Background(), toolruntime.Invocation{Name: name, Input: input, IdempotencyKey: key, Authorization: authorization})
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
