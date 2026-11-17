package taskgorm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

func TestTestingMutationRepositoryAppliesAllOperationsWithDurableReplayAndCAS(t *testing.T) {
	db := openTestingMutationDB(t)
	workTask := WorkTask{WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, ProjectID: 10, RequirementID: 11, Kind: "task", TaskKey: "TASK-1", Title: "Test", Status: "todo", Priority: "normal"}
	if err := db.Create(&workTask).Error; err != nil {
		t.Fatal(err)
	}
	repository, err := NewTestingMutationRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	service, err := task.NewTestingMutationService(repository)
	if err != nil {
		t.Fatal(err)
	}
	definitionCommand := testingMutationBase(task.TestingDefinitionUpdate, workTask.ID, "definition")
	definitionCommand.Definition = task.TestingDefinitionValues{Cases: []task.TestCase{
		{Key: "case-a", Title: "A", Steps: []task.TestStep{{Action: "run a", ExpectedResult: "ok"}}},
		{Key: "case-b", Title: "B", Steps: []task.TestStep{{Action: "run b"}}},
	}}
	definitionResult, err := service.ApplyTestingMutation(context.Background(), definitionCommand)
	if err != nil || definitionResult.Definition == nil || definitionResult.Definition.Version != 1 {
		t.Fatalf("definition=%+v err=%v", definitionResult, err)
	}
	replayedDefinition, err := service.ApplyTestingMutation(context.Background(), definitionCommand)
	if err != nil || replayedDefinition.Definition == nil || replayedDefinition.Definition.ID != definitionResult.Definition.ID {
		t.Fatalf("replayed definition=%+v err=%v", replayedDefinition, err)
	}

	executionCommand := testingMutationBase(task.TestingExecutionCreate, workTask.ID, "execution")
	executionCommand.Execution = task.TestingExecutionValues{DefinitionVersion: 1, Environment: "uat", CaseKeys: []string{"case-b", "case-a"}}
	executionResult, err := service.ApplyTestingMutation(context.Background(), executionCommand)
	if err != nil || executionResult.Execution == nil || len(executionResult.Runs) != 2 || executionResult.Runs[0].CaseKey != "case-b" {
		t.Fatalf("execution=%+v err=%v", executionResult, err)
	}
	if _, err := service.ApplyTestingMutation(context.Background(), executionCommand); err != nil {
		t.Fatal(err)
	}
	var executionCount, runCount, ledgerCount int64
	_ = db.Model(&TaskTestExecution{}).Count(&executionCount).Error
	_ = db.Model(&TaskTestExecutionRun{}).Count(&runCount).Error
	_ = db.Model(&TestingToolMutationApply{}).Count(&ledgerCount).Error
	if executionCount != 1 || runCount != 2 || ledgerCount != 2 {
		t.Fatalf("execution=%d runs=%d ledgers=%d", executionCount, runCount, ledgerCount)
	}

	firstRun := executionResult.Runs[0]
	runCommand := testingMutationBase(task.TestingExecutionRunUpdate, workTask.ID, "run-1")
	runCommand.Run = task.TestingRunValues{RunID: firstRun.ID, Version: firstRun.Version, Status: "passed", ActualResult: "done", Fields: map[string]bool{"status": true, "actual_result": true}}
	runResult, err := service.ApplyTestingMutation(context.Background(), runCommand)
	if err != nil || runResult.Run == nil || runResult.Run.Version != 2 || runResult.Execution == nil || runResult.Execution.Status != "running" {
		t.Fatalf("run=%+v err=%v", runResult, err)
	}
	secondRun := executionResult.Runs[1]
	secondCommand := testingMutationBase(task.TestingExecutionRunUpdate, workTask.ID, "run-2")
	secondCommand.Run = task.TestingRunValues{RunID: secondRun.ID, Version: secondRun.Version, Status: "skipped", Fields: map[string]bool{"status": true}}
	secondResult, err := service.ApplyTestingMutation(context.Background(), secondCommand)
	if err != nil || secondResult.Execution == nil || secondResult.Execution.Status != "passed" || secondResult.Execution.FinishedAt == nil {
		t.Fatalf("second run=%+v err=%v", secondResult, err)
	}
	if _, err := service.ApplyTestingMutation(context.Background(), runCommand); err != nil {
		t.Fatalf("same-key run replay: %v", err)
	}
	conflict := testingMutationBase(task.TestingExecutionRunUpdate, workTask.ID, "stale")
	conflict.Run = task.TestingRunValues{RunID: firstRun.ID, Version: 1, Status: "failed", FailureReason: "failure", Fields: map[string]bool{"status": true, "failure_reason": true}}
	if _, err := service.ApplyTestingMutation(context.Background(), conflict); !errors.Is(err, task.ErrTestingMutationConflict) {
		t.Fatalf("stale CAS error=%v", err)
	}
}

