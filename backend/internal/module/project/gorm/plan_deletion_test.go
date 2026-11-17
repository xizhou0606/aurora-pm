package projectgorm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

func planDeletionFixture(t *testing.T) (*gorm.DB, *MutationRepository, project.PlanMutation) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ProjectSet{}, &PlanUnit{}, &DeliveryNode{}, &DeliveryNodeLabel{}, &ToolMutationApply{}, &PlanDeletionSaga{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProjectSet{ID: 11, WorkspaceID: 7, TeamID: 8, Name: "P", Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&PlanUnit{ID: 12, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, Title: "V1", Status: "planning"}).Error; err != nil {
		t.Fatal(err)
	}
	command := project.PlanMutation{Operation: project.PlanDelete, WorkspaceID: 7, ActorUserID: 9, ID: 12,
		ApplyKey: "plan-delete-0001", InputDigest: strings.Repeat("a", 64)}
	return db, NewMutationRepository(db), command
}

func TestPlanDeletionSagaGatesPlanMutationAdvancesCASAndReplays(t *testing.T) {
	db, repository, command := planDeletionFixture(t)
	saga, err := repository.BeginPlanDeletion(context.Background(), command)
	if err != nil || saga.NextStep != 0 || saga.ProjectSetID != 11 {
		t.Fatalf("saga=%+v err=%v", saga, err)
	}
	if _, err := repository.ApplyPlanUnit(context.Background(), project.PlanMutation{Operation: project.PlanUpdate,
		WorkspaceID: 7, ActorUserID: 9, ID: 12, ApplyKey: "update-during-delete", InputDigest: strings.Repeat("b", 64),
		Title: "race", TitleSet: true}); !errors.Is(err, project.ErrMutationConflict) {
		t.Fatalf("mutation gate err=%v", err)
	}
	if _, err := repository.AdvancePlanDeletion(context.Background(), command.ApplyKey, 1); !errors.Is(err, project.ErrMutationConflict) {
		t.Fatalf("out of order err=%v", err)
	}
	for step := 0; step < 2; step++ {
		if _, err := repository.AdvancePlanDeletion(context.Background(), command.ApplyKey, step); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
	}
	result, err := repository.CompletePlanDeletion(context.Background(), command.ApplyKey)
	if err != nil || !result.Deleted || result.ID != 12 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	replayed, err := repository.CompletePlanDeletion(context.Background(), command.ApplyKey)
	if err != nil || replayed != result {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	var plans, labels, nodes, completed, ledgers int64
	db.Model(&PlanUnit{}).Where("id = ?", 12).Count(&plans)
	db.Model(&DeliveryNodeLabel{}).Where("plan_unit_id = ?", 12).Count(&labels)
	db.Model(&DeliveryNode{}).Where("plan_unit_id = ?", 12).Count(&nodes)
	db.Model(&PlanDeletionSaga{}).Where("apply_key = ? AND status = ? AND next_step = ?", command.ApplyKey, "succeeded", 3).Count(&completed)
	db.Model(&ToolMutationApply{}).Where("apply_key = ? AND result_json <> ''", command.ApplyKey).Count(&ledgers)
	if plans != 0 || labels != 0 || nodes != 0 || completed != 1 || ledgers != 1 {
		t.Fatalf("plans=%d labels=%d nodes=%d completed=%d ledgers=%d", plans, labels, nodes, completed, ledgers)
	}
}

func TestPlanDeletionRejectClearsGateButKeepsBoundLedger(t *testing.T) {
	db, repository, command := planDeletionFixture(t)
	if _, err := repository.BeginPlanDeletion(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if err := repository.RejectPlanDeletion(context.Background(), command.ApplyKey); err != nil {
		t.Fatal(err)
	}
	var unit PlanUnit
	if err := db.First(&unit, command.ID).Error; err != nil {
		t.Fatal(err)
	}
	var saga PlanDeletionSaga
	if err := db.Where("apply_key = ?", command.ApplyKey).First(&saga).Error; err != nil {
		t.Fatal(err)
	}
	var ledgers int64
	db.Model(&ToolMutationApply{}).Where("apply_key = ? AND result_json = ''", command.ApplyKey).Count(&ledgers)
	if unit.MutationState != "" || saga.Status != "rejected" || ledgers != 1 {
		t.Fatalf("state=%q saga=%q ledgers=%d", unit.MutationState, saga.Status, ledgers)
	}
}

func TestPlanDeletionWorkspaceIsolationRollsBackBegin(t *testing.T) {
	db, repository, command := planDeletionFixture(t)
	command.WorkspaceID = 8
	_, err := repository.BeginPlanDeletion(context.Background(), command)
	if !errors.Is(err, project.ErrMutationNotFound) {
		t.Fatalf("err=%v", err)
	}
	var ledgers, sagas int64
	db.Model(&ToolMutationApply{}).Count(&ledgers)
	db.Model(&PlanDeletionSaga{}).Count(&sagas)
	if ledgers != 0 || sagas != 0 {
		t.Fatalf("ledgers=%d sagas=%d", ledgers, sagas)
	}
}
