package taskgorm

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestTestingReadRepositoryUsesExactWorkspaceTaskAndExecutionScope(t *testing.T) {
	db := testingReadDB(t)
	now := time.Date(2026, 8, 27, 6, 0, 0, 0, time.UTC)
	taskOne := WorkTask{ID: 101, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: WorkTaskKindTask, TaskKey: "A-101", Title: "Task A"}
	taskTwo := WorkTask{ID: 201, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, ProjectID: 22, RequirementID: 23, Kind: WorkTaskKindTask, TaskKey: "B-201", Title: "Task B"}
	if err := db.Create(&[]WorkTask{taskOne, taskTwo}).Error; err != nil {
		t.Fatal(err)
	}
	definition := TaskTestDefinition{
		ID: 301, WorkspaceID: 1, WorkTaskID: 101, Version: 2, Purpose: "验证",
		CasesJSON:       `[{"key":"TC-1","title":"主路径","steps":[{"action":"打开"}]}]`,
		CreatedByUserID: 7, UpdatedByUserID: 8, CreatedAt: now, UpdatedAt: now,
	}
	execution := TaskTestExecution{
		ID: 401, WorkspaceID: 1, WorkTaskID: 101, TaskTestDefinitionID: 301, DefinitionVersion: 2,
		Environment: "uat", ExecutorUserID: 7, Status: "running", CreatedAt: now, UpdatedAt: now,
	}
	runs := []TaskTestExecutionRun{
		{ID: 501, WorkspaceID: 1, WorkTaskID: 101, TaskTestExecutionID: 401, Version: 1, Sequence: 1,
			CaseKey: "TC-1", Title: "主路径", StepsJSON: `[{"action":"打开"}]`, StepResultsJSON: `[]`, EvidenceJSON: `[]`, Status: "passed", CreatedAt: now, UpdatedAt: now},
		{ID: 502, WorkspaceID: 1, WorkTaskID: 101, TaskTestExecutionID: 401, Version: 1, Sequence: 2,
			CaseKey: "TC-2", Title: "异常路径", StepsJSON: `[{"action":"提交"}]`, StepResultsJSON: `[]`, EvidenceJSON: `[]`, Status: "failed", CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&definition).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&execution).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&runs).Error; err != nil {
		t.Fatal(err)
	}

	repository := NewTestingReadRepository(db)
	gotDefinition, err := repository.GetTestDefinition(context.Background(), 1, 101)
	if err != nil || gotDefinition.Version != 2 || len(gotDefinition.Cases) != 1 {
		t.Fatalf("definition=%+v err=%v", gotDefinition, err)
	}
	if _, err := repository.GetTestDefinition(context.Background(), 2, 101); !errors.Is(err, task.ErrTestDefinitionNotFound) {
		t.Fatalf("cross-workspace definition err=%v", err)
	}
	page, err := repository.ListTestExecutions(context.Background(), 1, 101, 50, 0)
	if err != nil || page.Total != 1 || page.Items[0].RunCounts["total"] != 2 || page.Items[0].RunCounts["failed"] != 1 {
		t.Fatalf("executions=%+v err=%v", page, err)
	}
	if _, err := repository.GetTestExecution(context.Background(), 2, 101, 401); !errors.Is(err, task.ErrTestExecutionNotFound) {
		t.Fatalf("cross-workspace execution err=%v", err)
	}
	runPage, err := repository.ListTestExecutionRuns(context.Background(), 1, 101, 401, task.TestRunFilter{Status: "passed", Limit: 10})
	if err != nil || runPage.Total != 1 || len(runPage.Items) != 1 || runPage.Items[0].ID != 501 {
		t.Fatalf("runs=%+v err=%v", runPage, err)
	}
}

func TestTestingReadRepositoryFailsClosedForMalformedStoredJSON(t *testing.T) {
	db := testingReadDB(t)
	now := time.Now().UTC()
	if err := db.Create(&WorkTask{ID: 101, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: WorkTaskKindTask, TaskKey: "A-101", Title: "Task A"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&TaskTestDefinition{
		ID: 301, WorkspaceID: 1, WorkTaskID: 101, Version: 1, CasesJSON: `{"not":"cases"}`,
		CreatedByUserID: 7, UpdatedByUserID: 7, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := NewTestingReadRepository(db).GetTestDefinition(context.Background(), 1, 101); !errors.Is(err, task.ErrTestingDataInvalid) {
		t.Fatalf("malformed definition err=%v", err)
	}
}

func testingReadDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&WorkTask{}, &TaskTestDefinition{}, &TaskTestExecution{}, &TaskTestExecutionRun{}); err != nil {
		t.Fatal(err)
	}
	return db
}
