package workflowgorm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

func TestSubjectScheduleValidatesPlanAndRequirementWorkflowNodePlans(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err := db.AutoMigrate(&WorkflowInstance{}); err != nil {
		t.Fatal(err)
	}
	rows := []WorkflowInstance{
		{ID: 1, WorkspaceID: 7, SubjectType: "plan_unit", SubjectID: 12, NodePlansJSON: `{"review":{"planned_start_at":"2026-08-05T00:00:00Z","planned_end_at":"2026-08-20T00:00:00Z"}}`},
		{ID: 2, WorkspaceID: 7, SubjectType: "requirement", SubjectID: 13, NodePlansJSON: `{"design":{"planned_start_at":"2026-08-06T00:00:00Z","planned_end_at":"2026-08-18T00:00:00Z"}}`},
		{ID: 3, WorkspaceID: 8, SubjectType: "plan_unit", SubjectID: 12, NodePlansJSON: `{"foreign":{"planned_start_at":"2025-01-01T00:00:00Z"}}`},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewSubjectScheduleRepository(db)
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	if err := repository.ValidateSubjectSchedules(context.Background(), 7, 12, []uint{13}, &start, &end); err != nil {
		t.Fatal(err)
	}
	tightStart := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	if err := repository.ValidateSubjectSchedules(context.Background(), 7, 12, []uint{13}, &tightStart, &end); !errors.Is(err, workflow.ErrSubjectScheduleConflict) {
		t.Fatalf("err=%v", err)
	}
}

func TestSubjectScheduleReturnsWorkspaceScopedRequirementBounds(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err := db.AutoMigrate(&WorkflowInstance{}); err != nil {
		t.Fatal(err)
	}
	rows := []WorkflowInstance{
		{WorkspaceID: 7, SubjectType: "requirement", SubjectID: 13, NodePlansJSON: `{"design":{"planned_start_at":"2026-08-06T00:00:00Z","planned_end_at":"2026-08-18T00:00:00Z"},"test":{"planned_start_at":"2026-08-20T00:00:00Z","planned_end_at":"2026-08-25T00:00:00Z"}}`},
		{WorkspaceID: 8, SubjectType: "requirement", SubjectID: 13, NodePlansJSON: `{"foreign":{"planned_start_at":"2025-01-01T00:00:00Z","planned_end_at":"2027-01-01T00:00:00Z"}}`},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	start, end, found, err := NewSubjectScheduleRepository(db).RequirementScheduleBounds(context.Background(), 7, 13)
	if err != nil || !found || start == nil || end == nil || start.Format(time.DateOnly) != "2026-08-06" || end.Format(time.DateOnly) != "2026-08-25" {
		t.Fatalf("start=%v end=%v found=%v err=%v", start, end, found, err)
	}
}