func TestTestingMutationRepositoryFailsClosedAcrossWorkspaceAndRollsBackLedger(t *testing.T) {
	db := openTestingMutationDB(t)
	row := WorkTask{WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, ProjectID: 10, RequirementID: 11, Kind: "task", TaskKey: "TASK-2", Title: "Test", Status: "todo", Priority: "normal"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	repository, _ := NewTestingMutationRepository(db)
	service, _ := task.NewTestingMutationService(repository)
	foreign := testingMutationBase(task.TestingDefinitionUpdate, row.ID, "foreign")
	foreign.WorkspaceID = 8
	foreign.Definition = task.TestingDefinitionValues{Cases: []task.TestCase{{Key: "one", Title: "One", Steps: []task.TestStep{{Action: "act"}}}}}
	if _, err := service.ApplyTestingMutation(context.Background(), foreign); !errors.Is(err, task.ErrTestingMutationNotFound) {
		t.Fatalf("foreign error=%v", err)
	}
	moved := testingMutationBase(task.TestingDefinitionUpdate, row.ID, "moved")
	moved.TaskScope.RequirementID = 99
	moved.Definition = task.TestingDefinitionValues{Cases: []task.TestCase{{Key: "one", Title: "One", Steps: []task.TestStep{{Action: "act"}}}}}
	if _, err := service.ApplyTestingMutation(context.Background(), moved); !errors.Is(err, task.ErrTestingMutationConflict) {
		t.Fatalf("hierarchy mismatch error=%v", err)
	}
	invalidVersion := testingMutationBase(task.TestingExecutionCreate, row.ID, "invalid-version")
	invalidVersion.Execution = task.TestingExecutionValues{DefinitionVersion: 99, Environment: "uat"}
	if _, err := service.ApplyTestingMutation(context.Background(), invalidVersion); !errors.Is(err, task.ErrTestingMutationConflict) {
		t.Fatalf("version error=%v", err)
	}
	var ledgers, executions int64
	_ = db.Model(&TestingToolMutationApply{}).Count(&ledgers).Error
	_ = db.Model(&TaskTestExecution{}).Count(&executions).Error
	if ledgers != 0 || executions != 0 {
		t.Fatalf("rollback ledgers=%d executions=%d", ledgers, executions)
	}
}

func openTestingMutationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&WorkTask{}, &TaskTestDefinition{}, &TaskTestExecution{}, &TaskTestExecutionRun{}, &TestingToolMutationApply{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func testingMutationBase(operation task.TestingMutationOperation, taskID uint, suffix string) task.TestingMutation {
	return task.TestingMutation{
		Operation: operation, WorkspaceID: 7, PrincipalID: 12, TaskID: taskID,
		TaskScope: task.MutationTaskScope{TeamID: 8, ProjectSetID: 9, PlanUnitID: 10, RequirementID: 11},
		ApplyKey:  "apply-" + suffix, ChangeSetID: "changeset-" + suffix, InputDigest: strings.Repeat("a", 64),
	}
}
