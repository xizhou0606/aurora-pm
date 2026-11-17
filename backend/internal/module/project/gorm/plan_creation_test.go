package projectgorm

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

func TestPlanCreationSagaFinalizesAndReplaysInProjectTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ProjectSet{}, &PlanUnit{}, &ToolMutationApply{}, &PlanCreationSaga{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProjectSet{ID: 11, WorkspaceID: 7, TeamID: 8, Name: "P", Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewMutationRepository(db)
	command := project.PlanMutation{Operation: project.PlanCreate, WorkspaceID: 7, ActorUserID: 9,
		ProjectSetID: 11, WorkflowTemplateID: 10, Title: "V1", Type: "version",
		ApplyKey: "plan-create-0001", InputDigest: strings.Repeat("a", 64)}
	saga, err := repository.BeginPlanCreation(context.Background(), command)
	if err != nil || saga.PlanUnitID == 0 || saga.Result.TeamID != 8 {
		t.Fatalf("saga=%+v err=%v", saga, err)
	}
	var provisional PlanUnit
	if err := db.First(&provisional, saga.PlanUnitID).Error; err != nil || provisional.MutationState != "creating" {
		t.Fatalf("plan=%+v err=%v", provisional, err)
	}
	saga, err = repository.RecordPlanCreationWorkflow(context.Background(), command.ApplyKey, 20, "grooming")
	if err != nil || saga.NextStep != 1 {
		t.Fatalf("saga=%+v err=%v", saga, err)
	}
	result, err := repository.CompletePlanCreation(context.Background(), command.ApplyKey)
	if err != nil || result.ID != saga.PlanUnitID || result.Status != "grooming" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	replayed, err := repository.CompletePlanCreation(context.Background(), command.ApplyKey)
	if err != nil || replayed != result {
		t.Fatalf("replay=%+v err=%v", replayed, err)
	}
	var stored PlanUnit
	if err := db.First(&stored, result.ID).Error; err != nil || stored.MutationState != "" || stored.Status != "grooming" {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
}
