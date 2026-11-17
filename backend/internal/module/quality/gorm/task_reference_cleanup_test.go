package qualitygorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"gorm.io/gorm"
)

func TestTaskReferenceCleanupIsWorkspaceScopedAndClearsFrozenFacts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&QualityBug{}); err != nil {
		t.Fatal(err)
	}
	taskID, executionID := uint(101), uint(201)
	rows := []QualityBug{
		{WorkspaceID: 7, TeamID: 1, ProjectSetID: 1, ProjectID: 1, RequirementID: 1, BugKey: "B-1", Title: "same", Status: "triage", Priority: "normal", SourceTestID: &taskID, SourceExecutionID: &executionID},
		{WorkspaceID: 8, TeamID: 1, ProjectSetID: 1, ProjectID: 1, RequirementID: 1, BugKey: "B-2", Title: "foreign", Status: "triage", Priority: "normal", SourceTestID: &taskID, SourceExecutionID: &executionID},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	service, err := quality.NewTaskReferenceCleanupService(NewTaskReferenceCleanupRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ClearTaskReferences(context.Background(), quality.TaskReferenceCleanupFacts{
		WorkspaceID: 7, TaskIDs: []uint{taskID}, TestExecutionIDs: []uint{executionID},
	}); err != nil {
		t.Fatal(err)
	}
	var same, foreign QualityBug
	if err := db.First(&same, rows[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&foreign, rows[1].ID).Error; err != nil {
		t.Fatal(err)
	}
	if same.SourceTestID != nil || same.SourceExecutionID != nil {
		t.Fatalf("same workspace refs=%v/%v", same.SourceTestID, same.SourceExecutionID)
	}
	if foreign.SourceTestID == nil || foreign.SourceExecutionID == nil {
		t.Fatalf("foreign workspace refs=%v/%v", foreign.SourceTestID, foreign.SourceExecutionID)
	}
}

func TestTaskReferenceCleanupRejectsInvalidFacts(t *testing.T) {
	service, err := quality.NewTaskReferenceCleanupService(&TaskReferenceCleanupRepository{db: &gorm.DB{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ClearTaskReferences(context.Background(), quality.TaskReferenceCleanupFacts{WorkspaceID: 7, TaskIDs: []uint{1, 1}}); err != quality.ErrTaskReferenceCleanupInvalid {
		t.Fatalf("err=%v", err)
	}
}
