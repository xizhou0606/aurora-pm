package qualitygorm

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"gorm.io/gorm"
)

type qualityMutationEffectNoop struct{}

func (qualityMutationEffectNoop) RecordBugMutation(context.Context, quality.BugMutationEffect) error {
	return nil
}

func TestMutationRepositoryCreateReplayIsDurableExactlyOnce(t *testing.T) {
	db, requirementID := openQualityMutationDB(t)
	repository := newQualityMutationRepository(t, db)
	command := qualityCreateCommand(requirementID, "apply-create")

	first, err := repository.ApplyBugMutation(context.Background(), command)
	if err != nil || first.Bug == nil {
		t.Fatalf("first apply result=%+v err=%v", first, err)
	}
	second, err := repository.ApplyBugMutation(context.Background(), command)
	if err != nil || second.Bug == nil || second.Bug.ID != first.Bug.ID {
		t.Fatalf("replay result=%+v err=%v", second, err)
	}
	assertQualityMutationCounts(t, db, 1, 1, 1)

	conflict := command
	conflict.WorkspaceID = 8
	if _, err := repository.ApplyBugMutation(context.Background(), conflict); !errors.Is(err, quality.ErrBugMutationConflict) {
		t.Fatalf("cross-workspace replay error=%v", err)
	}
	changeSetConflict := command
	changeSetConflict.ApplyKey = "different-apply-key"
	if _, err := repository.ApplyBugMutation(context.Background(), changeSetConflict); !errors.Is(err, quality.ErrBugMutationConflict) {
		t.Fatalf("ChangeSet replay error=%v", err)
	}
}

func TestMutationRepositoryBootstrapRejectsMissingDependencies(t *testing.T) {
	db, _ := openQualityMutationDB(t)
	if _, err := NewMutationRepository(nil, func(*gorm.DB) (requirement.MutationScopeResolver, error) { return nil, nil }, func(*gorm.DB) (quality.BugMutationEffectSink, error) { return qualityMutationEffectNoop{}, nil }); !errors.Is(err, quality.ErrBugMutationInvalid) {
		t.Fatalf("nil database error=%v", err)
	}
	if _, err := NewMutationRepository(db, nil, func(*gorm.DB) (quality.BugMutationEffectSink, error) { return qualityMutationEffectNoop{}, nil }); !errors.Is(err, quality.ErrBugMutationInvalid) {
		t.Fatalf("nil scope factory error=%v", err)
	}
	if _, err := NewMutationRepository(db, func(*gorm.DB) (requirement.MutationScopeResolver, error) { return nil, nil }, nil); !errors.Is(err, quality.ErrBugMutationInvalid) {
		t.Fatalf("nil effect factory error=%v", err)
	}
}

func TestMutationRepositoryBatchCreateRollsBackAtomically(t *testing.T) {
	db, requirementID := openQualityMutationDB(t)
	repository := newQualityMutationRepository(t, db)
	command := qualityCreateCommand(requirementID, "apply-batch")
	command.Operation = quality.BugBatchCreate
	command.Items = []quality.BatchBugMutationItem{
		{ClientID: "ok", Values: command.Values},
		{ClientID: "missing", Values: quality.BugMutationValues{RequirementID: requirementID + 999, Title: "must rollback", Status: "triage", Priority: "medium", SeverityKey: "p2"}},
	}
	command.Values = quality.BugMutationValues{}

	if _, err := repository.ApplyBugMutation(context.Background(), command); !errors.Is(err, quality.ErrBugMutationNotFound) {
		t.Fatalf("batch error=%v", err)
	}
	assertQualityMutationCounts(t, db, 0, 0, 0)
}

