package projectgorm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

type planScheduleGuardStub struct {
	calls int
	err   error
}

func TestPlanMutationScheduleUsesTransactionBoundWorkflowGuard(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err := db.AutoMigrate(&ProjectSet{}, &PlanUnit{}, &ToolMutationApply{}); err != nil {
		t.Fatal(err)
	}
	projectStart := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	projectEnd := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	db.Create(&ProjectSet{ID: 11, WorkspaceID: 7, TeamID: 8, Name: "P", PlannedStartAt: &projectStart, PlannedEndAt: &projectEnd})
	db.Create(&PlanUnit{ID: 12, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, Title: "V1", Type: "version"})
	guard := &planScheduleGuardStub{}
	repository, err := NewPlanMutationRepository(db, func(*gorm.DB) (project.PlanScheduleGuard, error) { return guard, nil })
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	command := project.PlanMutation{Operation: project.PlanUpdate, WorkspaceID: 7, ActorUserID: 3, ID: 12,
		ApplyKey: "plan-schedule-0001", InputDigest: strings.Repeat("b", 64),
		PlannedStartAt: &start, PlannedStartSet: true, PlannedEndAt: &end, PlannedEndSet: true}
	result, err := repository.ApplyPlanUnit(context.Background(), command)
	if err != nil || result.PlannedStartAt == nil || !result.PlannedStartAt.Equal(start) || guard.calls != 1 {
		t.Fatalf("result=%+v calls=%d err=%v", result, guard.calls, err)
	}
	guard.err = project.ErrMutationConflict
	blocked := command
	blocked.ApplyKey, blocked.InputDigest = "plan-schedule-0002", strings.Repeat("c", 64)
	blockedEnd := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	blocked.PlannedEndAt = &blockedEnd
	if _, err := repository.ApplyPlanUnit(context.Background(), blocked); !errors.Is(err, project.ErrMutationConflict) {
		t.Fatalf("blocked err=%v", err)
	}
	var stored PlanUnit
	db.First(&stored, 12)
	if stored.PlannedEndAt == nil || !stored.PlannedEndAt.Equal(end) {
		t.Fatalf("stored end=%v", stored.PlannedEndAt)
	}
}

func (guard *planScheduleGuardStub) ValidatePlanUnitSchedule(context.Context, uint, uint, *time.Time, *time.Time) error {
	guard.calls++
	return guard.err
}

func TestPlanMutationUpdateIsScopedIdempotentAndDeletingGated(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err := db.AutoMigrate(&ProjectSet{}, &PlanUnit{}, &ToolMutationApply{}); err != nil {
		t.Fatal(err)
	}
	db.Create(&[]ProjectSet{{ID: 11, WorkspaceID: 7, TeamID: 8, Name: "P"}, {ID: 21, WorkspaceID: 8, TeamID: 9, Name: "F"}})
	db.Create(&[]PlanUnit{{ID: 12, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, Title: "V1", Type: "version"}, {ID: 22, WorkspaceID: 8, TeamID: 9, ProjectSetID: 21, Title: "V2", Type: "version"}})
	repository := NewMutationRepository(db)
	command := project.PlanMutation{Operation: project.PlanUpdate, WorkspaceID: 7, ActorUserID: 3, ApplyKey: "plan-update-0001", InputDigest: strings.Repeat("a", 64), ID: 12, Title: "Renamed", TitleSet: true}
	first, err := repository.ApplyPlanUnit(context.Background(), command)
	if err != nil || first.Title != "Renamed" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	replayed, err := repository.ApplyPlanUnit(context.Background(), command)
	if err != nil || replayed.Title != "Renamed" {
		t.Fatalf("replay=%+v err=%v", replayed, err)
	}
	var ledgers int64
	db.Model(&ToolMutationApply{}).Where("apply_key = ?", command.ApplyKey).Count(&ledgers)
	if ledgers != 1 {
		t.Fatalf("ledgers=%d", ledgers)
	}
	foreign := command
	foreign.ApplyKey = "plan-update-foreign"
	foreign.ID = 22
	if _, err := repository.ApplyPlanUnit(context.Background(), foreign); !errors.Is(err, project.ErrMutationNotFound) {
		t.Fatalf("foreign err=%v", err)
	}
	db.Model(&PlanUnit{}).Where("id = ?", 12).Update("mutation_state", "deleting")
	blocked := command
	blocked.ApplyKey = "plan-update-blocked"
	blocked.Title = "No"
	if _, err := repository.ApplyPlanUnit(context.Background(), blocked); !errors.Is(err, project.ErrMutationConflict) {
		t.Fatalf("blocked err=%v", err)
	}
}
