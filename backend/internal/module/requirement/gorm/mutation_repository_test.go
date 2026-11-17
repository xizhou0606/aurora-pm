package requirementgorm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"gorm.io/gorm"
)

type mutationTestEffect struct {
	ID          uint `gorm:"primaryKey"`
	WorkspaceID uint `gorm:"not null;index"`
	Kind        string
}

type mutationTestDependencies struct {
	tx          *gorm.DB
	scope       requirement.PlanScope
	history     bool
	deletionErr error
}

func (value mutationTestDependencies) ResolveRequirementPlan(_ context.Context, workspaceID, planID uint) (requirement.PlanScope, error) {
	if workspaceID != value.scope.WorkspaceID || planID != value.scope.PlanUnitID {
		return requirement.PlanScope{}, requirement.ErrMutationNotFound
	}
	return value.scope, nil
}
func (value mutationTestDependencies) StartRequirementWorkflow(context.Context, requirement.Mutation, requirement.MutationResult) (uint, string, error) {
	if err := value.record("workflow-start"); err != nil {
		return 0, "", err
	}
	return 41, "grooming", nil
}
func (value mutationTestDependencies) ValidateRequirementSchedule(context.Context, uint, uint, uint, bool) error {
	return value.record("schedule")
}
func (value mutationTestDependencies) RequirementHasWorkflowHistory(context.Context, uint, uint) (bool, error) {
	return value.history, nil
}
func (value mutationTestDependencies) DeleteRequirementWorkflow(context.Context, uint, uint) error {
	return value.record("workflow-delete")
}
func (value mutationTestDependencies) MoveRequirementDependents(context.Context, uint, uint, requirement.PlanScope) (int64, int64, error) {
	return 2, 3, value.record("dependent-move")
}
func (value mutationTestDependencies) ValidateRequirementDeletion(context.Context, uint, uint) error {
	return value.deletionErr
}
func (value mutationTestDependencies) DeleteRequirementDependents(context.Context, uint, uint) (int64, int64, error) {
	return 2, 3, value.record("dependent-delete")
}
func (value mutationTestDependencies) RecordRequirementMutation(context.Context, requirement.MutationEffect) error {
	return value.record("audit")
}
func (value mutationTestDependencies) EnqueueRequirementCover(context.Context, requirement.MutationResult) error {
	return value.record("cover")
}
func (value mutationTestDependencies) record(kind string) error {
	return value.tx.Create(&mutationTestEffect{WorkspaceID: value.scope.WorkspaceID, Kind: kind}).Error
}