func TestMutationRepositoryImplementsUpdateDeleteAndBatchDelete(t *testing.T) {
	db, requirementID := openQualityMutationDB(t)
	repository := newQualityMutationRepository(t, db)
	batch := qualityCreateCommand(requirementID, "apply-batch-success")
	batch.Operation = quality.BugBatchCreate
	batch.Items = []quality.BatchBugMutationItem{
		{ClientID: "first", Values: batch.Values},
		{ClientID: "second", Values: quality.BugMutationValues{RequirementID: requirementID, Title: "second", Status: "triage", Priority: "medium", SeverityKey: "p2"}},
	}
	batch.Values = quality.BugMutationValues{}
	created, err := repository.ApplyBugMutation(context.Background(), batch)
	if err != nil || len(created.Created) != 2 || !created.Atomic {
		t.Fatalf("created=%+v err=%v", created, err)
	}

	update := qualityCreateCommand(requirementID, "apply-update")
	update.Operation, update.ID = quality.BugUpdate, created.Created[0].Bug.ID
	update.Values = quality.BugMutationValues{Title: "updated", Fields: map[string]bool{"title": true}}
	updated, err := repository.ApplyBugMutation(context.Background(), update)
	if err != nil || updated.Bug == nil || updated.Bug.Title != "updated" {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}

	assigneeID := uint(99)
	batchAssign := qualityCreateCommand(requirementID, "apply-batch-assign")
	batchAssign.Operation, batchAssign.IDs = quality.BugBatchAssign, []uint{
		created.Created[0].Bug.ID,
		created.Created[1].Bug.ID,
	}
	batchAssign.Values = quality.BugMutationValues{
		AssigneeUserID: &assigneeID,
		Fields:         map[string]bool{"assignee_user_id": true},
	}
	batchAssigned, err := repository.ApplyBugMutation(context.Background(), batchAssign)
	if err != nil || !batchAssigned.Atomic || batchAssigned.UpdatedCount != 2 ||
		batchAssigned.Updated[0].AssigneeUserID == nil || *batchAssigned.Updated[0].AssigneeUserID != assigneeID {
		t.Fatalf("batch assigned=%+v err=%v", batchAssigned, err)
	}

	remove := qualityCreateCommand(requirementID, "apply-delete")
	remove.Operation, remove.ID, remove.Values = quality.BugDelete, created.Created[0].Bug.ID, quality.BugMutationValues{}
	deleted, err := repository.ApplyBugMutation(context.Background(), remove)
	if err != nil || !deleted.Deleted || deleted.DeletedID != remove.ID {
		t.Fatalf("deleted=%+v err=%v", deleted, err)
	}

	batchRemove := qualityCreateCommand(requirementID, "apply-batch-delete")
	batchRemove.Operation, batchRemove.IDs, batchRemove.Values = quality.BugBatchDelete, []uint{created.Created[1].Bug.ID}, quality.BugMutationValues{}
	batchDeleted, err := repository.ApplyBugMutation(context.Background(), batchRemove)
	if err != nil || !batchDeleted.Atomic || batchDeleted.DeletedCount != 1 {
		t.Fatalf("batch deleted=%+v err=%v", batchDeleted, err)
	}
	assertQualityMutationCounts(t, db, 0, 0, 5)
}

func TestMutationRepositoryRejectsForeignWorkspaceScopeWithoutLedger(t *testing.T) {
	db, requirementID := openQualityMutationDB(t)
	repository := newQualityMutationRepository(t, db)
	command := qualityCreateCommand(requirementID, "apply-foreign")
	command.WorkspaceID = 8
	if _, err := repository.ApplyBugMutation(context.Background(), command); !errors.Is(err, quality.ErrBugMutationNotFound) {
		t.Fatalf("foreign scope error=%v", err)
	}
	assertQualityMutationCounts(t, db, 0, 0, 0)
}

