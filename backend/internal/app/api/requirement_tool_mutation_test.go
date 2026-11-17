package api

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func TestRequirementWorkflowLifecycleRequiresAndAppliesExactScheduleConfirmation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&projectgorm.ProjectSet{}, &projectgorm.PlanUnit{}, &workflowgorm.WorkflowInstance{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&projectgorm.ProjectSet{ID: 11, WorkspaceID: 7, TeamID: 8, Name: "Project", Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&projectgorm.PlanUnit{ID: 12, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, Title: "V1", Type: "version", Status: "planning"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&workflowgorm.WorkflowInstance{WorkspaceID: 7, SubjectType: "requirement", SubjectID: 13,
		NodePlansJSON: `{"design":{"planned_start_at":"2026-08-06T00:00:00Z","planned_end_at":"2026-08-18T00:00:00Z"}}`}).Error; err != nil {
		t.Fatal(err)
	}
	reads, _ := project.NewReadService(projectgorm.NewReadRepository(db))
	schedules, _ := workflow.NewSubjectScheduleService(workflowgorm.NewSubjectScheduleRepository(db))
	confirm, _ := project.NewWorkflowScheduleService(projectgorm.NewWorkflowScheduleRepository(db))
	lifecycle := requirementWorkflowLifecycle{projects: reads, schedules: schedules, confirm: confirm}

	if err := lifecycle.ValidateRequirementSchedule(context.Background(), 7, 13, 12, false); !errors.Is(err, workflow.ErrSubjectScheduleConflict) {
		t.Fatalf("missing confirmation err=%v", err)
	}
	var untouched projectgorm.PlanUnit
	if err := db.First(&untouched, 12).Error; err != nil || untouched.PlannedStartAt != nil || untouched.PlannedEndAt != nil {
		t.Fatalf("unconfirmed schedule changed: %+v err=%v", untouched, err)
	}
	if err := lifecycle.ValidateRequirementSchedule(context.Background(), 7, 13, 12, true); err != nil {
		t.Fatal(err)
	}
	var confirmed projectgorm.PlanUnit
	if err := db.First(&confirmed, 12).Error; err != nil || confirmed.PlannedStartAt == nil || confirmed.PlannedEndAt == nil {
		t.Fatalf("confirmed schedule=%+v err=%v", confirmed, err)
	}
	if err := lifecycle.ValidateRequirementSchedule(context.Background(), 7, 13, 12, true); !errors.Is(err, workflow.ErrSubjectScheduleConflict) {
		t.Fatalf("redundant confirmation err=%v", err)
	}
}