func TestRequirementMutationRepositoryCreateReplayAndBinding(t *testing.T) {
	repository, db := newRequirementMutationTestRepository(t, false)
	command := requirement.Mutation{
		Operation: requirement.MutationCreate, WorkspaceID: 7, PrincipalID: 3,
		ApplyKey: "requirement-create-0001", ChangeSetID: "changeset-create-0001", InputDigest: strings.Repeat("a", 64),
		PlanUnitID: 31, WorkflowTemplateID: 9, Title: "Checkout flow", ProductDocURL: "https://example.test/doc",
	}
	first, err := repository.ApplyRequirementMutation(context.Background(), command)
	if err != nil || first.Requirement == nil || first.Requirement.ID == 0 || first.Requirement.WorkflowInstanceID == nil {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	replayed, err := repository.ApplyRequirementMutation(context.Background(), command)
	if err != nil || replayed.Requirement == nil || replayed.Requirement.ID != first.Requirement.ID {
		t.Fatalf("replay=%+v err=%v", replayed, err)
	}
	var requirements, ledgers, effects int64
	_ = db.Model(&Requirement{}).Count(&requirements).Error
	_ = db.Model(&ToolMutationApply{}).Count(&ledgers).Error
	_ = db.Model(&mutationTestEffect{}).Count(&effects).Error
	if requirements != 1 || ledgers != 1 || effects != 3 {
		t.Fatalf("requirements=%d ledgers=%d effects=%d", requirements, ledgers, effects)
	}
	wrongChangeSet := command
	wrongChangeSet.ChangeSetID = "changeset-other-0001"
	if _, err := repository.ApplyRequirementMutation(context.Background(), wrongChangeSet); !errors.Is(err, requirement.ErrMutationConflict) {
		t.Fatalf("change set binding err=%v", err)
	}
	wrongWorkspace := command
	wrongWorkspace.WorkspaceID = 8
	if _, err := repository.ApplyRequirementMutation(context.Background(), wrongWorkspace); !errors.Is(err, requirement.ErrMutationConflict) {
		t.Fatalf("workspace binding err=%v", err)
	}
}

func TestRequirementMutationRepositoryUpdateBatchDeleteAndRollback(t *testing.T) {
	repository, db := newRequirementMutationTestRepository(t, false)
	planID := uint(21)
	rows := []Requirement{
		{WorkspaceID: 7, TeamID: 10, ProjectSetID: 20, ScheduledPlanUnitID: &planID, Version: 2, Title: "One", Priority: "normal", Status: "grooming"},
		{WorkspaceID: 7, TeamID: 10, ProjectSetID: 20, ScheduledPlanUnitID: &planID, Version: 4, Title: "Two", Priority: "normal", Status: "grooming"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	update := requirement.Mutation{Operation: requirement.MutationUpdate, WorkspaceID: 7, PrincipalID: 3,
		ApplyKey: "requirement-update-0001", ChangeSetID: "changeset-update-0001", InputDigest: strings.Repeat("b", 64),
		ID: rows[0].ID, ExpectedVersion: 2, Title: "Renamed", TitleSet: true, ProductDocURL: "https://example.test/new", ProductDocURLSet: true,
		PlanUnitID: 31,
	}
	updated, err := repository.ApplyRequirementMutation(context.Background(), update)
	if err != nil || updated.Requirement == nil || updated.Requirement.Title != "Renamed" || updated.Requirement.Version != 3 || updated.Requirement.PlanUnitID != 31 {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	batch := requirement.Mutation{Operation: requirement.MutationBatchMove, WorkspaceID: 7, PrincipalID: 3,
		ApplyKey: "requirement-batch-0001", ChangeSetID: "changeset-batch-0001", InputDigest: strings.Repeat("c", 64), PlanUnitID: 31,
		Items: []requirement.BatchMoveItem{{RequirementID: rows[1].ID, ExpectedVersion: 4}},
	}
	moved, err := repository.ApplyRequirementMutation(context.Background(), batch)
	if err != nil || moved.Batch == nil || moved.Batch.MovedCount != 1 || moved.Batch.AffectedTaskCount != 2 || moved.Batch.AffectedQualityCount != 3 {
		t.Fatalf("moved=%+v err=%v", moved, err)
	}
	remove := requirement.Mutation{Operation: requirement.MutationDelete, WorkspaceID: 7, PrincipalID: 3,
		ApplyKey: "requirement-delete-0001", ChangeSetID: "changeset-delete-0001", InputDigest: strings.Repeat("d", 64), ID: rows[0].ID,
	}
	deleted, err := repository.ApplyRequirementMutation(context.Background(), remove)
	if err != nil || deleted.Requirement == nil || !deleted.Requirement.Deleted {
		t.Fatalf("deleted=%+v err=%v", deleted, err)
	}
	var remaining int64
	if err := db.Model(&Requirement{}).Where("id = ?", rows[0].ID).Count(&remaining).Error; err != nil || remaining != 0 {
		t.Fatalf("remaining=%d err=%v", remaining, err)
	}

	callback := "test:requirement_ledger_failure"
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == (ToolMutationApply{}).TableName() {
			tx.AddError(errors.New("private ledger failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	rollback := requirement.Mutation{Operation: requirement.MutationCreate, WorkspaceID: 7, PrincipalID: 3,
		ApplyKey: "requirement-rollback-0001", ChangeSetID: "changeset-rollback-0001", InputDigest: strings.Repeat("e", 64),
		PlanUnitID: 31, WorkflowTemplateID: 9, Title: "Must rollback",
	}
	if _, err := repository.ApplyRequirementMutation(context.Background(), rollback); err == nil {
		t.Fatal("expected ledger failure")
	}
	var rolledBackRows, rolledBackLedgers int64
	_ = db.Model(&Requirement{}).Where("title = ?", rollback.Title).Count(&rolledBackRows).Error
	_ = db.Model(&ToolMutationApply{}).Where("apply_key = ?", rollback.ApplyKey).Count(&rolledBackLedgers).Error
	if rolledBackRows != 0 || rolledBackLedgers != 0 {
		t.Fatalf("rollback leaked requirements=%d ledgers=%d", rolledBackRows, rolledBackLedgers)
	}
}

func TestRequirementMutationRepositoryDeleteHistoryFailsClosed(t *testing.T) {
	repository, db := newRequirementMutationTestRepository(t, true)
	row := Requirement{WorkspaceID: 7, TeamID: 10, ProjectSetID: 20, PlanUnitID: 31, Version: 1, Title: "Protected", Priority: "normal", Status: "grooming"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	command := requirement.Mutation{Operation: requirement.MutationDelete, WorkspaceID: 7, PrincipalID: 3,
		ApplyKey: "requirement-protected-0001", ChangeSetID: "changeset-protected-0001", InputDigest: strings.Repeat("f", 64), ID: row.ID,
	}
	if _, err := repository.ApplyRequirementMutation(context.Background(), command); !errors.Is(err, requirement.ErrMutationConflict) {
		t.Fatalf("delete history err=%v", err)
	}
	var requirements, ledgers int64
	_ = db.Model(&Requirement{}).Where("id = ?", row.ID).Count(&requirements).Error
	_ = db.Model(&ToolMutationApply{}).Where("apply_key = ?", command.ApplyKey).Count(&ledgers).Error
	if requirements != 1 || ledgers != 0 {
		t.Fatalf("requirements=%d ledgers=%d", requirements, ledgers)
	}
}

func TestRequirementMutationRepositoryDeleteAttachmentConflictRollsBackBeforeEffects(t *testing.T) {
	repository, db := newRequirementMutationTestRepositoryWithDeletionError(t, false, requirement.ErrMutationConflict)
	row := Requirement{WorkspaceID: 7, TeamID: 10, ProjectSetID: 20, PlanUnitID: 31, Version: 1, Title: "Protected attachment", Priority: "normal", Status: "grooming"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	command := requirement.Mutation{Operation: requirement.MutationDelete, WorkspaceID: 7, PrincipalID: 3,
		ApplyKey: "requirement-attachment-0001", ChangeSetID: "changeset-attachment-0001", InputDigest: strings.Repeat("1", 64), ID: row.ID,
	}
	if _, err := repository.ApplyRequirementMutation(context.Background(), command); !errors.Is(err, requirement.ErrMutationConflict) {
		t.Fatalf("attachment conflict err=%v", err)
	}
	var requirements, ledgers, effects int64
	db.Model(&Requirement{}).Where("id = ?", row.ID).Count(&requirements)
	db.Model(&ToolMutationApply{}).Where("apply_key = ?", command.ApplyKey).Count(&ledgers)
	db.Model(&mutationTestEffect{}).Count(&effects)
	if requirements != 1 || ledgers != 0 || effects != 0 {
		t.Fatalf("conflict leaked requirements=%d ledgers=%d effects=%d", requirements, ledgers, effects)
	}
}

func newRequirementMutationTestRepository(t *testing.T, history bool) (*MutationRepository, *gorm.DB) {
	return newRequirementMutationTestRepositoryWithDeletionError(t, history, nil)
}

func newRequirementMutationTestRepositoryWithDeletionError(t *testing.T, history bool, deletionErr error) (*MutationRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&Requirement{}, &RequirementCover{}, &RequirementMaterial{}, &ToolMutationApply{}, &mutationTestEffect{}); err != nil {
		t.Fatal(err)
	}
	scope := requirement.PlanScope{WorkspaceID: 7, TeamID: 11, ProjectSetID: 22, PlanUnitID: 31, Type: "version"}
	repository, err := NewMutationRepository(db, func(tx *gorm.DB) (MutationDependencies, error) {
		deps := mutationTestDependencies{tx: tx, scope: scope, history: history, deletionErr: deletionErr}
		return MutationDependencies{Scopes: deps, Workflow: deps, Dependents: deps, Effects: deps, Covers: deps}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return repository, db
}