func TestMutationRepositoryScopeMismatchFailsClosedAndRollsBackLedger(t *testing.T) {
	db, requirementID := openQualityMutationDB(t)
	bug := QualityBug{
		WorkspaceID: 7, TeamID: 11, ProjectSetID: 21, ProjectID: 999, RequirementID: requirementID,
		BugKey: "BUG-stale", Title: "stale", Status: "triage", Priority: "medium", SeverityKey: "p2",
	}
	if err := db.Create(&bug).Error; err != nil {
		t.Fatal(err)
	}
	repository := newQualityMutationRepository(t, db)
	command := qualityCreateCommand(requirementID, "apply-stale")
	command.Operation, command.ID = quality.BugUpdate, bug.ID
	command.Values = quality.BugMutationValues{Title: "should not persist", Fields: map[string]bool{"title": true}}

	if _, err := repository.ApplyBugMutation(context.Background(), command); !errors.Is(err, quality.ErrBugMutationConflict) {
		t.Fatalf("scope mismatch error=%v", err)
	}
	var stored QualityBug
	if err := db.First(&stored, bug.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Title != "stale" {
		t.Fatalf("title changed to %q", stored.Title)
	}
	var ledgers int64
	if err := db.Model(&ToolMutationApply{}).Count(&ledgers).Error; err != nil || ledgers != 0 {
		t.Fatalf("ledger count=%d err=%v", ledgers, err)
	}
}

func TestMutationRepositoryConcurrentRequirementMoveVersionMismatchFailsClosed(t *testing.T) {
	db, requirementID := openQualityMutationDB(t)
	repository := newQualityMutationRepository(t, db)
	created, err := repository.ApplyBugMutation(context.Background(), qualityCreateCommand(requirementID, "apply-before-move"))
	if err != nil || created.Bug == nil {
		t.Fatalf("create=%+v err=%v", created, err)
	}
	newPlanUnitID := uint(32)
	if err := db.Model(&requirementgorm.Requirement{}).Where("id = ? AND workspace_id = ?", requirementID, 7).
		Updates(map[string]any{"project_new_id": 22, "version_task_new_id": newPlanUnitID, "version": gorm.Expr("version + 1")}).Error; err != nil {
		t.Fatal(err)
	}
	update := qualityCreateCommand(requirementID, "apply-after-move")
	update.Operation, update.ID = quality.BugUpdate, created.Bug.ID
	update.Values = quality.BugMutationValues{Title: "must not update", Fields: map[string]bool{"title": true}}
	if _, err := repository.ApplyBugMutation(context.Background(), update); !errors.Is(err, quality.ErrBugMutationConflict) {
		t.Fatalf("concurrent scope/version mismatch error=%v", err)
	}
	var stored QualityBug
	if err := db.First(&stored, created.Bug.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Title != "tool bug" {
		t.Fatalf("title changed to %q", stored.Title)
	}
	var ledgers int64
	if err := db.Model(&ToolMutationApply{}).Count(&ledgers).Error; err != nil || ledgers != 1 {
		t.Fatalf("ledger count=%d err=%v", ledgers, err)
	}
}

func TestMutationRepositoryBusinessValidationRollsBackLedger(t *testing.T) {
	db, requirementID := openQualityMutationDB(t)
	repository := newQualityMutationRepository(t, db)
	created, err := repository.ApplyBugMutation(context.Background(), qualityCreateCommand(requirementID, "apply-validation-create"))
	if err != nil || created.Bug == nil {
		t.Fatalf("create=%+v err=%v", created, err)
	}
	update := qualityCreateCommand(requirementID, "apply-validation-update")
	update.Operation, update.ID = quality.BugUpdate, created.Bug.ID
	update.Values = quality.BugMutationValues{Status: "done", Fields: map[string]bool{"status": true}}
	if _, err := repository.ApplyBugMutation(context.Background(), update); !errors.Is(err, quality.ErrBugMutationInvalid) {
		t.Fatalf("validation error=%v", err)
	}
	var stored QualityBug
	if err := db.First(&stored, created.Bug.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "triage" {
		t.Fatalf("status changed to %q", stored.Status)
	}
	var ledgers int64
	if err := db.Model(&ToolMutationApply{}).Count(&ledgers).Error; err != nil || ledgers != 1 {
		t.Fatalf("ledger count=%d err=%v", ledgers, err)
	}
}

func TestMutationRepositoryRetryAfterCommitPhaseFailureHasOneSideEffect(t *testing.T) {
	db, requirementID := openQualityMutationDB(t)
	repository := newQualityMutationRepository(t, db)
	command := qualityCreateCommand(requirementID, "apply-retry")
	callbackName := "test:fail_quality_apply_result_once"
	failed := false
	if err := db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if !failed && tx.Statement != nil && tx.Statement.Table == (ToolMutationApply{}).TableName() {
			failed = true
			tx.AddError(fmt.Errorf("simulated commit phase failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ApplyBugMutation(context.Background(), command); err == nil {
		t.Fatal("expected simulated failure")
	}
	if err := db.Callback().Update().Remove(callbackName); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ApplyBugMutation(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	assertQualityMutationCounts(t, db, 1, 1, 1)
}

func openQualityMutationDB(t *testing.T) (*gorm.DB, uint) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&requirementgorm.Requirement{}, &QualityBug{}, &QualityBugActivity{}, &QualityBugComment{}, &ToolMutationApply{}); err != nil {
		t.Fatal(err)
	}
	planUnitID := uint(31)
	row := requirementgorm.Requirement{
		WorkspaceID: 7, TeamID: 11, ProjectSetID: 21, ScheduledPlanUnitID: &planUnitID,
		Version: 3, Title: "requirement", Priority: "normal", Status: "backlog",
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	return db, row.ID
}

func newQualityMutationRepository(t *testing.T, db *gorm.DB) *MutationRepository {
	t.Helper()
	repository, err := NewMutationRepository(db, func(tx *gorm.DB) (requirement.MutationScopeResolver, error) {
		return requirement.NewMutationScopeService(requirementgorm.NewMutationScopeRepository(tx))
	}, func(*gorm.DB) (quality.BugMutationEffectSink, error) {
		return qualityMutationEffectNoop{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return repository
}

func qualityCreateCommand(requirementID uint, key string) quality.BugMutation {
	return quality.BugMutation{
		Operation: quality.BugCreate, WorkspaceID: 7, PrincipalID: 41,
		ApplyKey: key, ChangeSetID: "changeset-" + key, InputDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Values: quality.BugMutationValues{RequirementID: requirementID, Title: "tool bug", Status: "triage", Priority: "medium", SeverityKey: "p2"},
	}
}

func assertQualityMutationCounts(t *testing.T, db *gorm.DB, bugs, activities, ledgers int64) {
	t.Helper()
	for model, want := range map[any]int64{&QualityBug{}: bugs, &QualityBugActivity{}: activities, &ToolMutationApply{}: ledgers} {
		var got int64
		if err := db.Model(model).Count(&got).Error; err != nil || got != want {
			t.Fatalf("count %T=%d want=%d err=%v", model, got, want, err)
		}
	}
}
