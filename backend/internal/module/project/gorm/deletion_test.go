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

func TestProjectDeletionSagaGatesWritesAdvancesByCASAndReplaysResult(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ProjectSet{}, &PlanUnit{}, &DeliveryNode{}, &DeliveryNodeLabel{}, &ProjectPermission{},
		&ProjectMemberRole{}, &ProjectNews{}, &ToolMutationApply{}, &ProjectDeletionSaga{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProjectSet{ID: 11, WorkspaceID: 7, TeamID: 8, Name: "Delete", Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&PlanUnit{ID: 12, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, Title: "V1"}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewMutationRepository(db)
	command := project.SetMutation{Operation: project.SetDelete, WorkspaceID: 7, ID: 11, ApplyKey: "delete-project-0001", InputDigest: strings.Repeat("a", 64)}
	saga, err := repository.BeginProjectDeletion(context.Background(), command, project.DeletionImpact{Local: project.LocalProjectImpact{PlanUnitIDs: []uint{12}}, RequirementIDs: []uint{13}})
	if err != nil || saga.NextStep != project.DeletionPreflight {
		t.Fatalf("saga=%+v err=%v", saga, err)
	}
	if _, err := repository.ApplyProjectSet(context.Background(), project.SetMutation{Operation: project.SetUpdate, WorkspaceID: 7, ID: 11,
		ApplyKey: "update-during-delete", InputDigest: strings.Repeat("b", 64), Name: "Race", NameSet: true}); !errors.Is(err, project.ErrMutationConflict) {
		t.Fatalf("write during delete err=%v", err)
	}
	if _, err := repository.AdvanceProjectDeletion(context.Background(), command.ApplyKey, project.DeletionAI); !errors.Is(err, project.ErrMutationConflict) {
		t.Fatalf("out-of-order advance err=%v", err)
	}
	refreshed, err := repository.RefreshProjectDeletionImpact(context.Background(), command.ApplyKey,
		project.DeletionImpact{Local: project.LocalProjectImpact{PlanUnitIDs: []uint{12}}, RequirementIDs: []uint{13, 14}})
	if err != nil || len(refreshed.RequirementIDs) != 2 {
		t.Fatalf("refreshed=%+v err=%v", refreshed, err)
	}
	for step := project.DeletionPreflight; step < project.DeletionProject; step++ {
		if _, err := repository.AdvanceProjectDeletion(context.Background(), command.ApplyKey, step); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
	}
	result, err := repository.CompleteProjectDeletion(context.Background(), command.ApplyKey)
	if err != nil || !result.Deleted || result.ID != 11 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	replayed, err := repository.CompleteProjectDeletion(context.Background(), command.ApplyKey)
	if err != nil || replayed != result {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	var projectCount, planCount, ledgerCount, sagaCount int64
	db.Model(&ProjectSet{}).Where("id = ?", 11).Count(&projectCount)
	db.Model(&PlanUnit{}).Where("id = ?", 12).Count(&planCount)
	db.Model(&ToolMutationApply{}).Where("apply_key = ? AND result_json <> ''", command.ApplyKey).Count(&ledgerCount)
	db.Model(&ProjectDeletionSaga{}).Where("apply_key = ? AND status = ?", command.ApplyKey, "succeeded").Count(&sagaCount)
	if projectCount != 0 || planCount != 0 || ledgerCount != 1 || sagaCount != 1 {
		t.Fatalf("project=%d plan=%d ledger=%d saga=%d", projectCount, planCount, ledgerCount, sagaCount)
	}
}

func TestProjectDeletionBeginRejectsChangedImpactSnapshotAndRollsBack(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err := db.AutoMigrate(&ProjectSet{}, &PlanUnit{}, &ToolMutationApply{}, &ProjectDeletionSaga{}); err != nil {
		t.Fatal(err)
	}
	db.Create(&ProjectSet{ID: 11, WorkspaceID: 7, TeamID: 8, Name: "Delete", Status: "active"})
	db.Create(&PlanUnit{ID: 12, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, Title: "V1"})
	command := project.SetMutation{Operation: project.SetDelete, WorkspaceID: 7, ID: 11, ApplyKey: "delete-project-0002", InputDigest: strings.Repeat("c", 64)}
	_, err := NewMutationRepository(db).BeginProjectDeletion(context.Background(), command,
		project.DeletionImpact{Local: project.LocalProjectImpact{PlanUnitIDs: []uint{99}}})
	if !errors.Is(err, project.ErrMutationConflict) {
		t.Fatalf("err=%v", err)
	}
	var ledgers, sagas int64
	db.Model(&ToolMutationApply{}).Count(&ledgers)
	db.Model(&ProjectDeletionSaga{}).Count(&sagas)
	var row ProjectSet
	db.First(&row, 11)
	if ledgers != 0 || sagas != 0 || row.MutationState != "" {
		t.Fatalf("ledger=%d saga=%d state=%q", ledgers, sagas, row.MutationState)
	}
}
