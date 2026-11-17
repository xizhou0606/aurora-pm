package pmtools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
	"gorm.io/gorm"
)

type taskTestingQueryStub struct{ snapshot task.Snapshot }

func (stub taskTestingQueryStub) GetTask(_ context.Context, workspaceID uint, lookup task.Lookup) (task.Snapshot, error) {
	if workspaceID != stub.snapshot.WorkspaceID || (lookup.ID != stub.snapshot.ID && lookup.Reference != stub.snapshot.TaskKey) {
		return task.Snapshot{}, task.ErrTaskNotFound
	}
	return stub.snapshot, nil
}

type taskTestingMutationCapture struct {
	command task.TestingMutation
	result  task.TestingMutationResult
}

func (capture *taskTestingMutationCapture) ApplyTestingMutation(_ context.Context, command task.TestingMutation) (task.TestingMutationResult, error) {
	capture.command = command
	return capture.result, nil
}

func TestTaskTestingWriteHandlersAreStrictActiveAndMapAllThreeOperations(t *testing.T) {
	snapshot := task.Snapshot{ID: 9, WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, PlanUnitID: 10, RequirementID: 11, TaskKey: "TASK-9", Title: "Task"}
	definition := task.TestDefinitionSnapshot{ID: 1, WorkspaceID: 7, WorkTaskID: 9, Version: 1}
	execution := task.TestExecutionSnapshot{ID: 2, WorkspaceID: 7, WorkTaskID: 9, TaskTestDefinitionID: 1, DefinitionVersion: 1, Environment: "uat", ExecutorUserID: 3, Status: "running", RunCounts: map[string]int64{}}
	run := task.TestExecutionRunSnapshot{ID: 3, WorkspaceID: 7, WorkTaskID: 9, TaskTestExecutionID: 2, Version: 1, Sequence: 1, CaseKey: "case", Title: "Case", Steps: []task.TestStep{{Action: "run"}}, StepResults: []task.TestStepResult{}, Status: "pending", Evidence: []task.TestEvidence{}}
	tests := []struct {
		name      string
		input     json.RawMessage
		operation task.TestingMutationOperation
		result    task.TestingMutationResult
	}{
		{TaskTestingDefinitionUpdate, json.RawMessage(`{"task_ref":"TASK-9","cases":[{"key":"case","title":"Case","steps":[{"action":"run"}]}]}`), task.TestingDefinitionUpdate, task.TestingMutationResult{Definition: &definition}},
		{TaskTestingExecutionCreate, json.RawMessage(`{"id":9,"definition_version":1,"environment":"uat"}`), task.TestingExecutionCreate, task.TestingMutationResult{Execution: &execution, Runs: []task.TestExecutionRunSnapshot{run}}},
		{TaskTestingExecutionRunUpdate, json.RawMessage(`{"id":9,"run_id":3,"version":1,"status":"passed"}`), task.TestingExecutionRunUpdate, task.TestingMutationResult{Execution: &execution, Run: &run}},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			capture := &taskTestingMutationCapture{result: item.result}
			handlers, err := NewTaskTestingWriteHandlers(taskTestingQueryStub{snapshot: snapshot}, capture)
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
				capture.command.PrincipalID != 3 || len(capture.command.InputDigest) != 64 {
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
	capture := &taskTestingMutationCapture{}
	handlers, _ := NewTaskTestingWriteHandlers(taskTestingQueryStub{snapshot: snapshot}, capture)
	_, err := handlers[TaskTestingExecutionCreate].(toolruntime.IdempotentMutationHandler).Apply(context.Background(), toolruntime.MutationCall{
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input:         json.RawMessage(`{"id":9,"definition_version":1,"environment":"uat","unknown":true}`),
		ChangeSetID:   "changeset-12345678", ApplyKey: "apply-12345678",
	})
	if !errors.Is(err, task.ErrTestingMutationInvalid) {
		t.Fatalf("unknown field error=%v", err)
	}
}

type taskTestingWriteAuthorizer struct{ names []string }

func (authorizer taskTestingWriteAuthorizer) AuthorizeGrant(context.Context, toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant(authorizer.names, []string{"task:write"})
}

func TestTaskTestingWritesRunThroughOneApplicationForCLIAndMCPWithReplay(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	records := []any{&taskgorm.WorkTask{}, &taskgorm.TaskTestDefinition{}, &taskgorm.TaskTestExecution{}, &taskgorm.TaskTestExecutionRun{}, &taskgorm.TestingToolMutationApply{}}
	records = append(records, toolruntimegorm.ChangeSetMigrationRecords()...)
	if err := db.AutoMigrate(records...); err != nil {
		t.Fatal(err)
	}
	workTask := taskgorm.WorkTask{WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, ProjectID: 10, RequirementID: 11, Kind: "task", TaskKey: "TASK-1", Title: "Task", Status: "todo", Priority: "normal"}
	if err := db.Create(&workTask).Error; err != nil {
		t.Fatal(err)
	}
	repository, _ := taskgorm.NewTestingMutationRepository(db)
	service, _ := task.NewTestingMutationService(repository)
	names := TaskTestingWriteHandlerNames()
	handlers, err := NewTaskTestingWriteHandlers(taskTestingQueryStub{snapshot: task.Snapshot{
		ID: workTask.ID, WorkspaceID: 7, TeamID: workTask.TeamID, ProjectSetID: workTask.ProjectSetID,
		PlanUnitID: workTask.ProjectID, RequirementID: workTask.RequirementID, TaskKey: workTask.TaskKey, Title: workTask.Title,
	}}, service)
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
	catalog, _ := toolruntime.NewCatalog(definitions)
	changeSets, _ := toolruntime.NewChangeSetService(toolruntimegorm.NewChangeSetRepository(db), toolruntime.DefaultChangeSetPolicy())
	application, err := toolruntime.NewApplication(catalog, taskTestingWriteAuthorizer{names: names}, handlers, toolruntime.WithChangeSetService(changeSets))
	if err != nil {
		t.Fatal(err)
	}
	cli := toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7, GrantID: 5, ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI}
	mcp := cli
	mcp.ClientID, mcp.Channel, mcp.Audience = "codex", toolruntime.DeliveryMCP, toolruntime.AudienceMCP
	definitionResult := invokeAndConfirmTestingMutation(t, application, TaskTestingDefinitionUpdate,
		json.RawMessage(`{"task_ref":"TASK-1","cases":[{"key":"case","title":"Case","steps":[{"action":"run"}]}]}`), "definition-0001", cli)
	var definition task.TestDefinitionSnapshot
	if json.Unmarshal(definitionResult, &definition) != nil || definition.Version != 1 {
		t.Fatalf("definition=%s", definitionResult)
	}
	executionResult := invokeAndConfirmTestingMutation(t, application, TaskTestingExecutionCreate,
		json.RawMessage(`{"id":1,"definition_version":1,"environment":"uat"}`), "execution-0001", mcp)
	var created struct {
		Execution task.TestExecutionSnapshot      `json:"execution"`
		Runs      []task.TestExecutionRunSnapshot `json:"runs"`
	}
	if json.Unmarshal(executionResult, &created) != nil || len(created.Runs) != 1 || created.Execution.Status != "running" {
		t.Fatalf("execution=%s", executionResult)
	}
	cliExecutionResult := invokeAndConfirmTestingMutation(t, application, TaskTestingExecutionCreate,
		json.RawMessage(`{"task_ref":"TASK-1","definition_version":1,"environment":"uat"}`), "execution-cli-0001", cli)
	var cliCreated struct {
		Execution task.TestExecutionSnapshot      `json:"execution"`
		Runs      []task.TestExecutionRunSnapshot `json:"runs"`
	}
	if json.Unmarshal(cliExecutionResult, &cliCreated) != nil || len(cliCreated.Runs) != 1 ||
		cliCreated.Execution.DefinitionVersion != created.Execution.DefinitionVersion ||
		cliCreated.Execution.Environment != created.Execution.Environment || cliCreated.Runs[0].CaseKey != created.Runs[0].CaseKey {
		t.Fatalf("CLI execution=%s MCP execution=%s", cliExecutionResult, executionResult)
	}
	runInput, _ := json.Marshal(map[string]any{"id": workTask.ID, "run_id": created.Runs[0].ID, "version": 1, "status": "passed"})
	runResult := invokeAndConfirmTestingMutation(t, application, TaskTestingExecutionRunUpdate, runInput, "run-0001", cli)
	var updated struct {
		Run       task.TestExecutionRunSnapshot `json:"run"`
		Execution task.TestExecutionSnapshot    `json:"execution"`
	}
	if json.Unmarshal(runResult, &updated) != nil || updated.Run.Version != 2 || updated.Execution.Status != "passed" {
		t.Fatalf("run=%s", runResult)
	}
	var executions, runs, ledgers int64
	_ = db.Model(&taskgorm.TaskTestExecution{}).Count(&executions).Error
	_ = db.Model(&taskgorm.TaskTestExecutionRun{}).Count(&runs).Error
	_ = db.Model(&taskgorm.TestingToolMutationApply{}).Count(&ledgers).Error
	if executions != 2 || runs != 2 || ledgers != 4 {
		t.Fatalf("executions=%d runs=%d ledgers=%d", executions, runs, ledgers)
	}
}

func invokeAndConfirmTestingMutation(t *testing.T, application *toolruntime.Application, name string, input json.RawMessage, key string, authorization toolruntime.ExecutionAuthorization) json.RawMessage {
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
