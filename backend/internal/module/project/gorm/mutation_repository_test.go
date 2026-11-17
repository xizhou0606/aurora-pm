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

func TestMutationRepositoryDeliveryLabelsAreIdempotentAndWorkspaceScoped(t *testing.T) {
	repository, db := newProjectMutationTestRepository(t)
	create := project.DeliveryLabelMutation{
		Operation: project.DeliveryLabelCreate, WorkspaceID: 1, ApplyKey: "changeset-create-0001",
		InputDigest: strings.Repeat("a", 64), ID: 12, Name: "Release", SortOrder: 1,
	}
	first, err := repository.ApplyDeliveryLabel(context.Background(), create)
	if err != nil || first.ID == 0 || first.PlanUnitID != 12 || first.ProjectSetID != 11 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	replayed, err := repository.ApplyDeliveryLabel(context.Background(), create)
	if err != nil || replayed != first {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	var count int64
	if err := db.Model(&DeliveryNodeLabel{}).Where("workspace_id = ?", 1).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	conflict := create
	conflict.InputDigest = strings.Repeat("b", 64)
	if _, err := repository.ApplyDeliveryLabel(context.Background(), conflict); !errors.Is(err, project.ErrMutationConflict) {
		t.Fatalf("conflict err=%v", err)
	}
	foreign := create
	foreign.ApplyKey, foreign.ID = "changeset-foreign-0001", 22
	if _, err := repository.ApplyDeliveryLabel(context.Background(), foreign); !errors.Is(err, project.ErrMutationNotFound) {
		t.Fatalf("foreign err=%v", err)
	}

	update := project.DeliveryLabelMutation{
		Operation: project.DeliveryLabelUpdate, WorkspaceID: 1, ApplyKey: "changeset-update-0001",
		InputDigest: strings.Repeat("c", 64), ID: first.ID, Name: "General Availability",
	}
	updated, err := repository.ApplyDeliveryLabel(context.Background(), update)
	if err != nil || updated.Name != "General Availability" {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	if err := db.Create(&DeliveryNode{WorkspaceID: 1, PlanUnitID: 12, LabelID: &first.ID}).Error; err != nil {
		t.Fatal(err)
	}
	remove := project.DeliveryLabelMutation{
		Operation: project.DeliveryLabelDelete, WorkspaceID: 1, ApplyKey: "changeset-delete-0001",
		InputDigest: strings.Repeat("d", 64), ID: first.ID,
	}
	if _, err := repository.ApplyDeliveryLabel(context.Background(), remove); !errors.Is(err, project.ErrMutationConflict) {
		t.Fatalf("referenced delete err=%v", err)
	}
	var ledgerCount int64
	if err := db.Model(&ToolMutationApply{}).Where("apply_key = ?", remove.ApplyKey).Count(&ledgerCount).Error; err != nil || ledgerCount != 0 {
		t.Fatalf("failed apply ledger=%d err=%v", ledgerCount, err)
	}
	if err := db.Where("workspace_id = ? AND label_id = ?", 1, first.ID).Delete(&DeliveryNode{}).Error; err != nil {
		t.Fatal(err)
	}
	deleted, err := repository.ApplyDeliveryLabel(context.Background(), remove)
	if err != nil || !deleted.Deleted || deleted.ID != first.ID {
		t.Fatalf("deleted=%+v err=%v", deleted, err)
	}
}

func TestMutationRepositoryRollsBackSideEffectWhenLedgerResultFails(t *testing.T) {
	repository, db := newProjectMutationTestRepository(t)
	const callback = "test:fail_project_apply_result"
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == (ToolMutationApply{}).TableName() {
			tx.AddError(errors.New("private ledger failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	command := project.DeliveryLabelMutation{
		Operation: project.DeliveryLabelCreate, WorkspaceID: 1, ApplyKey: "changeset-rollback-0001",
		InputDigest: strings.Repeat("e", 64), ID: 12, Name: "Rollback",
	}
	if _, err := repository.ApplyDeliveryLabel(context.Background(), command); err == nil {
		t.Fatal("expected ledger result failure")
	}
	var labelCount, ledgerCount int64
	_ = db.Model(&DeliveryNodeLabel{}).Where("name = ?", "Rollback").Count(&labelCount).Error
	_ = db.Model(&ToolMutationApply{}).Where("apply_key = ?", command.ApplyKey).Count(&ledgerCount).Error
	if labelCount != 0 || ledgerCount != 0 {
		t.Fatalf("transaction leaked label=%d ledger=%d", labelCount, ledgerCount)
	}
}

func TestMutationRepositoryProjectSetsAreIdempotentScopedAndFailClosedOnSchedule(t *testing.T) {
	repository, db := newProjectMutationTestRepository(t)
	create := project.SetMutation{
		Operation: project.SetCreate, WorkspaceID: 1, ActorUserID: 3, ApplyKey: "changeset-set-create-0001",
		InputDigest: strings.Repeat("f", 64), TeamID: 101, TeamIDSet: true, Name: "Roadmap", NameSet: true,
	}
	first, err := repository.ApplyProjectSet(context.Background(), create)
	if err != nil || first.ID == 0 || first.Code == "" || first.Status != "active" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	replayed, err := repository.ApplyProjectSet(context.Background(), create)
	if err != nil || replayed != first {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	var count int64
	if err := db.Model(&ProjectSet{}).Where("workspace_id = ? AND name = ?", 1, "Roadmap").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	update := project.SetMutation{
		Operation: project.SetUpdate, WorkspaceID: 1, ActorUserID: 3, ApplyKey: "changeset-set-update-0001",
		InputDigest: strings.Repeat("1", 64), ID: 11, Name: "Renamed", NameSet: true,
	}
	updated, err := repository.ApplyProjectSet(context.Background(), update)
	if err != nil || updated.Name != "Renamed" {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	schedule := update
	schedule.ApplyKey, schedule.InputDigest = "changeset-set-schedule-0001", strings.Repeat("2", 64)
	schedule.PlannedStartSet = true
	now := time.Now().UTC()
	schedule.PlannedStartAt = &now
	if _, err := repository.ApplyProjectSet(context.Background(), schedule); !errors.Is(err, project.ErrMutationConflict) {
		t.Fatalf("schedule err=%v", err)
	}
	var ledgers int64
	if err := db.Model(&ToolMutationApply{}).Where("apply_key = ?", schedule.ApplyKey).Count(&ledgers).Error; err != nil || ledgers != 0 {
		t.Fatalf("schedule ledger=%d err=%v", ledgers, err)
	}
	transfer := project.SetMutation{
		Operation: project.SetTransferOwner, WorkspaceID: 1, ActorUserID: 3, ApplyKey: "changeset-set-transfer-0001",
		InputDigest: strings.Repeat("3", 64), ID: 11, NewOwnerUserID: 33,
	}
	transferred, err := repository.ApplyProjectSet(context.Background(), transfer)
	if err != nil || transferred.OwnerUserID == nil || *transferred.OwnerUserID != 33 {
		t.Fatalf("transferred=%+v err=%v", transferred, err)
	}
	transfer.WorkspaceID, transfer.ApplyKey = 2, "changeset-set-foreign-0001"
	if _, err := repository.ApplyProjectSet(context.Background(), transfer); !errors.Is(err, project.ErrMutationNotFound) {
		t.Fatalf("foreign err=%v", err)
	}
}

func newProjectMutationTestRepository(t *testing.T) (*MutationRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ProjectSet{}, &PlanUnit{}, &DeliveryNodeLabel{}, &DeliveryNode{}, &ToolMutationApply{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]ProjectSet{
		{ID: 11, WorkspaceID: 1, TeamID: 101, Name: "One", Status: "active"},
		{ID: 21, WorkspaceID: 2, TeamID: 201, Name: "Two", Status: "active"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]PlanUnit{
		{ID: 12, WorkspaceID: 1, TeamID: 101, ProjectSetID: 11, Title: "V1", Type: "version", Status: "planning"},
		{ID: 22, WorkspaceID: 2, TeamID: 201, ProjectSetID: 21, Title: "V2", Type: "version", Status: "planning"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	return NewMutationRepository(db), db
}
